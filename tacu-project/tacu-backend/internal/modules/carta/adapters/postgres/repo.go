// Package postgres guarda y lee cartas. Es el unico sitio que sabe que la
// persistencia es Postgres: el dominio y los casos de uso no importan nada de
// aqui.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/adapters/postgres/cartadb"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/db"
)

type Repo struct {
	pool *pgxpool.Pool
	q    *cartadb.Queries
}

func NuevoRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool, q: cartadb.New(pool)}
}

// ErrNoExiste lo devuelven las lecturas cuando no hay nada con ese id. El borde
// HTTP lo traduce a 404; cualquier otro error es un 500.
var ErrNoExiste = fmt.Errorf("no existe")

// Los topes del claim de foto. Viven aqui, junto a la consulta que los usa, y
// NO en el worker: el worker declara la intencion y este adaptador sabe como se
// expresa contra Postgres.
//
// maxIntentosFoto tiene que coincidir con el MaxAttempts del job. Son dos
// guardas distintas a proposito: River cuenta intentos del JOB y esta cuenta
// llamadas PAGADAS, asi que un job reencolado por otra via no puede saltarse el
// tope de gasto.
const (
	maxIntentosFoto        = 2
	maxFotosPorRestaurante = 4
	rescateFoto            = 3 * time.Minute
)

// CrearBorrador crea el restaurante y su importacion en una sola transaccion.
//
// Van juntos porque un restaurante sin importacion no le sirve a nadie: si la
// segunda insercion falla, la primera tampoco tiene que quedar.
func (r *Repo) CrearBorrador(
	ctx context.Context,
	restauranteID, importacionID id.ID,
	nombre string,
	tipos []domain.Tipo,
	tokenHash []byte,
	ip *netip.Addr,
	estado domain.Estado,
	claves []string,
	presupuesto dinero.MicrosUSD,
) error {
	imagenes, err := json.Marshal(claves)
	if err != nil {
		return fmt.Errorf("serializando las claves de las imagenes: %w", err)
	}
	elegidos, err := json.Marshal(clavesDeTipos(tipos))
	if err != nil {
		return fmt.Errorf("serializando los tipos: %w", err)
	}

	return db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		if err := q.CrearRestaurante(ctx, cartadb.CrearRestauranteParams{
			ID:        restauranteID,
			Nombre:    nombre,
			TokenHash: tokenHash,
			Tipos:     elegidos,
		}); err != nil {
			return fmt.Errorf("creando el restaurante: %w", err)
		}

		if err := q.CrearImportacion(ctx, cartadb.CrearImportacionParams{
			ID:                importacionID,
			RestauranteID:     restauranteID,
			Estado:            string(estado),
			Ip:                ip,
			Imagenes:          imagenes,
			PresupuestoMicros: int64(presupuesto),
		}); err != nil {
			return fmt.Errorf("creando la importacion: %w", err)
		}
		return nil
	})
}

// GuardarHojasIniciales guarda las hojas del restaurante que se creo sin ellas.
//
// Devuelve false cuando no toco ninguna fila, que es el caso de dos pestanas
// subiendo a la vez: la consulta solo muerde mientras la importacion sigue
// 'nueva', asi que la segunda no puede reemplazar las hojas de la primera.
func (r *Repo) GuardarHojasIniciales(ctx context.Context, importacionID id.ID,
	claves []string, estado domain.Estado) (bool, error) {
	imagenes, err := json.Marshal(claves)
	if err != nil {
		return false, fmt.Errorf("serializando las claves de las imagenes: %w", err)
	}

	filas, err := r.q.GuardarHojasIniciales(ctx, cartadb.GuardarHojasInicialesParams{
		ID:       importacionID,
		Imagenes: imagenes,
		Estado:   string(estado),
	})
	if err != nil {
		return false, fmt.Errorf("guardando las hojas de la carta: %w", err)
	}
	return filas > 0, nil
}

func clavesDeTipos(tipos []domain.Tipo) []string {
	fuera := make([]string, 0, len(tipos))
	for _, t := range tipos {
		fuera = append(fuera, string(t))
	}
	return fuera
}

// ReleerCarta devuelve la importacion a 'leyendo' para volver a leerla entera.
//
// Devuelve false si no estaba 'lista': ahi no hay nada roto, es que no se puede
// —ya se esta leyendo, o esta publicada— y el borde lo traduce a 409.
func (r *Repo) ReleerCarta(ctx context.Context, importacionID id.ID) (bool, error) {
	filas, err := r.q.ReleerCarta(ctx, importacionID)
	if err != nil {
		return false, fmt.Errorf("marcando la carta para releer: %w", err)
	}
	return filas > 0, nil
}

// BorrarPlato borra un plato de verdad. Lo pide el dueno; ninguna lectura llega
// aqui.
func (r *Repo) BorrarPlato(ctx context.Context, platoID id.ID) error {
	if err := r.q.BorrarPlato(ctx, platoID); err != nil {
		return fmt.Errorf("borrando el plato: %w", err)
	}
	return nil
}

// RecuperarPlato deshace el marcado de ausente.
func (r *Repo) RecuperarPlato(ctx context.Context, platoID id.ID) error {
	if err := r.q.RecuperarPlato(ctx, platoID); err != nil {
		return fmt.Errorf("recuperando el plato: %w", err)
	}
	return nil
}

