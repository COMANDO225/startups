package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// maxEtiqueta topea el nombre de una variante de precio. "Personal", "Familiar",
// "1/2 docena" caben de sobra; una frase entera no es una etiqueta y rompe la
// tarjeta del catalogo.
const maxEtiqueta = 40

var (
	ErrEtiquetaLarga  = errors.New("el nombre del precio es demasiado largo")
	ErrPrecioInvalido = errors.New("ese precio no se entiende")
)

// RepoEditar es lo que el caso de uso necesita para corregir un plato.
type RepoEditar interface {
	// EditarPlato aplica las etiquetas, vuelve a verificar el plato y recuenta
	// las marcas de la carta, todo en una transaccion.
	EditarPlato(ctx context.Context, platoID id.ID,
		correcciones []domain.CorreccionDePrecio) (domain.Plato, domain.Marcas, error)

	BorrarPlato(ctx context.Context, platoID id.ID) error
	RecuperarPlato(ctx context.Context, platoID id.ID) error
}

// Editar corrige lo que el dueno arregla a mano en la pantalla de revision.
type Editar struct{ repo RepoEditar }

func NuevoEditar(repo RepoEditar) *Editar { return &Editar{repo: repo} }

// Precios corrige a mano las opciones de precio de un plato: el nombre de cada
// una y, si hace falta, el importe.
//
// El importe se puede tocar porque el modelo se equivoca: acierta ~67% en
// valores y ademas CORRIGE en silencio lo que percibe como un error de formato,
// asi que el cruce de testigos marca el plato pero no puede arreglarlo. Quien
// puede es el dueno, que tiene la carta delante. Sin esto, un precio mal leido
// bloqueaba publicar sin salida ninguna.
//
// El importe llega como TEXTO y lo parsea el mismo dinero.Parsear que cruza los
// precios de la carta: asi "45", "45.50" y "S/ 45" valen igual aqui y alli, y no
// hay dos ideas distintas de que es un precio.
//
// La verificacion NO se hace aqui sino en el dominio, dentro de la misma
// transaccion que guarda: si el plato se limpiara y el recuento no, la barra de
// publicar seguiria bloqueada sobre platos que ya estan bien.
func (uc *Editar) Precios(ctx context.Context, platoID id.ID,
	etiquetas, importes []string) (domain.Plato, domain.Marcas, error) {
	correcciones := make([]domain.CorreccionDePrecio, 0, len(etiquetas))

	for i, e := range etiquetas {
		if len([]rune(strings.TrimSpace(e))) > maxEtiqueta {
			return domain.Plato{}, domain.Marcas{},
				fmt.Errorf("%w: el maximo son %d caracteres", ErrEtiquetaLarga, maxEtiqueta)
		}

		c := domain.CorreccionDePrecio{Etiqueta: e}

		// Vacio es "no lo toco", no "vale cero": un importe en blanco deja el
		// que ya estaba, que puede venir de la carta y estar bien.
		if i < len(importes) && strings.TrimSpace(importes[i]) != "" {
			cent, err := dinero.Parsear(importes[i])
			if err != nil {
				return domain.Plato{}, domain.Marcas{},
					fmt.Errorf("%w: %q", ErrPrecioInvalido, importes[i])
			}
			if cent < 0 {
				return domain.Plato{}, domain.Marcas{},
					fmt.Errorf("%w: %q", ErrPrecioInvalido, importes[i])
			}
			c.Centimos = &cent
		}
		correcciones = append(correcciones, c)
	}

	return uc.repo.EditarPlato(ctx, platoID, correcciones)
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
