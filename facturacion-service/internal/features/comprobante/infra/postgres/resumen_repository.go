package postgres

import (
	"context"
	"errors"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/features/comprobante/infra/postgres/comprobantedb"
	"facturacion-service/pkg/pgconv"
	"facturacion-service/pkg/pgxerr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repo) SiguienteCorrelativoResumen(ctx context.Context, tenantID string, tipo domain.TipoResumen, fechaRef time.Time) (int64, error) {
	return r.q(ctx).SiguienteCorrelativoResumen(ctx, comprobantedb.SiguienteCorrelativoResumenParams{
		TenantID: tenantID,
		Tipo:     string(tipo),
		FechaRef: fecha(fechaRef),
	})
}

func (r *Repo) CrearResumen(ctx context.Context, res *domain.Resumen) error {
	err := r.q(ctx).CrearResumen(ctx, comprobantedb.CrearResumenParams{
		ID:          res.ID(),
		TenantID:    res.TenantID(),
		Tipo:        string(res.Tipo()),
		FechaRef:    fecha(res.FechaRef()),
		Correlativo: res.Correlativo(),
		Estado:      string(res.Estado()),
	})
	return pgxerr.MapWriteError(err)
}

func (r *Repo) ResumenPorID(ctx context.Context, id string) (*domain.Resumen, error) {
	row, err := r.q(ctx).ResumenPorID(ctx, id)
	if err != nil {
		return nil, pgxerr.MapError(err, domain.ErrResumenNoEncontrado())
	}
	return resumenADominio(row), nil
}

func (r *Repo) TomarResumen(ctx context.Context, id string) (*domain.Resumen, error) {
	row, err := r.q(ctx).TomarResumen(ctx, comprobantedb.TomarResumenParams{
		ID:              id,
		TimeoutSegundos: int32(domain.TimeoutProcesando.Seconds()),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrYaTomado()
	}
	if err != nil {
		return nil, err
	}
	return resumenADominio(row), nil
}

func (r *Repo) ActualizarResumen(ctx context.Context, res *domain.Resumen) error {
	return r.q(ctx).ActualizarResumen(ctx, comprobantedb.ActualizarResumenParams{
		ID:           res.ID(),
		Estado:       string(res.Estado()),
		Ticket:       ptr(res.Ticket()),
		Xml:          res.XML(),
		Cdr:          res.CDR(),
		CodigoSunat:  ptr(res.CodigoSunat()),
		MensajeSunat: ptr(res.MensajeSunat()),
	})
}

func (r *Repo) ResumenesEnCurso(ctx context.Context, antiguedad time.Duration, limite int) ([]*domain.Resumen, error) {
	rows, err := r.q(ctx).ResumenesEnCurso(ctx, comprobantedb.ResumenesEnCursoParams{
		AntiguedadSegundos: int32(antiguedad.Seconds()),
		MaxIntentos:        int32(domain.MaxIntentos),
		Limite:             int32(limite),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*domain.Resumen, len(rows))
	for i, row := range rows {
		out[i] = resumenADominio(row)
	}
	return out, nil
}

func (r *Repo) GruposPendientes(ctx context.Context, limite int) ([]domain.GrupoBoletas, error) {
	rows, err := r.q(ctx).TenantsConBoletasPendientes(ctx, int32(limite))
	if err != nil {
		return nil, err
	}

	out := make([]domain.GrupoBoletas, len(rows))
	for i, row := range rows {
		out[i] = domain.GrupoBoletas{TenantID: row.TenantID, Fecha: row.FechaEmision.Time}
	}
	return out, nil
}

func (r *Repo) BoletasParaResumen(ctx context.Context, tenantID string, f time.Time, limite int) ([]*domain.Comprobante, error) {
	rows, err := r.q(ctx).BoletasParaResumen(ctx, comprobantedb.BoletasParaResumenParams{
		TenantID:     tenantID,
		FechaEmision: fecha(f),
		Limite:       int32(limite),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*domain.Comprobante, len(rows))
	for i, row := range rows {
		out[i] = aDominio(row)
	}
	return out, nil
}

func (r *Repo) AsignarResumen(ctx context.Context, resumenID string, ids []string) error {
	return r.q(ctx).AsignarResumen(ctx, comprobantedb.AsignarResumenParams{
		ResumenID: resumenID,
		Ids:       ids,
	})
}

func (r *Repo) ResolverComprobantesDeResumen(ctx context.Context, resumenID string, estado domain.Estado, codigo, mensaje string) error {
	return r.q(ctx).ResolverComprobantesDeResumen(ctx, comprobantedb.ResolverComprobantesDeResumenParams{
		ResumenID:    resumenID,
		Estado:       string(estado),
		CodigoSunat:  codigo,
		MensajeSunat: mensaje,
	})
}

func (r *Repo) ComprobantesDeResumen(ctx context.Context, resumenID string) ([]*domain.Comprobante, error) {
	rows, err := r.q(ctx).ComprobantesDeResumen(ctx, resumenID)
	if err != nil {
		return nil, err
	}

	out := make([]*domain.Comprobante, len(rows))
	for i, row := range rows {
		out[i] = aDominio(row)
	}
	return out, nil
}

func resumenADominio(row comprobantedb.Resumene) *domain.Resumen {
	return domain.ReconstruirResumen(domain.ResumenPersistido{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Tipo:         domain.TipoResumen(row.Tipo),
		FechaRef:     row.FechaRef.Time,
		Correlativo:  row.Correlativo,
		Estado:       domain.Estado(row.Estado),
		Ticket:       deref(row.Ticket),
		XML:          row.Xml,
		CDR:          row.Cdr,
		CodigoSunat:  deref(row.CodigoSunat),
		MensajeSunat: deref(row.MensajeSunat),
		Intentos:     int(row.Intentos),
		TomadoAt:     pgconv.TimestamptzToNullableTime(row.TomadoAt),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	})
}

func fecha(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
