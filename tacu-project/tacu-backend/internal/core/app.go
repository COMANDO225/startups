package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	cartahttp "tacu-backend/internal/modules/carta/adapters/http"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/adapters/worker"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/almacen"
	"tacu-backend/internal/platform/config"
	"tacu-backend/internal/platform/db"
	"tacu-backend/internal/platform/jobs"
)

// App es todo lo que hay que apagar al terminar.
type App struct {
	Fiber *fiber.App
	Pool  *pgxpool.Pool
	Cola  *river.Client[pgx.Tx]
	log   *slog.Logger
}

// encolador adapta el cliente de River a lo que pide el handler, para que la
// capa HTTP no importe River.
type encolador struct {
	cola *river.Client[pgx.Tx]
	log  *slog.Logger
}

// EncolarLectura manda a leer la carta entera.
//
// El descarte por duplicado se convierte en ERROR, y esa es toda la gracia:
// River lo devuelve sin error, asi que quien llama daba por encolado un job que
// no existe y la importacion se quedaba en 'leyendo' para siempre. Un fallo
// silencioso que solo se ve mirando river_job a mano.
func (e encolador) EncolarLectura(ctx context.Context, importacionID id.ID) error {
	res, err := e.cola.Insert(ctx, worker.LeerCartaArgs{ImportacionID: importacionID}, nil)
	if err != nil {
		return err
	}
	if res.UniqueSkippedAsDuplicate {
		return fmt.Errorf("esta carta ya se esta leyendo")
	}
	return nil
}

// EncolarFotos mete los jobs DENTRO de la transaccion que marca los platos como
// pendientes. Es InsertManyTx y no InsertMany: si el commit falla, ni los
// estados ni los jobs existieron.
func (e encolador) EncolarFotos(ctx context.Context, tx pgx.Tx, importacionID id.ID, platos []id.ID, corregir bool) error {
	if len(platos) == 0 {
		return nil
	}
	lote := make([]river.InsertManyParams, 0, len(platos))
	for _, p := range platos {
		lote = append(lote, river.InsertManyParams{
			Args: worker.GenerarFotoArgs{
				PlatoID: p, ImportacionID: importacionID, Corregir: corregir,
			},
		})
	}
	res, err := e.cola.InsertManyTx(ctx, tx, lote)
	if err != nil {
		return err
	}

	// UN JOB SALTADO POR DUPLICADO NO PUEDE SER SILENCIOSO.
	//
	// River lo descarta sin error, y el plato se queda en 'pendiente' esperando a
	// alguien que nunca va a venir: la tarjeta dice "En cola..." indefinidamente
	// y no hay ni un error en ningun log. Costo un rato largo de depuracion
	// exactamente por eso.
	//
	// Hoy que UniqueOpts solo cubre los estados en vuelo, esto es NORMAL y
	// correcto —significa que ese plato ya tiene su job andando—, asi que no es
	// un fallo. Pero queda escrito: el dia que vuelva a pasar en masa, el log lo
	// dice en vez de esconderlo.
	saltados := 0
	for _, r := range res {
		if r.UniqueSkippedAsDuplicate {
			saltados++
		}
	}
	if saltados > 0 {
		e.log.Info("jobs de foto saltados por duplicado",
			"importacion", importacionID, "saltados", saltados, "pedidos", len(platos))
	}
	return nil
}

// costoDeUnaFoto lee de la tabla de precios lo que cuesta una imagen del modelo
// que encabeza la cadena de generar_foto.
//
// Falla el arranque si no lo encuentra, y es deliberado: sin precio, la reserva
// de presupuesto seria cero y el tope de $3.00 no frenaria nada. Es mejor no
// arrancar que arrancar con el freno desconectado.
func costoDeUnaFoto(cfg config.IA) (dinero.MicrosUSD, error) {
	cadena := cfg.Tareas[string(ai.GenerarFoto)]
	if len(cadena) == 0 {
		return 0, errors.New("la tarea generar_foto no tiene modelos configurados")
	}
	precio, ok := cfg.Precios[cadena[0]]
	if !ok || precio.PorImagen <= 0 {
		return 0, fmt.Errorf("falta el precio por imagen de %q, y sin el el presupuesto no frena nada", cadena[0])
	}
	return dinero.USD(precio.PorImagen), nil
}

