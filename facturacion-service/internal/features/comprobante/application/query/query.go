package query

import (
	"context"

	"facturacion-service/internal/features/comprobante/domain"
)

type Obtener struct {
	repo domain.Repositorio
}

func NewObtener(repo domain.Repositorio) *Obtener {
	return &Obtener{repo: repo}
}

func (uc *Obtener) Execute(ctx context.Context, tenantID, id string) (*domain.Comprobante, error) {
	return uc.repo.PorID(ctx, tenantID, id)
}

type Listar struct {
	repo domain.Repositorio
}

func NewListar(repo domain.Repositorio) *Listar {
	return &Listar{repo: repo}
}

func (uc *Listar) Execute(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Comprobante, int, error) {
	return uc.repo.Listar(ctx, tenantID, limit, offset)
}

// RequierenAtencion lista lo que ningun reintento arregla. Sin este listado, un
// comprobante roto queda invisible: River deja de reintentarlo y nadie se entera.
type RequierenAtencion struct {
	repo domain.Repositorio
}

func NewRequierenAtencion(repo domain.Repositorio) *RequierenAtencion {
	return &RequierenAtencion{repo: repo}
}

func (uc *RequierenAtencion) Execute(ctx context.Context, tenantID string, limite int) ([]*domain.Comprobante, error) {
	return uc.repo.RequierenAtencion(ctx, tenantID, limite)
}
