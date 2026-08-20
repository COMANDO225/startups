package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/jobs"
)

// GenerarFotoArgs identifica UN plato. Un job por foto, no uno por carta.
//
// Es lo que hace que cada tarjeta se llene por su cuenta: el plato 37 puede
// terminar antes que el 12, un fallo deja a los otros 59 intactos, y el que
// falla se reintenta solo.
type GenerarFotoArgs struct {
	PlatoID       id.ID `json:"plato_id"`
	ImportacionID id.ID `json:"importacion_id"`

	// Corregir edita la foto actual en vez de hacer una nueva. Va en los args y
	// no en la fila del plato porque es de ESTA peticion: el mismo plato se
	// corrige hoy y se regenera de cero manana.
	Corregir bool `json:"corregir,omitempty"`
}

func (GenerarFotoArgs) Kind() string { return "carta.generar_foto" }

func (GenerarFotoArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: jobs.ColaFotos,

		// Unico por plato mientras el job siga EN VUELO.
		//
		// ByState HAY QUE PONERLO: el default de River incluye 'completed', y con
		// eso un plato ya generado no admitia otro job NUNCA — "Regenerar"
		// respondia 202, el plato quedaba en 'pendiente' y no se creaba nada.
		//
		// Que no se pague dos veces por una foto hecha lo garantiza
		// PlatosGenerables, no esto.
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
				rivertype.JobStateRetryable,
			},
		},

		// DOS intentos. El default de River son 25, y 25 llamadas pagadas contra
		// una pared son $0.84 en un solo plato.
		MaxAttempts: 2,
	}
}

// Los topes del claim —intentos, ventana de rescate y fotos en vuelo por
// restaurante— NO viven aqui: viven en el adaptador de Postgres, junto a la
// consulta que los aplica. El worker declara la intencion ("reclamalo si me
// toca") y el adaptador sabe como se expresa eso contra la base.
//
// Tenerlos en los dos sitios seria un valor que hay que acordarse de cambiar dos
// veces, y el dia que no coincidan el sintoma seria pagar de mas en silencio.

// RepoFoto es lo que el worker necesita para generar una foto.
type RepoFoto interface {
	// ReclamarFoto marca el plato como 'generando' si le toca. Es lo que se hace
	// ANTES de pagar.
	//
	// Devuelve el encargo entero porque tipo y base se leen en la MISMA
	// transaccion: un cambio de estilo a mitad de "generar todas" no puede dejar
	// media carta con cada uno.
	ReclamarFoto(ctx context.Context, platoID, importacionID id.ID) (domain.EncargoDeFoto, bool, error)

	// ReservarPresupuesto cobra por adelantado. false = se acabo.
	ReservarPresupuesto(ctx context.Context, importacionID id.ID, costo dinero.MicrosUSD) (bool, error)

	MarcarFotoLista(ctx context.Context, platoID id.ID, origen domain.OrigenFoto, clave string) error
	MarcarFotoConEstado(ctx context.Context, platoID id.ID, estado domain.EstadoFoto) error
}

// AlmacenFoto guarda la imagen generada.
type AlmacenFoto interface {
	Guardar(ctx context.Context, clave string, bytes []byte) error
	Borrar(ctx context.Context, clave string) error
}

// Generador pide la imagen a la IA.
type Generador interface {
	Ejecutar(ctx context.Context, e domain.EncargoDeFoto) (ai.Imagen, ai.Uso, error)
}

type GenerarFoto struct {
	river.WorkerDefaults[GenerarFotoArgs]

	repo      RepoFoto
	almacen   AlmacenFoto
	generador Generador
	atribuir  Atribuir

	// costoPorFoto es lo que se reserva antes de llamar. Sale de la tabla de
	// precios del modelo que encabeza la cadena.
	costoPorFoto dinero.MicrosUSD

	log *slog.Logger
}

func NuevoGenerarFoto(repo RepoFoto, almacen AlmacenFoto, generador Generador,
	atribuir Atribuir, costoPorFoto dinero.MicrosUSD, log *slog.Logger) *GenerarFoto {
	return &GenerarFoto{repo: repo, almacen: almacen, generador: generador,
		atribuir: atribuir, costoPorFoto: costoPorFoto, log: log}
}

