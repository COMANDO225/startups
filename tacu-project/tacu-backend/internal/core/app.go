package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
// costoDeUnaImagen saca del YAML lo que cuesta la tarea que se le pida.
//
// Generalizada desde costoDeUnaFoto porque ya son dos las tareas que generan
// imagenes y cobran: las fotos de plato y el redibujo del logo, con modelos y
// precios distintos. Cobrar el logo al precio de una foto pondria el
// presupuesto a mentir.
func costoDeUnaImagen(cfg config.IA, tarea ai.Tarea) (dinero.MicrosUSD, error) {
	cadena := cfg.Tareas[string(tarea)]
	if len(cadena) == 0 {
		return 0, fmt.Errorf("la tarea %s no tiene modelos configurados", tarea)
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

	alm, err := armarAlmacen(ctx, cfg.Almacen, log)
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
	costoFoto, err := costoDeUnaImagen(cfg.IA, ai.GenerarFoto)
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
	portada := app.NuevaPortada(repo, alm)
	// El logo cuesta lo suyo, no lo que cuesta una foto: otro proveedor y otro
	// precio. Se cobra del MISMO presupuesto de la carta.
	costoLogo, err := costoDeUnaImagen(cfg.IA, ai.GenerarLogo)
	if err != nil {
		pool.Close()
		return nil, err
	}
	logo := app.NuevoLogo(repo, alm, worker.NuevoGeneradorIA(ia, alm, log), costoLogo)
	publicar := app.NuevoPublicar(repo)

	// Las imagenes privadas se autorizan por su URL, porque un <img src> no manda
	// cabeceras. Se arma antes que el handler porque el handler ya recibe la
	// funcion de URLs envuelta: asi no queda ni un sitio que pueda armar una sin
	// firmar.
	firmante, err := almacen.NuevoFirmante(cfg.Almacen.FirmaSecreto)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("TACU_MEDIA_SECRETO: %w", err)
	}

	handler := cartahttp.NuevoHandler(
		importar, lector, encolador{cola, log}, fotos, editar, repo, estilo, referencias, paginas,
		portada, logo, conocedor, publicar, repo, firmante.Envolver(alm.URL), log,
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

	// Las imagenes se sirven desde el mismo binario. Lo privado —las hojas de la
	// carta, el estilo, las referencias— solo con la URL firmada; lo publico, que
	// es el catalogo, sin nada.
	//
	// nolint:contextcheck // contextcheck quiere que el closure reciba un ctx por
	// parametro. Un fiber.Handler tiene la firma que tiene, y el contexto que hay
	// que propagar —el de la peticion— sale de c.Context() ahi dentro, que es lo
	// que hace.
	f.Get(cfg.Almacen.Base+"/*", servirMedia(alm, firmante))

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
// Almacen es lo que la aplicacion necesita del almacenamiento de imagenes. La
// declara quien la consume, como el resto: ni el disco ni R2 exportan interfaz.
type Almacen interface {
	Guardar(ctx context.Context, clave string, bytes []byte) error
	Leer(ctx context.Context, clave string) ([]byte, string, error)
	Borrar(ctx context.Context, clave string) error
	URL(clave string) string
}

// armarAlmacen elige disco o bucket, y con el bucket COMPRUEBA que responde.
//
// Comprobar al arrancar y no a la primera foto: unas credenciales mal puestas se
// descubririan cuando un dueno pulsa "generar", despues de haberle cobrado
// $0.0336 por un error de configuracion nuestro. Aqui, el servidor no levanta y
// dice por que.
func armarAlmacen(ctx context.Context, cfg config.Almacen, log *slog.Logger) (Almacen, error) {
	if !cfg.UsaR2() {
		return almacen.NuevoDisco(cfg.Raiz, cfg.Base)
	}

	r2, err := almacen.NuevoR2(cfg.R2.Cuenta, cfg.R2.ClaveID, cfg.R2.Secreto,
		cfg.R2.BucketPublico, cfg.R2.BucketPrivado, cfg.R2.DominioPublico, cfg.Base, log)
	if err != nil {
		return nil, err
	}

	prueba, cancelar := context.WithTimeout(ctx, 15*time.Second)
	defer cancelar()
	if err := r2.Comprobar(prueba); err != nil {
		return nil, fmt.Errorf("no se pudo hablar con R2: %w", err)
	}

	log.Info("almacen en R2",
		"publico", cfg.R2.BucketPublico, "privado", cfg.R2.BucketPrivado,
		"dominio", cfg.R2.DominioPublico)
	return r2, nil
}

// LectorDeImagenes es lo unico que servirMedia necesita: entregar bytes por
// clave. Una interfaz y no *almacen.Disco porque las privadas —las hojas de la
// carta, el estilo— salen por aqui tambien cuando viven en un bucket.
type LectorDeImagenes interface {
	Leer(ctx context.Context, clave string) ([]byte, string, error)
}

func servirMedia(alm LectorDeImagenes, firmante *almacen.Firmante) fiber.Handler {
	return func(c fiber.Ctx) error {
		clave := c.Params("*")

		// Se comprueba con la MISMA variable que despues se lee. Hacerlo en un
		// middleware aparte obligaria a volver a sacar la clave de la ruta, y ahi
		// nace el fallo clasico: el que autoriza mira una clave y el que abre
		// abre otra.
		if err := firmante.Comprobar(clave, c.Query("exp"), c.Query("f"), time.Now()); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}

		// La ruta viene de la URL, o sea del usuario. NO se concatena con
		// ninguna raiz: quien lee sabe confinar —el disco con os.Root, el bucket
		// porque una clave no es una ruta—. La primera version hacia
		// SendFile(raiz + "/" + loQueVenga) y GET /media/../secreto.txt
		// devolvia el archivo.
		datos, tipo, err := alm.Leer(c.Context(), clave)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no existe"})
		}

		if tipo != "" {
			c.Set("Content-Type", tipo)
		}
		// Inmutables: cada escritura produce una clave nueva. Lo privado va como
		// private para que no lo guarde una cache compartida; que el navegador se
		// lo quede mas alla de la caducidad no estorba —el contenido no cambia y
		// es su dueno quien lo tiene—, y ahorra volver a bajarlo.
		cache := "public, max-age=31536000, immutable"
		if !almacen.EsPublica(clave) {
			cache = "private, max-age=31536000, immutable"
		}
		c.Set("Cache-Control", cache)

		// En memoria y no en streaming: lo que sale por aqui son imagenes de
		// como mucho unos megas —el tope de subida las acota— y a cambio esto
		// sirve igual desde el disco que desde un bucket, sin que el handler
		// sepa cual de los dos hay debajo.
		return c.Send(datos)
	}
}
