package app

import (
	"context"
	"fmt"

	"tacu-backend/internal/kernel/id"
)

// RepoPortada es lo que hace falta para colgarle al restaurante la foto de su
// local.
type RepoPortada interface {
	RestauranteDeImportacion(ctx context.Context, importacionID id.ID) (id.ID, error)

	// GuardarPortada devuelve la clave de la portada ANTERIOR, para poder
	// borrarla del almacen.
	GuardarPortada(ctx context.Context, restauranteID id.ID, clave string) (string, error)
}

// Portada es la foto del local, la que encabeza el catalogo publico.
//
// Cuelga del RESTAURANTE y no de la importacion: es identidad del negocio, no
// contenido de una carta. Releer, anadir hojas o volver a publicar no se la
// lleva por delante.
type Portada struct {
	repo RepoPortada
	alm  AlmacenDeImagenes
}

func NuevaPortada(repo RepoPortada, alm AlmacenDeImagenes) *Portada {
	return &Portada{repo: repo, alm: alm}
}

// Poner guarda la foto y devuelve su clave.
//
// EL ORDEN: primero los archivos, despues la fila, y la anterior se borra al
// final. Un objeto huerfano no lo ve nadie; una fila que apunta a un archivo que
// no esta es una imagen rota en el catalogo, que es lo que ya nos paso una vez.
func (uc *Portada) Poner(ctx context.Context, impID id.ID, bytes []byte) (string, error) {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return "", err
	}

	clave, err := GuardarPortada(ctx, uc.alm, restaurante, bytes)
	if err != nil {
		return "", err
	}

	anterior, err := uc.repo.GuardarPortada(ctx, restaurante, clave)
	if err != nil {
		return "", err
	}

	// El borrado de la vieja se IGNORA a proposito, y por eso va con `_ =` y no
	// con un if que devuelve nil: esa forma se lee como un error tragado por
	// descuido. Aqui la portada nueva ya esta puesta y es lo que el dueno pidio;
	// lo que queda si esto falla es un objeto de mas en el bucket, no una carta
	// rota. Devolver error aqui le diria que no se guardo, y si se guardo.
	if anterior != "" && anterior != clave {
		_ = BorrarFoto(ctx, uc.alm, anterior)
	}
	return clave, nil
}

// Quitar deja el catalogo con el nombre en texto, que es como salia antes.
func (uc *Portada) Quitar(ctx context.Context, impID id.ID) error {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return err
	}
	anterior, err := uc.repo.GuardarPortada(ctx, restaurante, "")
	if err != nil {
		return err
	}
	if err := BorrarFoto(ctx, uc.alm, anterior); err != nil {
		return fmt.Errorf("borrando la portada: %w", err)
	}
	return nil
}