// Work genera la foto de un plato.
//
// El orden de los pasos NO es negociable, y cada uno existe por una razon:
//
//  1. claim      -> no pagar por lo que ya se pago o no toca
//  2. reserva    -> no pasarse del presupuesto de la carta
//  3. IA         -> lo unico que cuesta dinero
//  4. guardar    -> lo primero tras recibir los bytes, para achicar la ventana
//  5. marcar     -> recien ahora el plato queda 'lista'
func (w *GenerarFoto) Work(ctx context.Context, job *river.Job[GenerarFotoArgs]) error {
	platoID, impID := job.Args.PlatoID, job.Args.ImportacionID

	// 1. El claim. Si devuelve false, otro worker lo tiene, ya se genero, o se
	// agotaron los intentos. En los tres casos: retirarse SIN pagar.
	encargo, mio, err := w.repo.ReclamarFoto(ctx, platoID, impID)
	if err != nil {
		return fmt.Errorf("reclamando el plato: %w", err)
	}
	if !mio {
		// Puede ser tambien que este restaurante ya tenga sus 4 en vuelo. En ese
		// caso NO es un fallo: se pospone y el worker agarra el siguiente job,
		// que tarde o temprano es de otro restaurante.
		return river.JobSnooze(5 * time.Second)
	}

	// 2. La reserva. Se cobra ANTES de gastar: si se comprobara despues, 60
	// workers concurrentes leerian todos "aun queda" y se pasarian todos.
	hay, err := w.repo.ReservarPresupuesto(ctx, impID, w.costoPorFoto)
	if err != nil {
		return fmt.Errorf("reservando presupuesto: %w", err)
	}
	if !hay {
		// Se acabo el presupuesto de ESTA carta. Se marca y se devuelve nil, no
		// error: un job que va a fallar identicamente para siempre no se
		// reintenta. Los que si salieron se quedan, y la pantalla muestra el
		// motivo honesto en los que faltan.
		if err := w.repo.MarcarFotoConEstado(ctx, platoID, domain.FotoSinCredito); err != nil {
			w.log.Error("no se pudo marcar sin presupuesto", "plato", platoID, "error", err)
		}
		w.log.Info("presupuesto agotado", "importacion", impID, "plato", encargo.Plato.Nombre)
		return nil
	}

	// 3. La llamada pagada. Desde aqui el dinero ya se gasto pase lo que pase.
	ctx = w.atribuir(ctx, impID)
	encargo.Corregir = job.Args.Corregir
	img, uso, err := w.generador.Ejecutar(ctx, encargo)
	if err != nil {
		w.fallo(ctx, platoID, err)
		if job.Attempt >= job.MaxAttempts {
			return river.JobCancel(err)
		}
		return fmt.Errorf("generando la foto de %q: %w", encargo.Plato.Nombre, err)
	}

	// 4. Guardar lo PRIMERO tras recibir los bytes.
	//
	// Entre la respuesta del modelo y este guardado hay una ventana en la que un
	// proceso que muere pierde una foto ya pagada. Gemini no ofrece clave de
	// idempotencia para imagenes, asi que no se puede cerrar: solo achicar. Son
	// ~200 ms sobre 4.2 s y $0.0336, y foto_intentos < 2 garantiza que no se
	// repita mas de una vez.
	clave, err := app.GuardarFoto(ctx, w.almacen,
		app.ClaveDeFoto(encargo.RestauranteID, platoID), img.Bytes)
	if err != nil {
		w.fallo(ctx, platoID, err)
		return fmt.Errorf("guardando la foto de %q: %w", encargo.Plato.Nombre, err)
	}

	// 5. Recien ahora la tarjeta se llena en la pantalla.
	if err := w.repo.MarcarFotoLista(ctx, platoID, domain.FotoDeIA, clave); err != nil {
		return fmt.Errorf("marcando la foto lista: %w", err)
	}

	// La anterior se va DESPUES de que la fila apunte a la nueva: regenerar
	// escribe siempre una clave nueva —por la cache del navegador— asi que sin
	// esto cada correccion deja la version vieja en el bucket para siempre.
	if err := app.BorrarFoto(ctx, w.almacen, encargo.Plato.Foto.Clave); err != nil {
		w.log.WarnContext(ctx, "no se pudo borrar la foto anterior", "error", err)
	}

	w.log.Info("foto generada",
		"plato", encargo.Plato.Nombre, "importacion", impID, "tipos", encargo.Tipos,
		"costo_usd", uso.CostoUSD, "modelo", uso.Modelo, "kb", len(img.Bytes)/1024)
	return nil
}