// Armar es la raiz de composicion: el UNICO sitio donde se sabe que la
// persistencia es Postgres, que el almacen es disco y que el HTTP es Fiber.
//
// Todo lo demas recibe interfaces y no se entera.
func Armar(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	if cfg.BD.DSN == "" {
		return nil, errors.New("falta el DSN de la base de datos (TACU_BD_DSN)")
	}

	pool, err := db.Abrir(ctx, cfg.BD.DSN, cfg.BD.MaxConexiones)
	if err != nil {
		return nil, err
	}

	repo := postgres.NuevoRepo(pool)

	alm, err := almacen.NuevoDisco(cfg.Almacen.Raiz, cfg.Almacen.Base)
	if err != nil {
		pool.Close()
		return nil, err
	}

	// El Libro real: cada llamada a la IA queda anotada con lo que costo. Hasta
	// ahora era un no-op y el costo se calculaba y se tiraba.
	libro := postgres.NuevoLibro(repo, log)

	ia, err := ArmarIA(ctx, cfg.IA, libro, log, ai.LeerCarta)
	if err != nil {
		pool.Close()
		return nil, err
	}

	lector := app.NuevoLeer(ia)
	organizador := app.NuevoOrganizar(ia)
	conocedor := app.NuevoConocer(ia, repo)

	// La cola. Sus tablas las gestiona River, no nuestras migraciones: mezclarlas
	// obligaria a escribir a mano lo que la libreria ya sabe generar.
	if err := jobs.Migrar(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	// El costo de una foto sale de la tabla de precios del modelo que ENCABEZA la
	// cadena de generar_foto. Es lo que se reserva antes de llamar.
	//
	// Si la cadena cae al respaldo —que cuesta el doble— la reserva se queda
	// corta y el presupuesto se pasa un poco. Se acepta: el respaldo solo entra
	// cuando el principal falla, o sea en pocas fotos, y sobrestimar siempre
	// significaria generar menos fotos de las que caben.
	costoFoto, err := costoDeUnaFoto(cfg.IA)
	if err != nil {
		pool.Close()
		return nil, err
	}

	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers,
		worker.NuevoLeerCarta(repo, alm, lector, organizador, conocedor,
			postgres.ConImportacion, log)); err != nil {
		pool.Close()
		return nil, fmt.Errorf("registrando el worker de lectura: %w", err)
	}
	if err := river.AddWorkerSafely(workers,
		worker.NuevoGenerarFoto(repo, alm, worker.NuevoGeneradorIA(ia, alm, log),
			postgres.ConImportacion, costoFoto, log)); err != nil {
		pool.Close()
		return nil, fmt.Errorf("registrando el worker de fotos: %w", err)
	}

	cola, err := jobs.Nuevo(pool, workers, jobs.Config{
		MaxFotos:     cfg.IA.MaxConcurrenciaPorProveedor,
		TiempoMaxJob: 3 * time.Minute,
	}, log)
	if err != nil {
		pool.Close()
		return nil, err
	}

	importar := app.NuevoImportar(repo, alm, dinero.USD(cfg.IA.PresupuestoPorImportacionUSD))

	// La vista previa del estilo usa el MISMO generador que las fotos de los
	// platos: si saliera de otro modelo dejaria de predecir lo que va a salir.
	estilo := app.NuevoEstilo(repo, alm, worker.NuevoGeneradorIA(ia, alm, log),
		postgres.ConImportacion, costoFoto)
	fotos := app.NuevasFotos(repo, encolador{cola, log}, alm)
	editar := app.NuevoEditar(repo)
	referencias := app.NuevasReferencias(repo, alm)
	paginas := app.NuevasPaginas(repo, alm)
	publicar := app.NuevoPublicar(repo)

	handler := cartahttp.NuevoHandler(
		importar, lector, encolador{cola, log}, fotos, editar, repo, estilo, referencias, paginas,
		conocedor, publicar, repo, alm.URL, log,
		cfg.Servidor.LeerCartaSincrono, cfg.Servidor.TamanoMaxSubidaMB, costoFoto.Dolares(),
	)

	f := fiber.New(fiber.Config{
		AppName:     "tacu",
		BodyLimit:   cfg.Servidor.TamanoMaxSubidaMB * 1024 * 1024,
		ReadTimeout: 30 * time.Second,
		// Sin WriteTimeout: la lectura sincrona de una carta tarda ~10 s y con
		// el respaldo puede llegar a 43. Se acota con el contexto del handler,
		// que es donde el limite significa algo.
		ErrorHandler: manejarError(log),
	})

	f.Use(registrarPeticion(log))
	// AQUI FALTABA PUT y se llevo por delante guardar el tipo de restaurante y
	// guardar el estilo, los dos por PUT. El fallo es INVISIBLE: el preflight
	// responde 204, la peticion real la corta este middleware antes del router,
	// asi que no hay linea en el log y el navegador ve un 503 sin explicacion.
	// Cualquier metodo que use una ruta de Montar() tiene que estar en esta lista.
	f.Use(cors.New(cors.Config{
		AllowOrigins: cfg.Servidor.OrigenesPermitidos,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	v1 := f.Group("/v1")
	handler.Montar(v1)

	// Las imagenes se sirven desde el mismo binario. Un CDN es una linea de
	// nginx el dia que haga falta, y hasta entonces esto no tiene contras.
	f.Get(cfg.Almacen.Base+"/*", servirMedia(alm))

	// nolint:contextcheck // El linter quiere que este closure propague el ctx de
	// Armar, y eso seria un bug: ese contexto es el del ARRANQUE y cmd/api lo
	// cancela en cuanto Armar retorna. Usarlo aqui haria que todos los chequeos
	// de salud fallaran al instante con "context canceled". El contexto correcto
	// en un handler es el de la peticion, que es de donde sale el de abajo.
	f.Get("/salud", func(c fiber.Ctx) error {
		// Timeout propio y NO c.Context(): en Fiber v3 ese contexto devuelve
		// Background si nadie llamo a SetContext, y no se cancela nunca. Una
		// base colgada dejaria el chequeo de salud esperando para siempre, que
		// es justo lo contrario de lo que un chequeo de salud tiene que hacer.
		ctx, cancelar := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancelar()

		if err := pool.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).
				JSON(fiber.Map{"estado": "sin base de datos"})
		}
		return c.JSON(fiber.Map{"estado": "ok"})
	})

	return &App{Fiber: f, Pool: pool, Cola: cola, log: log}, nil
}

// Arrancar pone a trabajar la cola. Va aparte de Armar para que un test pueda
// montar la aplicacion sin que empiece a consumir jobs.
func (a *App) Arrancar(ctx context.Context) error {
	return a.Cola.Start(ctx)
}

// Cerrar apaga en orden inverso al arranque.
//
// El HTTP se cierra ANTES que la cola: primero se deja de aceptar trabajo nuevo
// y despues se le da tiempo al que ya esta en vuelo. Al reves, un job encolado
// por la ultima peticion se quedaria sin quien lo tome.
//
// A la cola se le pide parada suave: un job de foto a mitad ya se pago, y
// cortarlo es tirar dinero que no se recupera.
func (a *App) Cerrar(ctx context.Context) {
	if err := a.Fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("el servidor no cerro limpio", "error", err)
	}
	if err := a.Cola.Stop(ctx); err != nil {
		a.log.Error("la cola no cerro limpio", "error", err)
	}
	a.Pool.Close()
}