// BorrarPlatosDeLaHoja borra los platos de una hoja que se acaba de quitar.
//
// Devuelve cuantos se fueron. La hoja vacia no borra NADA: hoja_clave vacia es
// "no se sabe de donde salio", y un DELETE con esa condicion se llevaria por
// delante todos los platos sin atribuir de la carta.
func (r *Repo) BorrarPlatosDeLaHoja(ctx context.Context, importacionID id.ID, hoja string) (int, error) {
	if hoja == "" {
		return 0, nil
	}
	n, err := r.q.BorrarPlatosDeLaHoja(ctx, cartadb.BorrarPlatosDeLaHojaParams{
		ImportacionID: importacionID,
		HojaClave:     hoja,
	})
	if err != nil {
		return 0, fmt.Errorf("borrando los platos de la hoja: %w", err)
	}
	return int(n), nil
}

// GuardarCarta escribe la carta leida y deja la importacion en 'lista', TODO en
// una sola transaccion.
//
// NO BORRA. Antes empezaba con un DELETE de todos los platos y los reinsertaba
// con ids nuevos: como la foto cuelga del id, releer una carta de 74 platos
// tiraba $2.50 en fotos ya pagadas y cada precio corregido a mano. Ahora cruza
// por nombre (domain.Reconciliar) y el plato que sigue en la carta sigue siendo
// el mismo plato.
//
// Reintentar el job entero sigue siendo seguro, que es lo que daba el borrado:
// cruzar por nombre es idempotente, asi que la segunda pasada actualiza lo
// mismo y no duplica nada.
func (r *Repo) GuardarCarta(
	ctx context.Context,
	importacionID id.ID,
	carta domain.Carta,
	cruda []byte,
	marcas domain.Marcas,
) error {
	return db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		filas, err := q.PlatosParaReconciliar(ctx, importacionID)
		if err != nil {
			return fmt.Errorf("leyendo los platos actuales: %w", err)
		}
		existentes := make([]domain.PlatoExistente, 0, len(filas))
		for _, f := range filas {
			existentes = append(existentes, domain.PlatoExistente{
				ID: f.ID, Nombre: f.Nombre, Hoja: f.HojaClave,
			})
		}

		// Las hojas que ESTA lectura miro. Solo un plato colgado de una de
		// ellas puede quedar ausente: si el dueno quito una hoja y eligio
		// quedarse con sus platos, esta lectura no tiene nada que decir sobre
		// ellos porque no vio esa hoja.
		hojas, err := q.ClavesDeImagenes(ctx, importacionID)
		if err != nil {
			return fmt.Errorf("leyendo las hojas de la carta: %w", err)
		}

		plan := domain.Reconciliar(carta, existentes, claves(hojas))

		for _, p := range plan.Actualizar {
			precios, err := json.Marshal(p.Precios)
			if err != nil {
				return fmt.Errorf("serializando los precios de %q: %w", p.Nombre, err)
			}
			if err := q.ActualizarPlatoLeido(ctx, cartadb.ActualizarPlatoLeidoParams{
				ID:             p.ID,
				Categoria:      p.Categoria,
				OrdenCategoria: int32(p.OrdenCategoria),
				Orden:          int32(p.Orden),
				Nombre:         p.Nombre,
				Descripcion:    p.Descripcion,
				Precios:        precios,
				Revisar:        string(p.Revisar),
				PlatoTipico:    p.Tipico,
				TipicoOrigen:   p.TipicoOrigen,
				HojaClave:      p.Hoja,
			}); err != nil {
				return fmt.Errorf("actualizando el plato %q: %w", p.Nombre, err)
			}
		}

		for _, p := range plan.Insertar {
			precios, err := json.Marshal(p.Precios)
			if err != nil {
				return fmt.Errorf("serializando los precios de %q: %w", p.Nombre, err)
			}
			if err := q.InsertarPlato(ctx, cartadb.InsertarPlatoParams{
				ID:             id.Nuevo(),
				ImportacionID:  importacionID,
				Categoria:      p.Categoria,
				OrdenCategoria: int32(p.OrdenCategoria),
				Orden:          int32(p.Orden),
				Nombre:         p.Nombre,
				Descripcion:    p.Descripcion,
				Precios:        precios,
				Revisar:        string(p.Revisar),
				PlatoTipico:    p.Tipico,
				TipicoOrigen:   p.TipicoOrigen,
				HojaClave:      p.Hoja,
			}); err != nil {
				return fmt.Errorf("insertando el plato %q: %w", p.Nombre, err)
			}
		}

		if len(plan.Ausentes) > 0 {
			if err := q.MarcarPlatosAusentes(ctx, plan.Ausentes); err != nil {
				return fmt.Errorf("marcando los platos ausentes: %w", err)
			}
		}

		if err := q.MarcarImportacionLista(ctx, cartadb.MarcarImportacionListaParams{
			ID:              importacionID,
			CartaCruda:      cruda,
			MarcasRevisar:   int32(marcas.Revisar),
			MarcasConfirmar: int32(marcas.Confirmar),
		}); err != nil {
			return fmt.Errorf("marcando la importacion lista: %w", err)
		}
		return nil
	})
}

// MarcarEtapa dice en que va la lectura. Un fallo aqui NO puede tumbar la
// lectura: es informacion de progreso, no el trabajo.
func (r *Repo) MarcarEtapa(ctx context.Context, importacionID id.ID, etapa int16) error {
	return r.q.MarcarEtapa(ctx, cartadb.MarcarEtapaParams{ID: importacionID, Etapa: etapa})
}

func (r *Repo) MarcarFallida(ctx context.Context, importacionID id.ID, motivo string) error {
	return r.q.MarcarImportacionFallida(ctx, cartadb.MarcarImportacionFallidaParams{
		ID:    importacionID,
		Error: motivo,
	})
}

