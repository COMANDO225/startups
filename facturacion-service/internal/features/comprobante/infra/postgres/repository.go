package postgres

import (
	"context"
	"errors"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/features/comprobante/infra/postgres/comprobantedb"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/pgconv"
	"facturacion-service/pkg/pgxerr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// q respeta la transaccion activa en el contexto, si la hay.
func (r *Repo) q(ctx context.Context) *comprobantedb.Queries {
	return comprobantedb.New(transaction.Querier(ctx, r.pool))
}

func (r *Repo) SiguienteCorrelativo(ctx context.Context, tenantID string, tipoDoc domain.TipoDoc, serie string) (int64, error) {
	n, err := r.q(ctx).SiguienteCorrelativo(ctx, comprobantedb.SiguienteCorrelativoParams{
		TenantID: tenantID,
		TipoDoc:  string(tipoDoc),
		Serie:    serie,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrSerieNoConfigurada(serie)
	}
	return n, err
}

func (r *Repo) Crear(ctx context.Context, c *domain.Comprobante) error {
	importe, err := pgconv.StringToNumeric(c.ImporteTotal())
	if err != nil {
		return err
	}

	err = r.q(ctx).CrearComprobante(ctx, comprobantedb.CrearComprobanteParams{
		ID:             c.ID(),
		TenantID:       c.TenantID(),
		IdempotencyKey: c.IdempotencyKey(),
		TipoDoc:        string(c.TipoDoc()),
		Serie:          c.Serie(),
		Correlativo:    c.Correlativo(),
		Estado:         string(c.Estado()),
		Payload:        c.Payload(),
		Moneda:         c.Moneda(),
		ImporteTotal:   importe,
		FechaEmision:   pgtype.Date{Time: c.FechaEmision(), Valid: true},
		CreatedAt:      pgconv.TimeToTimestamptz(c.CreatedAt()),
		UpdatedAt:      pgconv.TimeToTimestamptz(c.UpdatedAt()),
	})
	return pgxerr.MapWriteError(err)
}

func (r *Repo) PorID(ctx context.Context, tenantID, id string) (*domain.Comprobante, error) {
	row, err := r.q(ctx).ComprobantePorID(ctx, comprobantedb.ComprobantePorIDParams{
		TenantID: tenantID,
		ID:       id,
	})
	if err != nil {
		return nil, pgxerr.MapError(err, domain.ErrNoEncontrado())
	}
	return aDominio(row), nil
}

func (r *Repo) PorIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Comprobante, error) {
	row, err := r.q(ctx).ComprobantePorIdempotencyKey(ctx, comprobantedb.ComprobantePorIdempotencyKeyParams{
		TenantID:       tenantID,
		IdempotencyKey: key,
	})
	if err != nil {
		return nil, pgxerr.MapError(err, domain.ErrNoEncontrado())
	}
	return aDominio(row), nil
}

func (r *Repo) Listar(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Comprobante, int, error) {
	q := r.q(ctx)

	rows, err := q.ListarComprobantes(ctx, comprobantedb.ListarComprobantesParams{
		TenantID: tenantID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := q.ContarComprobantes(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*domain.Comprobante, len(rows))
	for i, row := range rows {
		out[i] = aDominio(row)
	}
	return out, int(total), nil
}

func (r *Repo) Tomar(ctx context.Context, id string) (*domain.Comprobante, error) {
	row, err := r.q(ctx).TomarComprobante(ctx, comprobantedb.TomarComprobanteParams{
		ID:              id,
		EstadosTomables: domain.EstadosTomables(),
		TimeoutSegundos: int32(domain.TimeoutProcesando.Seconds()),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrYaTomado()
	}
	if err != nil {
		return nil, err
	}
	return aDominio(row), nil
}

func (r *Repo) EstadoActual(ctx context.Context, id string) (domain.Estado, error) {
	estado, err := r.q(ctx).EstadoComprobante(ctx, id)
	if err != nil {
		return "", pgxerr.MapError(err, domain.ErrNoEncontrado())
	}
	return domain.Estado(estado), nil
}

func (r *Repo) Huerfanos(ctx context.Context, antiguedad time.Duration, limite int) ([]*domain.Comprobante, error) {
	rows, err := r.q(ctx).ComprobantesHuerfanos(ctx, comprobantedb.ComprobantesHuerfanosParams{
		AntiguedadSegundos: int32(antiguedad.Seconds()),
		MaxIntentos:        int32(domain.MaxIntentos),
		Limit:              int32(limite),
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

func (r *Repo) MarcarWebhookEnviado(ctx context.Context, id string) error {
	return r.q(ctx).MarcarWebhookEnviado(ctx, id)
}

func (r *Repo) MarcarAnulado(ctx context.Context, id, motivo string) error {
	return r.q(ctx).MarcarComprobanteAnulado(ctx, comprobantedb.MarcarComprobanteAnuladoParams{
		ID:     id,
		Motivo: motivo,
	})
}

func (r *Repo) SinNotificar(ctx context.Context, antiguedad time.Duration, limite int) ([]*domain.Comprobante, error) {
	rows, err := r.q(ctx).ComprobantesSinNotificar(ctx, comprobantedb.ComprobantesSinNotificarParams{
		AntiguedadSegundos: int32(antiguedad.Seconds()),
		Limite:             int32(limite),
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

func (r *Repo) RequierenAtencion(ctx context.Context, tenantID string, limite int) ([]*domain.Comprobante, error) {
	rows, err := r.q(ctx).ComprobantesRequierenAtencion(ctx, comprobantedb.ComprobantesRequierenAtencionParams{
		TenantID:    tenantID,
		MaxIntentos: int32(domain.MaxIntentos),
		Limit:       int32(limite),
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

func (r *Repo) Actualizar(ctx context.Context, c *domain.Comprobante) error {
	return r.q(ctx).ActualizarComprobante(ctx, comprobantedb.ActualizarComprobanteParams{
		ID:           c.ID(),
		Estado:       string(c.Estado()),
		Xml:          c.XML(),
		Cdr:          c.CDR(),
		Ticket:       ptr(c.Ticket()),
		CodigoSunat:  ptr(c.CodigoSunat()),
		MensajeSunat: ptr(c.MensajeSunat()),
	})
}