func manejarError(log *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		codigo := fiber.StatusInternalServerError
		var fe *fiber.Error
		if errors.As(err, &fe) {
			codigo = fe.Code
		}

		// El detalle va al log, no a la respuesta: un mensaje de error de
		// Postgres en el cuerpo le cuenta al mundo como se llaman las tablas.
		if codigo >= 500 {
			log.Error("error sin manejar", "ruta", c.Path(), "error", err)
			return c.Status(codigo).JSON(fiber.Map{"error": "algo salio mal"})
		}
		return c.Status(codigo).JSON(fiber.Map{"error": err.Error()})
	}
}

func registrarPeticion(log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		inicio := time.Now()
		err := c.Next()
		log.Info("peticion",
			"metodo", c.Method(),
			"ruta", c.Path(),
			"estado", c.Response().StatusCode(),
			"ms", time.Since(inicio).Milliseconds(),
		)
		return err
	}
}

// servirMedia entrega una imagen del almacen.
//
// La ruta viene de la URL, o sea del usuario, asi que NO se concatena con la
// raiz: se abre a traves de almacen.Abrir, que usa os.Root y no puede salirse.
// La primera version hacia c.SendFile(raiz + "/" + c.Params("*")) y
// GET /media/../secreto.txt devolvia el archivo.
func servirMedia(alm *almacen.Disco) fiber.Handler {
	noExiste := func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no existe"})
	}

	return func(c fiber.Ctx) error {
		f, err := alm.Abrir(c.Params("*"))
		if err != nil {
			return noExiste(c)
		}

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			_ = f.Close()
			return noExiste(c)
		}

		if tipo := mime.TypeByExtension(filepath.Ext(c.Params("*"))); tipo != "" {
			c.Set("Content-Type", tipo)
		}
		// Las imagenes son inmutables: la clave lleva el id del plato y
		// regenerar una foto produce una clave nueva.
		c.Set("Cache-Control", "public, max-age=31536000, immutable")

		// SIN defer f.Close(). El cuerpo se escribe DESPUES de que este handler
		// retorna, asi que cerrar aqui manda 0 bytes con un 200 — que fue
		// exactamente lo que paso al primer intento: el ataque quedaba
		// bloqueado y las imagenes de verdad salian vacias.
		//
		// fasthttp cierra el reader por su cuenta cuando implementa io.Closer,
		// que es el caso de *os.File.
		return c.SendStream(f, int(info.Size()))
	}
}