// Obtener devuelve la importacion con su carta.
//
// Son dos consultas y no un JOIN a proposito: un JOIN de importacion con 42
// platos repite las columnas de la importacion 42 veces, y reconstruir la
// jerarquia en Go es mas codigo que hacer la segunda consulta.
func (r *Repo) Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error) {
	fila, err := r.q.ObtenerImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return domain.Importacion{}, ErrNoExiste
		}
		return domain.Importacion{}, fmt.Errorf("leyendo la importacion: %w", err)
	}

	imp := domain.Importacion{
		ID: fila.ID,
		Restaurante: domain.Restaurante{
			ID:     fila.RestauranteID,
			Nombre: fila.RestauranteNombre,
			Slug:   opcional(fila.RestauranteSlug),
		},
		Estado:      domain.Estado(fila.Estado),
		Etapa:       fila.Etapa,
		Marcas:      domain.Marcas{Revisar: int(fila.MarcasRevisar), Confirmar: int(fila.MarcasConfirmar)},
		Presupuesto: dinero.MicrosUSD(fila.PresupuestoMicros),
		Reservado:   dinero.MicrosUSD(fila.ReservadoMicros),
		Gastado:     dinero.MicrosUSD(fila.GastadoMicros),
		Imagenes:    claves(fila.Imagenes),
		Error:       fila.Error,
	}

	// Mientras esta leyendo todavia no hay platos que traer.
	if imp.Estado == domain.Leyendo {
		return imp, nil
	}

	imp.Carta, err = r.leerCarta(ctx, importacionID)
	if err != nil {
		return domain.Importacion{}, err
	}
	return imp, nil
}

// leerCarta reconstruye la jerarquia categoria -> platos desde filas planas.
//
// Las filas vienen ordenadas por (orden_categoria, orden), asi que basta con
// abrir una categoria nueva cada vez que cambia orden_categoria. No hace falta
// mapa ni ordenar en memoria.
func (r *Repo) leerCarta(ctx context.Context, importacionID id.ID) (domain.Carta, error) {
	filas, err := r.q.ListarPlatos(ctx, importacionID)
	if err != nil {
		return domain.Carta{}, fmt.Errorf("leyendo los platos: %w", err)
	}

	var carta domain.Carta
	ultima := int32(-1)

	for _, f := range filas {
		var precios []domain.Precio
		if err := json.Unmarshal(f.Precios, &precios); err != nil {
			return domain.Carta{}, fmt.Errorf("los precios de %q no son JSON valido: %w", f.Nombre, err)
		}

		if f.OrdenCategoria != ultima {
			carta.Categorias = append(carta.Categorias, domain.Categoria{Nombre: f.Categoria})
			ultima = f.OrdenCategoria
		}

		i := len(carta.Categorias) - 1
		carta.Categorias[i].Platos = append(carta.Categorias[i].Platos, domain.Plato{
			ID:           f.ID,
			Nombre:       f.Nombre,
			Descripcion:  f.Descripcion,
			Categoria:    f.Categoria,
			Precios:      precios,
			Revisar:      domain.MotivoRevision(f.Revisar),
			Tipico:       f.PlatoTipico,
			TipicoOrigen: f.TipicoOrigen,
			Hoja:         f.HojaClave,
			Ausente:      f.Ausente,
			FotoAjuste:   f.FotoAjuste,
			Foto: domain.Foto{
				Estado:   domain.EstadoFoto(f.FotoEstado),
				Origen:   domain.OrigenFoto(f.FotoOrigen),
				Clave:    f.FotoClave,
				Intentos: int(f.FotoIntentos),
			},
		})
	}
	return carta, nil
}

// ClavesDeImagenes devuelve las claves de las fotos que subio el dueno.
//
// El worker las necesita porque cuando su job corre, la peticion HTTP que traia
// las imagenes en memoria ya termino. Meterlas en los argumentos del job seria
// guardar 40 MB de JSON en una fila y replicarlos en cada reintento.
func (r *Repo) ClavesDeImagenes(ctx context.Context, importacionID id.ID) ([]string, error) {
	crudo, err := r.q.ClavesDeImagenes(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return nil, ErrNoExiste
		}
		return nil, fmt.Errorf("leyendo las claves: %w", err)
	}

	var claves []string
	if err := json.Unmarshal(crudo, &claves); err != nil {
		return nil, fmt.Errorf("las claves no son JSON valido: %w", err)
	}
	return claves, nil
}

// TokenHash devuelve el hash del token del dueno de una importacion, para que el
// borde HTTP lo compare. Devuelve el hash y no el token porque el token no se
// guarda en ningun lado: solo lo tiene quien lo recibio al crear el borrador.
func (r *Repo) TokenHash(ctx context.Context, importacionID id.ID) ([]byte, error) {
	h, err := r.q.TokenHashDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return nil, ErrNoExiste
		}
		return nil, fmt.Errorf("leyendo el token: %w", err)
	}
	return h, nil
}