// fallo deja el plato en 'error', que en la pantalla es el recuadro con su boton
// de reintentar. No se traga el error: solo registra el estado.
func (w *GenerarFoto) fallo(ctx context.Context, platoID id.ID, causa error) {
	if err := w.repo.MarcarFotoConEstado(ctx, platoID, domain.FotoConError); err != nil {
		w.log.Error("no se pudo marcar la foto con error",
			"plato", platoID, "causa", causa, "error", err)
	}
}

func extensionDe(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ".jpg"
}

// GeneradorIA pide la foto a la capa de IA usando el prompt del modulo.
type GeneradorIA struct {
	ia  *ai.Cliente
	alm LectorDeAlmacen
	log *slog.Logger
}

// LectorDeAlmacen lee las fotos de ejemplo.
type LectorDeAlmacen interface {
	Leer(ctx context.Context, clave string) ([]byte, string, error)
}

func NuevoGeneradorIA(ia *ai.Cliente, alm LectorDeAlmacen, log *slog.Logger) *GeneradorIA {
	return &GeneradorIA{ia: ia, alm: alm, log: log}
}

// Una referencia ilegible NO cancela la foto: el prompt por si solo ya produce
// una correcta.
func (g *GeneradorIA) referencias(ctx context.Context, claves []string) ([]ai.Imagen, error) {
	if len(claves) == 0 {
		return nil, nil
	}
	fuera := make([]ai.Imagen, 0, len(claves))
	for _, c := range claves {
		bytes, mime, err := g.alm.Leer(ctx, c)
		if err != nil {
			g.log.Warn("no se pudo leer una foto de ejemplo, se genera sin ella",
				"clave", c, "error", err)
			continue
		}
		fuera = append(fuera, ai.Imagen{Bytes: bytes, MIME: mime})
	}
	return fuera, nil
}

var errSinImagen = errors.New("el proveedor no devolvio ninguna imagen")

// Pintar genera una imagen desde un prompt pelado, sin plato ni receta detras.
//
// Vive aqui y no en un adaptador propio porque lo unico que necesita es el
// cliente de IA que este tipo ya tiene, y porque asi la vista previa del estilo
// sale del MISMO modelo y la misma proporcion que las fotos de los platos: una
// vista previa generada por otro camino dejaria de predecir lo que va a salir.
func (g *GeneradorIA) Pintar(ctx context.Context, prompt string) ([]byte, string, error) {
	resp, err := g.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:      ai.GenerarFoto,
		Prompt:     prompt,
		Proporcion: "1:1",
	})
	if err != nil {
		return nil, "", err
	}
	if len(resp.Imagenes) == 0 {
		return nil, "", errSinImagen
	}
	return resp.Imagenes[0].Bytes, resp.Imagenes[0].MIME, nil
}

func (g *GeneradorIA) Ejecutar(ctx context.Context, e domain.EncargoDeFoto) (ai.Imagen, ai.Uso, error) {
	prompt, claves := app.PromptFoto(e.Plato, e.Tipos, e.Base),
		app.Referencias(e.Plato, e.Tipos, e.Base)

	// Corregir solo tiene sentido si hay foto que corregir y algo que pedirle.
	// Sin cualquiera de las dos se genera de cero, que es el comportamiento de
	// siempre y nunca deja al dueno sin foto.
	if e.Corregir && e.Plato.Foto.Clave != "" && strings.TrimSpace(e.Plato.FotoAjuste) != "" {
		prompt = app.PromptCorreccion(e.Plato.FotoAjuste)
		claves = []string{e.Plato.Foto.Clave}
	}

	// Aqui y no en el claim: son bytes, y el claim es una transaccion corta con
	// un lock tomado.
	refs, err := g.referencias(ctx, claves)
	if err != nil {
		return ai.Imagen{}, ai.Uso{}, err
	}

	resp, err := g.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:       ai.GenerarFoto,
		Prompt:      prompt,
		Referencias: refs,

		// 1:1 porque la tarjeta del catalogo es cuadrada. Sin fijarlo, Gemini
		// devuelve 16:9 y la foto sale recortada en la pantalla.
		Proporcion: "1:1",
	})
	if err != nil {
		return ai.Imagen{}, resp.Uso, err
	}
	if len(resp.Imagenes) == 0 {
		return ai.Imagen{}, resp.Uso, errSinImagen
	}
	return resp.Imagenes[0], resp.Uso, nil
}
