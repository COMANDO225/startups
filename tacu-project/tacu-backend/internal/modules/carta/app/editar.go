package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// maxEtiqueta topea el nombre de una variante de precio. "Personal", "Familiar",
// "1/2 docena" caben de sobra; una frase entera no es una etiqueta y rompe la
// tarjeta del catalogo.
const maxEtiqueta = 40

var ErrEtiquetaLarga = errors.New("el nombre del precio es demasiado largo")

// RepoEditar es lo que el caso de uso necesita para corregir un plato.
type RepoEditar interface {
	// EditarPlato aplica las etiquetas, vuelve a verificar el plato y recuenta
	// las marcas de la carta, todo en una transaccion.
	EditarPlato(ctx context.Context, platoID id.ID, etiquetas []string) (domain.Plato, domain.Marcas, error)

	BorrarPlato(ctx context.Context, platoID id.ID) error
	RecuperarPlato(ctx context.Context, platoID id.ID) error
}

// Editar corrige lo que el dueno arregla a mano en la pantalla de revision.
type Editar struct{ repo RepoEditar }

func NuevoEditar(repo RepoEditar) *Editar { return &Editar{repo: repo} }

// Etiquetas le pone nombre a los precios de un plato con varias opciones.
//
// Es lo que apaga el aviso rojo de "tiene varios precios y la carta no dice de
// que es cada uno", que es el unico motivo que el dueno puede resolver
// escribiendo. Los demas —un precio ilegible, uno discordante— se resuelven
// mirando la carta, no tecleando.
//
// La verificacion NO se hace aqui sino en el dominio, dentro de la misma
// transaccion que guarda: si el plato se limpiara y el recuento no, la barra de
// publicar seguiria bloqueada sobre platos que ya estan bien.
func (uc *Editar) Etiquetas(ctx context.Context, platoID id.ID, etiquetas []string) (domain.Plato, domain.Marcas, error) {
	for _, e := range etiquetas {
		if len([]rune(strings.TrimSpace(e))) > maxEtiqueta {
			return domain.Plato{}, domain.Marcas{},
				fmt.Errorf("%w: el maximo son %d caracteres", ErrEtiquetaLarga, maxEtiqueta)
		}
	}
	return uc.repo.EditarPlato(ctx, platoID, etiquetas)
}

// Quitar borra un plato de la carta.
//
// Solo se llega aqui por decision del dueno. Ninguna lectura borra un plato:
// puede tener una foto pagada detras, y una lectura que no lo trajo se pudo
// equivocar —una hoja movida, un reflejo— asi que lo que hace es marcarlo
// ausente y esperar a que alguien mire.
func (uc *Editar) Quitar(ctx context.Context, platoID id.ID) error {
	return uc.repo.BorrarPlato(ctx, platoID)
}

// Recuperar deshace el ausente: el plato sigue en la carta aunque la ultima
// lectura no lo trajera.
func (uc *Editar) Recuperar(ctx context.Context, platoID id.ID) error {
	return uc.repo.RecuperarPlato(ctx, platoID)
}