// AnotarGasto registra lo que costo una llamada a la IA y lo acumula en la
// importacion, en una sola transaccion: si se anota el detalle pero no el
// acumulado, el presupuesto queda mintiendo.
func (r *Repo) AnotarGasto(
	ctx context.Context,
	importacionID *uuid.UUID,
	tarea, modelo string,
	tokensEntrada, tokensSalida, imagenes int,
	costo dinero.MicrosUSD,
	intentos int,
) error {
	return db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		if err := q.AnotarGastoIA(ctx, cartadb.AnotarGastoIAParams{
			ImportacionID: importacionID,
			Tarea:         tarea,
			Modelo:        modelo,
			TokensEntrada: int32(tokensEntrada),
			TokensSalida:  int32(tokensSalida),
			Imagenes:      int32(imagenes),
			CostoMicros:   int64(costo),
			Intentos:      int32(intentos),
		}); err != nil {
			return fmt.Errorf("anotando el gasto: %w", err)
		}

		// Los CLIs de laboratorio gastan sin pertenecer a ninguna importacion:
		// su gasto se registra igual, pero no hay acumulado que actualizar.
		if importacionID == nil {
			return nil
		}
		if err := q.SumarGastado(ctx, cartadb.SumarGastadoParams{
			ID:            *importacionID,
			GastadoMicros: int64(costo),
		}); err != nil {
			return fmt.Errorf("acumulando el gasto: %w", err)
		}
		return nil
	})
}

// AnotarGastoDeLectura es el atajo para el caso mas comun: anotar lo que costo
// una llamada de IA a partir del Uso que devuelve la propia capa de IA.
//
// Existe para que quien llama no tenga que desarmar el Uso campo por campo ni
// acordarse de convertir el costo a micros.
func (r *Repo) AnotarGastoDeLectura(ctx context.Context, importacionID id.ID, uso ai.Uso) error {
	return r.AnotarGasto(ctx, &importacionID, string(uso.Tarea), uso.Modelo,
		uso.TokensEntrada, uso.TokensSalida, uso.Imagenes,
		dinero.USD(uso.CostoUSD), uso.Intentos)
}

// GastoPorTarea responde cuanto costo una carta y en que. Es el unit economics
// del producto, que es la unica pregunta de observabilidad que hoy importa.
func (r *Repo) GastoPorTarea(ctx context.Context, importacionID id.ID) (map[string]dinero.MicrosUSD, error) {
	filas, err := r.q.GastoPorTarea(ctx, &importacionID)
	if err != nil {
		return nil, fmt.Errorf("resumiendo el gasto: %w", err)
	}
	total := make(map[string]dinero.MicrosUSD, len(filas))
	for _, f := range filas {
		total[f.Tarea] = dinero.MicrosUSD(f.CostoMicros)
	}
	return total, nil
}

func opcional(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- fotos ---

// ReclamarFoto toma el plato si le toca a este worker. Ver el comentario de la
// consulta en queries/foto.sql: es lo que impide pagar dos veces por la misma
// foto y lo que reparte la capacidad entre restaurantes.
//
// Va dentro de una transaccion con un lock de la importacion porque sin el, el
// techo por restaurante SE FILTRA: en READ COMMITTED varios claims cuentan la
// misma instantanea y entran todos. Medido: con el tope en 4, seis generando a
// la vez.
//
// El lock es POR IMPORTACION, no global: dos restaurantes distintos siguen
// reclamando en paralelo. Lo unico que se serializa son los claims de una misma
// carta, que es exactamente lo que el techo quiere ordenar.
func (r *Repo) ReclamarFoto(ctx context.Context, platoID, importacionID id.ID) (domain.EncargoDeFoto, bool, error) {
	var encargo domain.EncargoDeFoto
	var mio bool

	err := db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		if err := q.BloquearImportacion(ctx, importacionID.String()); err != nil {
			return fmt.Errorf("bloqueando la importacion: %w", err)
		}

		fila, err := q.ReclamarFoto(ctx, cartadb.ReclamarFotoParams{
			PlatoID:           platoID,
			MaxIntentos:       maxIntentosFoto,
			Rescate:           intervalo(rescateFoto),
			ImportacionID:     importacionID,
			MaxPorRestaurante: maxFotosPorRestaurante,
		})
		if err != nil {
			if db.SinFilas(err) {
				return nil // no le toca: mio se queda en false
			}
			return fmt.Errorf("reclamando la foto: %w", err)
		}

		encargo.Plato = domain.Plato{
			ID:              fila.ID,
			Nombre:          fila.Nombre,
			Descripcion:     fila.Descripcion,
			Categoria:       fila.Categoria,
			Tipico:          fila.PlatoTipico,
			FotoAjuste:      fila.FotoAjuste,
			FotoReferencias: claves(fila.FotoReferencias),
			Foto:            domain.Foto{Clave: fila.FotoClave},
		}
		mio = true

		// Dentro de la misma transaccion que el claim: ver RepoFoto.
		rest, err := q.RestauranteDeImportacion(ctx, importacionID)
		if err != nil {
			return fmt.Errorf("leyendo el restaurante: %w", err)
		}
		encargo.Tipos = domain.TiposValidos(claves(rest.Tipos))

		base, err := baseDeFoto(ctx, q, rest.ID, fila.Categoria)
		if err != nil {
			return err
		}

		// El estilo del dueno y lo que el banco sabe se pliegan aqui, y quien
		// gana el recipiente lo decide el dominio: la vajilla del dueno vale
		// para lo que va en plato, no para una bebida. Plegarlo aqui deja
		// intacta la firma de Ensamblar y de PromptFoto.
		tipico, err := tipicoDePlato(ctx, q, fila.PlatoTipico)
		if err != nil {
			return err
		}
		encargo.Base = tipico.ConLaBaseDelDueno(base.Receta())
		return nil
	})
	return encargo, mio, err
}

