package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

var (
	ErrDemasiadasPaginas = errors.New("ya no caben mas paginas")
	ErrCartaOcupada      = errors.New("la carta todavia se esta leyendo")
	ErrPaginaDesconocida = errors.New("esa pagina no es de esta carta")
)

type RepoPaginas interface {
	// RestauranteDeImportacion abre la clave del objeto: el almacen guarda por
	// tenant.
	RestauranteDeImportacion(ctx context.Context, importacionID id.ID) (id.ID, error)

	// BorrarPlatosDeLaHoja borra los platos que salieron de una hoja. Solo se
	// llama cuando el dueno lo eligio en el aviso, y no tiene vuelta atras.
	BorrarPlatosDeLaHoja(ctx context.Context, importacionID id.ID, hoja string) (int, error)

	Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error)
	GuardarImagenes(ctx context.Context, importacionID id.ID, claves []string) error
}

// PaginasUC gestiona las hojas de la carta de papel: anadir, quitar, reordenar.
//
// NINGUNA de las tres lee nada. El dueno edita sus hojas y despues pulsa "leer
// mi carta", que dispara UNA lectura sobre la carta entera. Antes anadir una
// hoja la leia sola y sumaba sus platos sin cruzar con los que ya habia: eran
// dos formas distintas de meter platos, con reglas distintas, y ninguna
// explicaba por que quitar una hoja no deshacia nada.
type PaginasUC struct {
	repo    RepoPaginas
	almacen Almacen
}

func NuevasPaginas(repo RepoPaginas, almacen Almacen) *PaginasUC {
	return &PaginasUC{repo: repo, almacen: almacen}
}

// MaxPaginas son las hojas que puede tener una carta. Cuatro cubre un menu de
// dos hojas por las dos caras, que es lo mas grande que hemos visto.
const MaxPaginas = 4

// Agregar guarda una hoja mas. NO la lee.
//
// Antes la mandaba a leer sola, y esa era una segunda forma de meter platos en
// la carta con reglas distintas de la lectura completa: sumaba sin cruzar, y el
// dueno no entendia por que quitar una hoja no deshacia nada. Ahora las hojas se
// editan —anadir, quitar, reordenar— y la lectura la dispara el dueno una vez,
// sobre la carta entera.
func (uc *PaginasUC) Agregar(ctx context.Context, impID id.ID, bytes []byte, mime string) ([]string, error) {
	imp, err := uc.repo.Obtener(ctx, impID)
	if err != nil {
		return nil, err
	}

	// Mientras la lectura inicial esta en vuelo NO se puede anadir: esa lectura
	// termina borrando los platos anteriores, asi que los de la pagina nueva se
	// perderian sin que nada avise.
	if imp.Estado != domain.Lista {
		return nil, fmt.Errorf("%w: %s", ErrCartaOcupada, imp.Estado)
	}
	if len(imp.Imagenes) >= MaxPaginas {
		return nil, fmt.Errorf("%w: el maximo son %d", ErrDemasiadasPaginas, MaxPaginas)
	}

	ext, ok := extensionDe(mime)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrImagenInvalida, mime)
	}

	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return nil, err
	}

	// Nombre irrepetible y no "3.jpg": quitar la pagina 2 y subir otra chocaria
	// contra el archivo viejo y el dueno veria la hoja equivocada.
	clave := ClaveDeHoja(restaurante, impID, ext)
	if err := GuardarHoja(ctx, uc.almacen, clave, bytes); err != nil {
		return nil, fmt.Errorf("guardando la pagina: %w", err)
	}

	claves := append(imp.Imagenes, clave)
	if err := uc.repo.GuardarImagenes(ctx, impID, claves); err != nil {
		return nil, err
	}

	return claves, nil
}

// Quitar saca una pagina de la lista.
//
// El archivo NO se borra del almacen. Los platos de esa hoja tampoco se borran
// aqui: se marcan ausentes, que es lo mismo que hace una lectura cuando un
// plato deja de aparecer, y el dueno decide. Borrar por nuestra cuenta un plato
// que el ya fotografio es tirar $0.0336 suyos por una suposicion nuestra.
//
// Los platos sin hoja atribuida —las filas anteriores a la atribucion— no se
// tocan: no sabemos si eran de esta hoja.
func (uc *PaginasUC) Quitar(ctx context.Context, impID id.ID, clave string) ([]string, error) {
	imp, err := uc.repo.Obtener(ctx, impID)
	if err != nil {
		return nil, err
	}

	claves := make([]string, 0, len(imp.Imagenes))
	for _, c := range imp.Imagenes {
		if c != clave {
			claves = append(claves, c)
		}
	}
	if len(claves) == len(imp.Imagenes) {
		return nil, ErrPaginaDesconocida
	}

	if err := uc.repo.GuardarImagenes(ctx, impID, claves); err != nil {
		return nil, err
	}
	return claves, nil
}

// QuitarConPlatos quita la hoja Y borra los platos que salieron de ella.
//
// Devuelve cuantos se borraron. Es irreversible y por eso solo se llega aqui
// desde el aviso, con el numero delante: si alguno tenia foto, la foto se va con
// el. La otra salida del aviso es Quitar, que no toca ningun plato.
func (uc *PaginasUC) QuitarConPlatos(ctx context.Context, impID id.ID, clave string) (int, error) {
	if _, err := uc.Quitar(ctx, impID, clave); err != nil {
		return 0, err
	}
	// Se borran DESPUES de sacar la hoja de la lista: al reves, un fallo al
	// guardar la lista dejaria los platos borrados y la hoja todavia puesta.
	return uc.repo.BorrarPlatosDeLaHoja(ctx, impID, clave)
}

// Reordenar cambia el orden de las paginas. Solo acepta una permutacion de las
// que ya hay: una lista con una clave de otra carta seria una forma de leer
// archivos ajenos.
func (uc *PaginasUC) Reordenar(ctx context.Context, impID id.ID, claves []string) ([]string, error) {
	imp, err := uc.repo.Obtener(ctx, impID)
	if err != nil {
		return nil, err
	}
	if len(claves) != len(imp.Imagenes) {
		return nil, ErrPaginaDesconocida
	}

	quedan := make(map[string]bool, len(imp.Imagenes))
	for _, c := range imp.Imagenes {
		quedan[c] = true
	}
	for _, c := range claves {
		if !quedan[c] {
			return nil, ErrPaginaDesconocida
		}
		delete(quedan, c)
	}
	return claves, uc.repo.GuardarImagenes(ctx, impID, claves)
}
