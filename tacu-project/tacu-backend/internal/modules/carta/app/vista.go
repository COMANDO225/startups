package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// ErrSinPresupuesto se devuelve cuando la carta ya gasto lo suyo. Es un 402 en
// el borde y no un 500: no hay nada roto, no queda dinero.
var ErrSinPresupuesto = errors.New("esta carta ya gasto su presupuesto")

// RepoVista es lo que hace falta para la vista previa del estilo.
type RepoVista interface {
	Base(ctx context.Context, importacionID id.ID, categoria string) ([]domain.Tipo, domain.Receta, error)

	// La clave de la vista NO viaja dentro de la Receta: la receta son las
	// ranuras del prompt, y esta es una foto ya hecha.
	VistaDeBase(ctx context.Context, importacionID id.ID, categoria string) (string, error)
	GuardarVistaDeBase(ctx context.Context, importacionID id.ID, categoria, clave string) error

	ReservarPresupuesto(ctx context.Context, importacionID id.ID, costo dinero.MicrosUSD) (bool, error)
}

// Pintor dibuja una imagen a partir de un prompt pelado.
//
// Lo declara aqui quien lo consume: al generador de fotos de platos se le pasa
// un EncargoDeFoto entero, y una vista previa no tiene plato, ni tipos, ni
// presupuesto por plato. Lo unico que comparten es que llaman al mismo modelo.
type Pintor interface {
	Pintar(ctx context.Context, prompt string) (bytes []byte, mime string, err error)
}

// ConImportacion carga el gasto de la llamada a la carta que la pidio. Sin esto
// la vista previa se generaria sin aparecer en el libro de nadie.
type ConImportacion func(ctx context.Context, importacionID id.ID) context.Context

// VistaUC dibuja el recipiente vacio que el dueno acaba de describir.
//
// Se genera A PETICION y no al guardar el estilo: guardar es gratis y se hace a
// cada tecla que el dueno corrige, y cobrarle $0.0336 por cada correccion seria
// cobrarle por escribir.
type VistaUC struct {
	repo     RepoVista
	almacen  Almacen
	pintor   Pintor
	atribuir ConImportacion
	costo    dinero.MicrosUSD
}

func NuevaVista(repo RepoVista, almacen Almacen, pintor Pintor,
	atribuir ConImportacion, costo dinero.MicrosUSD) *VistaUC {
	return &VistaUC{repo: repo, almacen: almacen, pintor: pintor,
		atribuir: atribuir, costo: costo}
}

// Clave devuelve la vista que ya existe, o vacio si nunca se genero.
func (uc *VistaUC) Clave(ctx context.Context, impID id.ID, categoria string) (string, error) {
	return uc.repo.VistaDeBase(ctx, impID, categoria)
}

// Generar dibuja la vista previa y devuelve su clave.
//
// El orden es el mismo del worker de fotos y por lo mismo: se reserva ANTES de
// llamar, porque comprobar despues deja que dos pestanas abiertas se pasen las
// dos del presupuesto.
func (uc *VistaUC) Generar(ctx context.Context, impID id.ID, categoria string) (string, error) {
	_, base, err := uc.repo.Base(ctx, impID, categoria)
	if err != nil {
		return "", err
	}

	hay, err := uc.repo.ReservarPresupuesto(ctx, impID, uc.costo)
	if err != nil {
		return "", fmt.Errorf("reservando presupuesto: %w", err)
	}
	if !hay {
		return "", ErrSinPresupuesto
	}

	bytes, mime, err := uc.pintor.Pintar(uc.atribuir(ctx, impID), PromptDeVajilla(base))
	if err != nil {
		return "", fmt.Errorf("dibujando la vista del estilo: %w", err)
	}

	// Clave nueva en cada generacion, nunca la misma sobrescrita: el navegador
	// tiene cacheada la anterior y el dueno vería la de antes creyendo que su
	// cambio no hizo nada.
	clave := fmt.Sprintf("estilo/%s/%s%s", impID, id.Nuevo(), extensionDeImagen(mime))
	if err := uc.almacen.Guardar(ctx, clave, bytes); err != nil {
		return "", fmt.Errorf("guardando la vista del estilo: %w", err)
	}
	return clave, uc.repo.GuardarVistaDeBase(ctx, impID, categoria, clave)
}