// tipicoDePlato trae lo que el banco sabe de ESTE plato. Sin clave emparejada
// devuelve la entrada vacia, que hereda entera de la cocina: es exactamente lo
// que pasaba antes de que el banco existiera.
func tipicoDePlato(ctx context.Context, q *cartadb.Queries, clave string) (domain.PlatoTipico, error) {
	if clave == "" {
		return domain.PlatoTipico{}, nil
	}
	fila, err := q.PlatoTipicoPorClave(ctx, clave)
	if err != nil {
		if db.SinFilas(err) {
			// La clave quedo colgando de un plato que ya no esta en el banco.
			// No es motivo para no generar la foto.
			return domain.PlatoTipico{}, nil
		}
		return domain.PlatoTipico{}, fmt.Errorf("leyendo el plato tipico %q: %w", clave, err)
	}
	return domain.PlatoTipico{
		Clave:      fila.Clave,
		Nombre:     fila.Nombre,
		Cocina:     fila.Cocina,
		Curso:      fila.Curso,
		Aspecto:    fila.Aspecto,
		Recipiente: fila.Recipiente,
		Guarnicion: fila.Guarnicion,
		Jamas:      fila.Jamas,
		Envasado:   fila.Envasado,
	}, nil
}

// BancoDePlatos trae el catalogo entero. Se lee UNA vez por carta, al emparejar:
// la regla de emparejamiento vive en el dominio y necesita ver todos los
// candidatos a la vez.
func (r *Repo) BancoDePlatos(ctx context.Context) ([]domain.PlatoTipico, error) {
	filas, err := r.q.ListarPlatosTipicos(ctx)
	if err != nil {
		return nil, fmt.Errorf("leyendo el banco de platos: %w", err)
	}
	banco := make([]domain.PlatoTipico, 0, len(filas))
	for _, f := range filas {
		banco = append(banco, domain.PlatoTipico{
			Clave:        f.Clave,
			Nombre:       f.Nombre,
			Cocina:       f.Cocina,
			Curso:        f.Curso,
			Aspecto:      f.Aspecto,
			Recipiente:   f.Recipiente,
			Guarnicion:   f.Guarnicion,
			Jamas:        f.Jamas,
			Envasado:     f.Envasado,
			Patrones:     claves(f.Patrones),
			Ingredientes: claves(f.Ingredientes),
		})
	}
	return banco, nil
}

// ActualizarTipicos guarda a que plato del banco corresponde cada plato de una
// carta YA guardada.
//
// Fila a fila y no en lote: son 74 UPDATE por carta y solo corre cuando alguien
// pide reconocer, no en el camino de cada foto.
func (r *Repo) ActualizarTipicos(ctx context.Context, platos []domain.Plato) error {
	for _, p := range platos {
		if p.Tipico == "" {
			continue
		}
		origen := p.TipicoOrigen
		if origen == "" {
			origen = domain.TipicoPorRegla
		}
		if err := r.q.MarcarTipicoDePlato(ctx, cartadb.MarcarTipicoDePlatoParams{
			PlatoID: p.ID,
			Clave:   p.Tipico,
			Origen:  origen,
		}); err != nil {
			return fmt.Errorf("marcando el plato tipico de %q: %w", p.Nombre, err)
		}
	}
	return nil
}

// PendientesDeDescribir son las filas cosechadas que todavia no saben como se
// ven. Se rellenan solas segun aparecen en cartas reales, y en lote con
// cmd/describir.
func (r *Repo) PendientesDeDescribir(ctx context.Context, limite int) ([]domain.PlatoTipico, error) {
	filas, err := r.q.PlatosTipicosSinDescribir(ctx, int32(limite))
	if err != nil {
		return nil, fmt.Errorf("leyendo los platos sin describir: %w", err)
	}
	fuera := make([]domain.PlatoTipico, 0, len(filas))
	for _, f := range filas {
		fuera = append(fuera, domain.PlatoTipico{
			Clave:        f.Clave,
			Nombre:       f.Nombre,
			Cocina:       f.Cocina,
			Curso:        f.Curso,
			Patrones:     claves(f.Patrones),
			Ingredientes: claves(f.Ingredientes),
		})
	}
	return fuera, nil
}

// GuardarPlatosTipicos mete en el banco lo que se aprendio de una carta.
//
// Fila a fila y no en lote: son dos o tres por carta, y el ON CONFLICT DO
// NOTHING de la consulta hace que una clave repetida no rompa las demas.
func (r *Repo) GuardarPlatosTipicos(ctx context.Context, platos []domain.PlatoTipico) error {
	for _, p := range platos {
		patrones, err := json.Marshal(p.Patrones)
		if err != nil {
			return fmt.Errorf("serializando los patrones de %q: %w", p.Clave, err)
		}
		if err := r.q.GuardarPlatoTipico(ctx, cartadb.GuardarPlatoTipicoParams{
			Clave:      p.Clave,
			Nombre:     p.Nombre,
			Cocina:     p.Cocina,
			Curso:      p.Curso,
			Aspecto:    p.Aspecto,
			Recipiente: p.Recipiente,
			Guarnicion: p.Guarnicion,
			Jamas:      p.Jamas,
			Patrones:   patrones,
		}); err != nil {
			return fmt.Errorf("guardando el plato tipico %q: %w", p.Clave, err)
		}
	}
	return nil
}

// Pliega la base general con el override de la categoria. La consulta devuelve
// la general primero, asi que basta con plegar en orden.
func baseDeFoto(ctx context.Context, q *cartadb.Queries, restauranteID id.ID, categoria string) (domain.Estilo, error) {
	filas, err := q.BaseDeFoto(ctx, cartadb.BaseDeFotoParams{
		RestauranteID: restauranteID,
		Categoria:     categoria,
	})
	if err != nil {
		return domain.Estilo{}, fmt.Errorf("leyendo la base de fotos: %w", err)
	}

	var estilo domain.Estilo
	for _, f := range filas {
		estilo = estiloDeFila(f.Recipiente, f.VajillaClave, f.VajillaVistaClave,
			f.Fondo, f.FondoClave, f.FondoVistaClave).Sobre(estilo)
	}
	return estilo, nil
}

