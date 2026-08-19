// Package worker corre en segundo plano lo que tarda demasiado para una
// peticion HTTP.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/riverqueue/river"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/jobs"
)

// LeerCartaArgs es lo unico que viaja en el job: el id.
//
// Las imagenes NO van aqui. Un job con cuatro fotos de 10 MB dentro son 40 MB de
// JSON en una fila de Postgres, replicados a cada reintento. Van al almacen y el
// worker las relee por su clave.
type LeerCartaArgs struct {
	ImportacionID id.ID `json:"importacion_id"`
}

func (LeerCartaArgs) Kind() string { return "carta.leer" }

func (LeerCartaArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: jobs.ColaLecturas,

		// SIN UniqueOpts, y es deliberado.
		//
		// Estaba para que dos peticiones solapadas no encolaran dos lecturas de
		// la misma carta y se pagara dos veces. Eso hoy lo impide el ESTADO, que
		// es una guarda mas fuerte porque vive en la misma transaccion que el
		// cambio: subir las hojas solo muerde desde 'nueva' o 'fallida', y
		// releer solo desde 'lista'. La segunda peticion no llega ni a encolar.
		//
		// Y era activamente daniino: River descarta el duplicado SIN error, asi
		// que una carta ya leida no se podia volver a leer y la importacion se
		// quedaba en 'leyendo' para siempre esperando un job que nunca existio.
		// Quitar 'completed' de ByState no bastaba; el insert se seguia
		// descartando aunque el indice ya no lo bloqueara.

		// TRES intentos, no los 25 de River por defecto. Cada intento cuesta
		// $0.017: 25 serian $0.43 quemados contra una pared. Tres cubre el 429
		// pasajero y el 500 de Gemini, que es lo que de verdad pasa.
		MaxAttempts: 3,
	}
}

// Almacen es lo que el worker necesita para releer las imagenes.
type Almacen interface {
	Abrir(clave string) (*os.File, error)
}

// Repo es lo que el worker necesita de la persistencia.
type Repo interface {
	ClavesDeImagenes(ctx context.Context, importacionID id.ID) ([]string, error)
	// MarcarEtapa dice en que va la lectura, para que la pantalla no tenga que
	// inventar una barra de progreso sobre diez segundos de espera.
	MarcarEtapa(ctx context.Context, importacionID id.ID, etapa int16) error

	// GuardarTipos propone los tipos de negocio que se dedujeron de la carta.
	GuardarTipos(ctx context.Context, importacionID id.ID, tipos []domain.Tipo) error

	GuardarCarta(ctx context.Context, importacionID id.ID, carta domain.Carta,
		cruda []byte, marcas domain.Marcas) error

	// BancoDePlatos es el catalogo de platos tipicos. Se lee entero una vez por
	// carta: la regla de emparejamiento vive en el dominio y necesita ver todos
	// los candidatos.
	BancoDePlatos(ctx context.Context) ([]domain.PlatoTipico, error)
	MarcarFallida(ctx context.Context, importacionID id.ID, motivo string) error
}

// Lector y Organizador se declaran como interfaces para poder probar el worker
// sin gastar dinero.
type Lector interface {
	Ejecutar(ctx context.Context, imagenes []ai.Imagen) (*app.Resultado, error)
}

type Organizador interface {
	Ejecutar(ctx context.Context, c domain.Carta) (domain.Carta, ai.Uso, error)
}

// Conocedor amplia el banco de platos con lo que esta carta trae y el banco
// todavia no sabe. Interfaz aqui para poder probar el worker sin gastar dinero.
type Conocedor interface {
	Aplicar(ctx context.Context, c *domain.Carta,
		banco []domain.PlatoTipico) (emparejados, aprendidos int, err error)
}

// Atribuir marca el contexto para que el gasto de las llamadas a la IA se cargue
// a esta importacion.
//
// Se recibe como funcion en vez de importar el adaptador de Postgres, que es
// quien sabe como se marca: el worker no tiene por que conocer la persistencia.
//
// SIN ESTO EL COSTO SE PIERDE, y de la peor manera: se registra igual en gasto_ia
// pero con importacion_id NULL, asi que la carta dice que costo $0.00 mientras el
// log del worker dice $0.0147. Paso de verdad la primera vez que el worker
// funciono.
type Atribuir func(ctx context.Context, importacionID id.ID) context.Context

