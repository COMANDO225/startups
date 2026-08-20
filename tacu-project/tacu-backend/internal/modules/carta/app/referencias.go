package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// maxReferencias por ambito. Con dos ya se entiende el estilo; a partir de ahi
// solo suben los tokens de entrada de cada llamada, que se pagan en todas las
// fotos de la carta.
const maxReferencias = 2

var ErrDemasiadasReferencias = errors.New("ya no caben mas fotos de ejemplo")

// RepoReferencias es solo lo del PLATO: las fotos del restaurante son del
// estilo, viven en sus dos ranuras y las gestiona EstiloUC.
type RepoReferencias interface {
	PlatoPorID(ctx context.Context, platoID id.ID) (domain.Plato, error)
	GuardarReferenciasDePlato(ctx context.Context, platoID id.ID, claves []string) error
}

// Referencias gestiona las fotos de ejemplo que guian la generacion.
type ReferenciasUC struct {
	repo    RepoReferencias
	almacen Almacen
}

func NuevasReferencias(repo RepoReferencias, almacen Almacen) *ReferenciasUC {
	return &ReferenciasUC{repo: repo, almacen: almacen}
}

// AgregarAPlato guarda una foto de ejemplo para un plato.
func (uc *ReferenciasUC) AgregarAPlato(ctx context.Context, platoID id.ID, bytes []byte, mime string) ([]string, error) {
	plato, err := uc.repo.PlatoPorID(ctx, platoID)
	if err != nil {
		return nil, err
	}
	if len(plato.FotoReferencias) >= maxReferencias {
		return nil, fmt.Errorf("%w: el maximo son %d por plato", ErrDemasiadasReferencias, maxReferencias)
	}

	clave := fmt.Sprintf("referencias/plato/%s/%s%s", platoID, id.Nuevo(), extensionDeImagen(mime))
	if err := uc.almacen.Guardar(ctx, clave, bytes); err != nil {
		return nil, fmt.Errorf("guardando la foto de ejemplo: %w", err)
	}

	claves := append(plato.FotoReferencias, clave)
	return claves, uc.repo.GuardarReferenciasDePlato(ctx, platoID, claves)
}

// QuitarDePlato borra una foto de ejemplo de la lista del plato.
//
// El archivo NO se borra del almacen, igual que con las fotos: es barato dejarlo
// y borrarlo abre la puerta a que un fallo a mitad deje la fila apuntando a un
// archivo que ya no esta.
func (uc *ReferenciasUC) QuitarDePlato(ctx context.Context, platoID id.ID, clave string) ([]string, error) {
	plato, err := uc.repo.PlatoPorID(ctx, platoID)
	if err != nil {
		return nil, err
	}
	claves := sinClave(plato.FotoReferencias, clave)
	return claves, uc.repo.GuardarReferenciasDePlato(ctx, platoID, claves)
}

func sinClave(claves []string, quitar string) []string {
	fuera := make([]string, 0, len(claves))
	for _, c := range claves {
		if c != quitar {
			fuera = append(fuera, c)
		}
	}
	return fuera
}