// estiloDeFila arma las dos ranuras desde las seis columnas. Una sola funcion
// para las dos lecturas —la plegada y la propia— o el dia que una ranura gane un
// campo habria que acordarse de dos sitios.
func estiloDeFila(recipiente, vajillaClave, vajillaVista,
	fondo, fondoClave, fondoVista string) domain.Estilo {
	return domain.Estilo{
		Vajilla: domain.Ranura{Texto: recipiente, Foto: vajillaClave, Vista: vajillaVista},
		Fondo:   domain.Ranura{Texto: fondo, Foto: fondoClave, Vista: fondoVista},
	}
}

// Una lista rota no puede tumbar la generacion: se trata como si no hubiera.
func claves(crudo []byte) []string {
	if len(crudo) == 0 {
		return nil
	}
	var fuera []string
	if err := json.Unmarshal(crudo, &fuera); err != nil {
		return nil
	}
	return fuera
}

// ReservarPresupuesto cobra por adelantado. Devuelve false cuando ya no cabe.
func (r *Repo) ReservarPresupuesto(ctx context.Context, importacionID id.ID, costo dinero.MicrosUSD) (bool, error) {
	_, err := r.q.ReservarPresupuesto(ctx, cartadb.ReservarPresupuestoParams{
		Costo:         int64(costo),
		ImportacionID: importacionID,
	})
	if err != nil {
		if db.SinFilas(err) {
			return false, nil // no cabe: la condicion del UPDATE no se cumplio
		}
		return false, fmt.Errorf("reservando presupuesto: %w", err)
	}
	return true, nil
}

func (r *Repo) MarcarFotoLista(ctx context.Context, platoID id.ID, origen domain.OrigenFoto, clave string) error {
	return r.q.MarcarFotoLista(ctx, cartadb.MarcarFotoListaParams{
		PlatoID: platoID,
		Origen:  string(origen),
		Clave:   clave,
	})
}

func (r *Repo) MarcarFotoConEstado(ctx context.Context, platoID id.ID, estado domain.EstadoFoto) error {
	return r.q.MarcarFotoConEstado(ctx, cartadb.MarcarFotoConEstadoParams{
		PlatoID: platoID,
		Estado:  string(estado),
	})
}

// Aplica las etiquetas, reverifica el plato y recuenta las marcas, en UNA
// transaccion: la barra de publicar lee ese recuento, y si el plato se limpiara
// sin recontarlo seguiria bloqueada para siempre.
func (r *Repo) EditarPlato(ctx context.Context, platoID id.ID, etiquetas []string) (domain.Plato, domain.Marcas, error) {
	var plato domain.Plato
	var marcas domain.Marcas

	err := db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		fila, err := q.PlatoPorID(ctx, platoID)
		if err != nil {
			if db.SinFilas(err) {
				return ErrNoExiste
			}
			return fmt.Errorf("leyendo el plato: %w", err)
		}

		var precios []domain.Precio
		if err := json.Unmarshal(fila.Precios, &precios); err != nil {
			return fmt.Errorf("los precios de %q no son JSON valido: %w", fila.Nombre, err)
		}

		// Por POSICION, como los pinta la pantalla. Una lista mas corta deja el
		// resto como estaba.
		for i := range precios {
			if i < len(etiquetas) {
				precios[i].Etiqueta = strings.TrimSpace(etiquetas[i])
			}
		}

		plato = domain.Plato{
			ID:          fila.ID,
			Nombre:      fila.Nombre,
			Descripcion: fila.Descripcion,
			Categoria:   fila.Categoria,
			Precios:     precios,
			FotoAjuste:  fila.FotoAjuste,
		}
		plato.Verificar()

		nuevos, err := json.Marshal(precios)
		if err != nil {
			return fmt.Errorf("serializando los precios: %w", err)
		}
		if err := q.ActualizarPlato(ctx, cartadb.ActualizarPlatoParams{
			PlatoID: platoID,
			Precios: nuevos,
			Revisar: string(plato.Revisar),
		}); err != nil {
			return fmt.Errorf("guardando el plato: %w", err)
		}

		motivos, err := q.MotivosDeImportacion(ctx, fila.ImportacionID)
		if err != nil {
			return fmt.Errorf("recontando las marcas: %w", err)
		}
		for _, m := range motivos {
			switch domain.MotivoRevision(m).Nivel() {
			case domain.NivelRevisar:
				marcas.Revisar++
			case domain.NivelConfirmar:
				marcas.Confirmar++
			case domain.NivelNinguno:
			}
		}

		return q.ActualizarMarcas(ctx, cartadb.ActualizarMarcasParams{
			ID:        fila.ImportacionID,
			Revisar:   int32(marcas.Revisar),
			Confirmar: int32(marcas.Confirmar),
		})
	})
	return plato, marcas, err
}