type LeerCarta struct {
	river.WorkerDefaults[LeerCartaArgs]

	repo        Repo
	almacen     Almacen
	lector      Lector
	organizador Organizador
	conocedor   Conocedor
	atribuir    Atribuir
	log         *slog.Logger
}

func NuevoLeerCarta(repo Repo, almacen Almacen, lector Lector,
	organizador Organizador, conocedor Conocedor, atribuir Atribuir,
	log *slog.Logger) *LeerCarta {
	return &LeerCarta{repo: repo, almacen: almacen, lector: lector,
		organizador: organizador, conocedor: conocedor, atribuir: atribuir, log: log}
}

// Work lee la carta, la ordena para vender y la guarda.
//
// Todo el resultado se escribe en UNA transaccion que empieza borrando los
// platos anteriores (ver Repo.GuardarCarta), asi que reintentar el job entero es
// seguro por construccion: no hay que comparar nada ni deduplicar.
func (w *LeerCarta) Work(ctx context.Context, job *river.Job[LeerCartaArgs]) error {
	impID := job.Args.ImportacionID

	// Todo lo que cueste dinero de aqui en adelante se carga a esta importacion.
	// Va lo primero para que no haya ninguna llamada antes del marcado.
	ctx = w.atribuir(ctx, impID)

	// El progreso se anota pero NUNCA se comprueba: si falla, el dueno ve una
	// etapa vieja, y eso es infinitamente mejor que perder la lectura entera.
	etapa := func(e int16) {
		if err := w.repo.MarcarEtapa(ctx, impID, e); err != nil {
			w.log.Warn("no se pudo anotar la etapa", "importacion", impID, "etapa", e, "error", err)
		}
	}

	etapa(domain.EtapaArchivos)
	imagenes, claves, err := w.leerImagenes(ctx, impID)
	if err != nil {
		// Sin las imagenes no hay nada que reintentar: o el almacen las perdio
		// o las claves estan mal, y ninguna de las dos se arregla esperando.
		return w.rendirse(ctx, impID, err)
	}

	etapa(domain.EtapaLeyendo)
	res, err := w.lector.Ejecutar(ctx, imagenes)
	if err != nil {
		// Aqui SI se reintenta: un 429 o un 500 de Gemini suelen pasar solos.
		// Solo en el ultimo intento se marca fallida, para que el dueno no vea
		// "fallida" durante los reintentos y crea que ya no hay nada que hacer.
		if job.Attempt >= job.MaxAttempts {
			return w.rendirse(ctx, impID, err)
		}
		return fmt.Errorf("leyendo la carta (intento %d de %d): %w",
			job.Attempt, job.MaxAttempts, err)
	}

	// El cruce de precios ya lo hizo el caso de uso al leer; aqui solo se anota,
	// porque es la etapa que al dueno le importa ver.
	etapa(domain.EtapaCruzandoPrecios)
	carta := res.Carta

	// De que hoja salio cada plato. El modelo dice un numero —el de la imagen
	// que recibio— y aqui se traduce a la clave real, que es lo unico que sirve
	// para quitar una hoja con sus platos o para releer solo una.
	domain.AtribuirHojas(&carta, claves)

	// Ordenar para vender es una MEJORA, no un requisito: si falla, se guarda la
	// carta en el orden de lectura y se sigue. Tumbar una extraccion de $0.017
	// porque el reordenado de $0.001 fallo seria absurdo.
	etapa(domain.EtapaOrdenando)
	if ordenada, _, err := w.organizador.Ejecutar(ctx, carta); err != nil {
		w.log.Warn("no se pudo ordenar la carta, se guarda como salio",
			"importacion", impID, "error", err)
	} else {
		carta = ordenada
	}

	// QUE ES cada plato, antes de guardarlo. Aqui es donde la carta deja de ser
	// una lista de nombres: sin esto la guarnicion la pone el tipo de negocio y
	// una gaseosa sale con camote y choclo al lado.
	//
	// Un fallo del banco NO tumba la lectura, que ya esta pagada: se guarda sin
	// emparejar y las fotos salen como salian antes.
	// Lo que el banco no sabe se le pregunta UNA vez y se guarda, asi que el
	// siguiente restaurante que traiga ese plato ya no lo pregunta.
	//
	// Falla suave, como organizar: una carta cuyos platos no se reconocen se
	// publica igual con las fotos que se hacian antes, y tumbar una lectura ya
	// pagada por esto seria absurdo.
	etapa(domain.EtapaConociendo)
	emparejados, aprendidos := 0, 0
	if banco, err := w.repo.BancoDePlatos(ctx); err != nil {
		w.log.Warn("no se pudo leer el banco de platos", "importacion", impID, "error", err)
	} else if emparejados, aprendidos, err = w.conocedor.Aplicar(ctx, &carta, banco); err != nil {
		w.log.Warn("no se pudo ampliar el banco de platos",
			"importacion", impID, "error", err)
	}

	cruda, err := json.Marshal(res.Carta)
	if err != nil {
		return fmt.Errorf("serializando la carta cruda: %w", err)
	}

	// Clasificar va ANTES de guardar, y no por gusto: la pantalla marca como
	// hechas todas las etapas anteriores a la que recibe, asi que emitirla
	// despues hacia que la lista diera por reconocido el tipo de restaurante
	// mientras todavia estaba ordenando, y luego retrocediera.
	//
	// Se puede mover porque los tipos van a la fila del restaurante y la carta a
	// las de plato: no dependen uno del otro. Los tipos se proponen, no se
	// imponen: el dueno los cambia con un clic. Si falla, se queda vacio, que no
	// inventa guarniciones.
	etapa(domain.EtapaClasificando)
	tipos := domain.InferirTipos(carta)
	if err := w.repo.GuardarTipos(ctx, impID, tipos); err != nil {
		w.log.Warn("no se pudieron guardar los tipos de restaurante",
			"importacion", impID, "tipos", tipos, "error", err)
	}

	etapa(domain.EtapaGuardando)
	if err := w.repo.GuardarCarta(ctx, impID, carta, cruda, res.Marcas); err != nil {
		return fmt.Errorf("guardando la carta: %w", err)
	}

	etapa(domain.EtapaTerminada)

	w.log.Info("carta leida",
		"importacion", impID,
		"platos", len(carta.Platos()),
		"emparejados", emparejados,
		"aprendidos", aprendidos,
		"categorias", len(carta.Categorias),
		"revisar", res.Marcas.Revisar,
		"confirmar", res.Marcas.Confirmar,
		"tipos", tipos,
		"costo_usd", res.Uso.CostoUSD,
		"modelo", res.Uso.Modelo,
	)
	return nil
}

