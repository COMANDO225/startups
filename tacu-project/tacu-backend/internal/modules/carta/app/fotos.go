package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// RepoFotos es lo que el caso de uso necesita de la persistencia.
type RepoFotos interface {
	// MarcarPendientes elige los platos, los marca y encola sus jobs TODO en la
	// misma transaccion. Si el commit falla, ni los estados ni los jobs
	// existieron: no queda un plato en 'pendiente' esperando un job que nadie
	// encolo.
	MarcarPendientes(ctx context.Context, importacionID id.ID, soloEstos []id.ID,
		encolar func(context.Context, pgx.Tx, []id.ID) error) ([]id.ID, error)

	// PlatoPorID hace falta para saber que archivo hay que borrar al reemplazar
	// o quitar una foto: la clave vive en la fila del plato.
	PlatoPorID(ctx context.Context, platoID id.ID) (domain.Plato, error)

	// RestauranteDePlato abre la clave del objeto: el almacen guarda por tenant.
	RestauranteDePlato(ctx context.Context, platoID id.ID) (id.ID, error)

	MarcarFotoLista(ctx context.Context, platoID id.ID, origen domain.OrigenFoto, clave string) error
	MarcarFotoConEstado(ctx context.Context, platoID id.ID, estado domain.EstadoFoto) error
	ImportacionDePlato(ctx context.Context, platoID id.ID) (id.ID, error)
	GuardarAjusteFoto(ctx context.Context, platoID id.ID, ajuste string) error
}

// EncoladorFotos mete los jobs DENTRO de la transaccion que marca los platos.
//
// Recibe la transaccion a proposito: encolar despues del commit deja una ventana
// en la que los platos estan 'pendiente' y no hay job que los atienda, y si el
// proceso muere justo ahi se quedan asi para siempre.
type EncoladorFotos interface {
	EncolarFotos(ctx context.Context, tx pgx.Tx, importacionID id.ID, platos []id.ID, corregir bool) error
}

// Fotos orquesta la generacion, la subida y el borrado.
type Fotos struct {
	repo    RepoFotos
	cola    EncoladorFotos
	almacen Almacen
}

func NuevasFotos(repo RepoFotos, cola EncoladorFotos, almacen Almacen) *Fotos {
	return &Fotos{repo: repo, cola: cola, almacen: almacen}
}

// Generar encola las fotos que faltan.
//
// Con soloEstos vacio son todas las que se puedan. "Las que se puedan" excluye
// SIEMPRE las que subio el dueno: la suya es mejor que cualquier cosa que
// generemos y pisarla seria destruir trabajo suyo.
//
// corregir edita la foto actual en vez de hacer una nueva. Solo aplica con platos
// concretos: "generar todas" nunca corrige.
func (uc *Fotos) Generar(ctx context.Context, importacionID id.ID, soloEstos []id.ID, corregir bool) (int, error) {
	// Un slice VACIO no es lo mismo que nil aqui abajo: la consulta compara
	// contra un array de Postgres, y `id = ANY('{}')` es falso para todos los
	// platos mientras que `'{}' IS NULL` tambien es falso. O sea que un slice
	// vacio no selecciona nada en vez de seleccionar todo.
	//
	// Paso de verdad la primera vez que se pidio "generar todas": el endpoint
	// respondia 202 con encoladas=0 y ninguna foto se generaba, sin un solo
	// error en ningun log.
	if len(soloEstos) == 0 {
		soloEstos = nil
	}

	elegidos, err := uc.repo.MarcarPendientes(ctx, importacionID, soloEstos,
		func(ctx context.Context, tx pgx.Tx, platos []id.ID) error {
			return uc.cola.EncolarFotos(ctx, tx, importacionID, platos, corregir && len(soloEstos) > 0)
		})
	if err != nil {
		return 0, err
	}
	return len(elegidos), nil
}

// SubirPropia guarda la foto que trae el dueno.
//
// Queda con origen 'propia', que es lo que hace que "generar todas" no la pise
// nunca mas.
func (uc *Fotos) SubirPropia(ctx context.Context, platoID id.ID, bytes []byte, mime string) error {
	plato, err := uc.repo.PlatoPorID(ctx, platoID)
	if err != nil {
		return err
	}

	restaurante, err := uc.repo.RestauranteDePlato(ctx, platoID)
	if err != nil {
		return err
	}

	clave, err := GuardarFoto(ctx, uc.almacen, ClaveDeFoto(restaurante, platoID), bytes)
	if err != nil {
		return fmt.Errorf("guardando la foto: %w", err)
	}
	if err := uc.repo.MarcarFotoLista(ctx, platoID, domain.FotoPropia, clave); err != nil {
		return err
	}

	// La anterior se va DESPUES de que la fila apunte a la nueva. Al reves, un
	// fallo al escribir la fila dejaria al plato apuntando a un archivo que
	// acabamos de borrar.
	return BorrarFoto(ctx, uc.almacen, plato.Foto.Clave)
}

// maxAjuste topea lo que el dueno puede escribir para corregir una foto.
//
// No es por la base sino por el prompt: el ajuste se le suma a una plantilla que
// ya trae recipiente, escala, camara, luz y encuadre, y un texto largo compite
// con todo eso. 500 caracteres dan de sobra para "el ceviche va con mas cancha y
// camote grueso" sin dejar pegar una novela que ahogue la receta.
const maxAjuste = 500

var ErrAjusteLargo = errors.New("el ajuste de la foto es demasiado largo")

// AjustarFoto guarda la correccion del dueno para la foto de un plato.
//
// GUARDAR Y REGENERAR SON DOS PASOS, y esa separacion es deliberada: el dueno
// puede corregir el texto tres veces antes de gastar los $0.0336, y regenerar
// sigue siendo el mismo boton de siempre en vez de una segunda ruta que encola.
func (uc *Fotos) AjustarFoto(ctx context.Context, platoID id.ID, ajuste string) error {
	ajuste = strings.TrimSpace(ajuste)
	if len(ajuste) > maxAjuste {
		return fmt.Errorf("%w: %d caracteres, el maximo es %d", ErrAjusteLargo, len(ajuste), maxAjuste)
	}
	return uc.repo.GuardarAjusteFoto(ctx, platoID, ajuste)
}

// Quitar devuelve el plato al recuadro gris.
//
// El archivo NO se borra del almacen a proposito: es barato dejarlo y borrarlo
// abre la puerta a que un fallo a mitad deje la fila apuntando a un archivo que
// ya no esta. Un barrido de huerfanos es un trabajo aparte, sin prisa.
func (uc *Fotos) Quitar(ctx context.Context, platoID id.ID) error {
	plato, err := uc.repo.PlatoPorID(ctx, platoID)
	if err != nil {
		return err
	}
	if err := uc.repo.MarcarFotoConEstado(ctx, platoID, domain.SinFoto); err != nil {
		return err
	}
	return BorrarFoto(ctx, uc.almacen, plato.Foto.Clave)
}

func extensionDeImagen(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ".jpg"
}