// Publicar apunta el restaurante a esta importacion y le fija el slug.
//
// Las dos escrituras van en una transaccion: un restaurante con slug pero sin
// importacion publicada devuelve un 404 en una URL que el dueno ya repartio.
//
// El slug se desambigua con un sufijo si otro restaurante ya lo tiene. Se
// comprueba antes de escribir en vez de reaccionar al UNIQUE porque el mensaje
// de error de Postgres no dice cual de los dos indices choco.
func (r *Repo) Publicar(ctx context.Context, importacionID id.ID, base string) (string, error) {
	var slug string

	err := db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		rest, err := q.RestauranteDeImportacion(ctx, importacionID)
		if err != nil {
			if db.SinFilas(err) {
				return ErrNoExiste
			}
			return fmt.Errorf("leyendo el restaurante: %w", err)
		}

		slug = base
		for intento := 2; ; intento++ {
			tomado, err := q.SlugTomado(ctx, cartadb.SlugTomadoParams{
				Slug: &slug, RestauranteID: rest.ID,
			})
			if err != nil {
				return fmt.Errorf("comprobando el slug: %w", err)
			}
			if !tomado {
				break
			}
			slug = fmt.Sprintf("%s-%d", base, intento)
		}

		if err := q.PublicarImportacion(ctx, cartadb.PublicarImportacionParams{
			Slug:          &slug,
			ImportacionID: &importacionID,
			RestauranteID: rest.ID,
		}); err != nil {
			return fmt.Errorf("publicando: %w", err)
		}
		return q.MarcarImportacionPublicada(ctx, importacionID)
	})
	return slug, err
}

// CartaPublica devuelve la carta que se ve en /r/{slug}. SIN token.
func (r *Repo) CartaPublica(ctx context.Context, slug string) (domain.Importacion, error) {
	fila, err := r.q.ImportacionPublicadaPorSlug(ctx, &slug)
	if err != nil {
		if db.SinFilas(err) {
			return domain.Importacion{}, ErrNoExiste
		}
		return domain.Importacion{}, fmt.Errorf("buscando la carta publica: %w", err)
	}

	imp := domain.Importacion{
		ID:     fila.ID,
		Estado: domain.Publicada,
		Restaurante: domain.Restaurante{
			Nombre: fila.RestauranteNombre,
			Slug:   opcional(fila.RestauranteSlug),
		},
	}
	imp.Carta, err = r.leerCarta(ctx, fila.ID)
	if err != nil {
		return imp, err
	}

	// Los ausentes NO salen al publico: la ultima lectura dijo que ya no estan
	// en la carta. Siguen guardados con su foto porque el dueno todavia no ha
	// decidido, pero enseniarle a un cliente un plato que el restaurante quito
	// es como publicar un precio viejo.
	imp.Carta = imp.Carta.SinAusentes()
	return imp, nil
}

// PlatoPorID devuelve un plato suelto.
func (r *Repo) PlatoPorID(ctx context.Context, platoID id.ID) (domain.Plato, error) {
	f, err := r.q.PlatoPorID(ctx, platoID)
	if err != nil {
		if db.SinFilas(err) {
			return domain.Plato{}, ErrNoExiste
		}
		return domain.Plato{}, fmt.Errorf("leyendo el plato: %w", err)
	}
	var precios []domain.Precio
	if err := json.Unmarshal(f.Precios, &precios); err != nil {
		return domain.Plato{}, fmt.Errorf("los precios de %q no son JSON valido: %w", f.Nombre, err)
	}
	return domain.Plato{
		ID:              f.ID,
		Nombre:          f.Nombre,
		Descripcion:     f.Descripcion,
		Categoria:       f.Categoria,
		Precios:         precios,
		Revisar:         domain.MotivoRevision(f.Revisar),
		FotoAjuste:      f.FotoAjuste,
		FotoReferencias: claves(f.FotoReferencias),
	}, nil
}

func (r *Repo) GuardarReferenciasDePlato(ctx context.Context, platoID id.ID, refs []string) error {
	crudo, err := json.Marshal(refs)
	if err != nil {
		return fmt.Errorf("serializando las referencias: %w", err)
	}
	return r.q.GuardarReferenciasDePlato(ctx, cartadb.GuardarReferenciasDePlatoParams{
		PlatoID:     platoID,
		Referencias: crudo,
	})
}

// GuardarImagenes reescribe la lista de paginas de la carta. Es lo que usan
// anadir, quitar y reordenar: las tres son la misma lista escrita entera.
func (r *Repo) GuardarImagenes(ctx context.Context, importacionID id.ID, claves []string) error {
	crudo, err := json.Marshal(claves)
	if err != nil {
		return fmt.Errorf("serializando las paginas: %w", err)
	}
	return r.q.GuardarImagenesDeImportacion(ctx, cartadb.GuardarImagenesDeImportacionParams{
		ID:       importacionID,
		Imagenes: crudo,
	})
}

// GuardarNombre renombra el restaurante de una importacion.
func (r *Repo) GuardarNombre(ctx context.Context, importacionID id.ID, nombre string) error {
	rest, err := r.q.RestauranteDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return ErrNoExiste
		}
		return fmt.Errorf("leyendo el restaurante: %w", err)
	}
	return r.q.GuardarNombreDeRestaurante(ctx, cartadb.GuardarNombreDeRestauranteParams{
		ID:     rest.ID,
		Nombre: nombre,
	})
}

// GuardarTipos cambia los tipos de negocio del restaurante. La lista va
// ordenada: el primero es el principal.
func (r *Repo) GuardarTipos(ctx context.Context, importacionID id.ID, tipos []domain.Tipo) error {
	rest, err := r.q.RestauranteDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return ErrNoExiste
		}
		return fmt.Errorf("leyendo el restaurante: %w", err)
	}

	crudo, err := json.Marshal(comoTexto(tipos))
	if err != nil {
		return fmt.Errorf("serializando los tipos: %w", err)
	}
	return r.q.GuardarTiposDeRestaurante(ctx, cartadb.GuardarTiposDeRestauranteParams{
		ID:    rest.ID,
		Tipos: crudo,
	})
}

