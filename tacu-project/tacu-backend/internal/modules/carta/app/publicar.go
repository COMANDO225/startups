package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

var ErrNoSePuedePublicar = errors.New("la carta todavia no se puede publicar")

type RepoPublicar interface {
	Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error)
	Publicar(ctx context.Context, importacionID id.ID, slug string) (string, error)
	CartaPublica(ctx context.Context, slug string) (domain.Importacion, error)
}

type Publicar struct{ repo RepoPublicar }

func NuevoPublicar(repo RepoPublicar) *Publicar { return &Publicar{repo: repo} }

// Ejecutar deja la carta visible en /r/{slug} y devuelve el slug.
//
// La regla de si se puede vive en el DOMINIO (Importacion.PuedePublicarse), no
// aqui: es la misma que pinta la pantalla, y tenerla en dos sitios seria que un
// dia el boton se habilite y el servidor diga que no.
func (uc *Publicar) Ejecutar(ctx context.Context, importacionID id.ID) (string, error) {
	imp, err := uc.repo.Obtener(ctx, importacionID)
	if err != nil {
		return "", err
	}
	if !imp.PuedePublicarse() {
		return "", fmt.Errorf("%w: quedan %d platos por corregir",
			ErrNoSePuedePublicar, imp.Marcas.Revisar)
	}

	// Un nombre sin ninguna letra utilizable no da URL. En vez de inventar una,
	// se cae al id: feo pero unico, y el dueno puede cambiar el nombre y volver
	// a publicar.
	slug := domain.Slug(imp.Restaurante.Nombre)
	if slug == "" {
		slug = "carta-" + importacionID.String()[:8]
	}
	return uc.repo.Publicar(ctx, importacionID, slug)
}

// Carta devuelve la carta publica de un slug. No pide token: es la que ve el
// cliente del restaurante.
func (uc *Publicar) Carta(ctx context.Context, slug string) (domain.Importacion, error) {
	return uc.repo.CartaPublica(ctx, slug)
}