// rendirse marca la importacion como fallida y CANCELA el job.
//
// Devuelve river.JobCancel y no un error normal: un job que va a fallar
// identicamente para siempre no tiene que ocupar la cola tres veces. El motivo
// queda guardado porque sin el, el dueno ve "fallida" y nadie sabe por que.
func (w *LeerCarta) rendirse(ctx context.Context, impID id.ID, causa error) error {
	if err := w.repo.MarcarFallida(ctx, impID, causa.Error()); err != nil {
		w.log.Error("no se pudo marcar la importacion como fallida",
			"importacion", impID, "error", err)
	}
	return river.JobCancel(causa)
}

// leerImagenes trae del almacen las fotos que subio el dueno.
// Devuelve tambien las claves, en el MISMO orden en que van las imagenes: es lo
// que permite traducir el numero de hoja que dice el modelo a la hoja de verdad.
func (w *LeerCarta) leerImagenes(ctx context.Context, impID id.ID) ([]ai.Imagen, []string, error) {
	claves, err := w.repo.ClavesDeImagenes(ctx, impID)
	if err != nil {
		return nil, nil, fmt.Errorf("buscando las imagenes: %w", err)
	}
	if len(claves) == 0 {
		return nil, nil, fmt.Errorf("la importacion no tiene imagenes")
	}

	imagenes := make([]ai.Imagen, 0, len(claves))
	for _, clave := range claves {
		f, err := w.almacen.Abrir(clave)
		if err != nil {
			return nil, nil, fmt.Errorf("abriendo %s: %w", clave, err)
		}
		bytes, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("leyendo %s: %w", clave, err)
		}
		imagenes = append(imagenes, ai.Imagen{Bytes: bytes, MIME: mimeDe(clave)})
	}
	return imagenes, claves, nil
}

// mimeDe deduce el tipo por la extension.
//
// La extension la puso el servidor al guardar, a partir del tipo SNIFFEADO de lo
// que subio el dueno. O sea que aqui ya es un dato de confianza, no una cabecera
// que mando un cliente.
func mimeDe(clave string) string {
	switch filepath.Ext(clave) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	}
	return "image/jpeg"
}