func comoTexto(tipos []domain.Tipo) []string {
	fuera := make([]string, 0, len(tipos))
	for _, t := range tipos {
		fuera = append(fuera, string(t))
	}
	return fuera
}

// GuardarBase guarda el estilo del restaurante o de una categoria. categoria
// vacia es la base general.
func (r *Repo) GuardarBase(ctx context.Context, importacionID id.ID, categoria string, e domain.Estilo) error {
	rest, err := r.q.RestauranteDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return ErrNoExiste
		}
		return fmt.Errorf("leyendo el restaurante: %w", err)
	}
	return r.q.GuardarBaseDeFoto(ctx, cartadb.GuardarBaseDeFotoParams{
		ID:                id.Nuevo(),
		RestauranteID:     rest.ID,
		Categoria:         categoria,
		Recipiente:        e.Vajilla.Texto,
		VajillaClave:      e.Vajilla.Foto,
		VajillaVistaClave: e.Vajilla.Vista,
		Fondo:             e.Fondo.Texto,
		FondoClave:        e.Fondo.Foto,
		FondoVistaClave:   e.Fondo.Vista,
	})
}

// EstiloPropio devuelve lo que ESTA categoria tiene escrito, sin heredar nada.
//
// Existe para escribir: quien sube una foto o dibuja una ranura tiene que
// guardar sobre lo propio. Guardando sobre lo plegado, la primera foto que
// subiera una categoria le copiaria dentro todo el texto de la general y esa
// categoria dejaria de seguirla para siempre.
func (r *Repo) EstiloPropio(ctx context.Context, importacionID id.ID, categoria string) (domain.Estilo, error) {
	rest, err := r.q.RestauranteDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return domain.Estilo{}, ErrNoExiste
		}
		return domain.Estilo{}, fmt.Errorf("leyendo el restaurante: %w", err)
	}

	f, err := r.q.EstiloPropio(ctx, cartadb.EstiloPropioParams{
		RestauranteID: rest.ID,
		Categoria:     categoria,
	})
	if err != nil {
		if db.SinFilas(err) {
			return domain.Estilo{}, nil // todavia no hay nada propio
		}
		return domain.Estilo{}, fmt.Errorf("leyendo el estilo propio: %w", err)
	}
	return estiloDeFila(f.Recipiente, f.VajillaClave, f.VajillaVistaClave,
		f.Fondo, f.FondoClave, f.FondoVistaClave), nil
}

// Base devuelve el estilo ya plegado para una categoria, y los tipos del negocio.
func (r *Repo) Base(ctx context.Context, importacionID id.ID, categoria string) ([]domain.Tipo, domain.Estilo, error) {
	rest, err := r.q.RestauranteDeImportacion(ctx, importacionID)
	if err != nil {
		if db.SinFilas(err) {
			return nil, domain.Estilo{}, ErrNoExiste
		}
		return nil, domain.Estilo{}, fmt.Errorf("leyendo el restaurante: %w", err)
	}
	estilo, err := baseDeFoto(ctx, r.q, rest.ID, categoria)
	return domain.TiposValidos(claves(rest.Tipos)), estilo, err
}

// GuardarAjusteFoto guarda la correccion que el dueno escribio para la foto de
// un plato. No regenera nada: eso lo pide el mismo boton de siempre.
func (r *Repo) GuardarAjusteFoto(ctx context.Context, platoID id.ID, ajuste string) error {
	return r.q.GuardarAjusteFoto(ctx, cartadb.GuardarAjusteFotoParams{
		PlatoID: platoID,
		Ajuste:  ajuste,
	})
}

// MarcarPendientes selecciona los platos a los que generarles foto y los deja
// listos para encolar, TODO en una transaccion junto con los jobs.
//
// Si el commit falla, ni los estados ni los jobs existieron: no queda ni un
// plato en 'pendiente' esperando un job que nadie encolo.
func (r *Repo) MarcarPendientes(ctx context.Context, importacionID id.ID, soloEstos []id.ID,
	encolar func(context.Context, pgx.Tx, []id.ID) error) ([]id.ID, error) {
	var elegidos []id.ID

	err := db.EnTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		ids, err := q.PlatosGenerables(ctx, cartadb.PlatosGenerablesParams{
			ImportacionID: importacionID,
			SoloEstos:     soloEstos,
		})
		if err != nil {
			return fmt.Errorf("buscando los platos generables: %w", err)
		}
		if len(ids) == 0 {
			return nil
		}

		if err := q.MarcarFotosPendientes(ctx, ids); err != nil {
			return fmt.Errorf("marcando pendientes: %w", err)
		}
		if err := encolar(ctx, tx, ids); err != nil {
			return fmt.Errorf("encolando las fotos: %w", err)
		}
		elegidos = ids
		return nil
	})
	return elegidos, err
}

// intervalo convierte una duracion de Go al interval de Postgres.
func intervalo(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// ImportacionDePlato dice a que carta pertenece un plato.
//
// Los endpoints de foto reciben un plato, pero el token que autoriza es de la
// importacion. Esto resuelve el dueno antes de comprobar nada.
func (r *Repo) ImportacionDePlato(ctx context.Context, platoID id.ID) (id.ID, error) {
	impID, err := r.q.ImportacionDePlato(ctx, platoID)
	if err != nil {
		if db.SinFilas(err) {
			return id.Nulo, ErrNoExiste
		}
		return id.Nulo, fmt.Errorf("buscando la importacion del plato: %w", err)
	}
	return impID, nil
}
