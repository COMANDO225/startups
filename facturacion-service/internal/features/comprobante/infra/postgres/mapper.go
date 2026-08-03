package postgres

import (
	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/features/comprobante/infra/postgres/comprobantedb"
	"facturacion-service/pkg/pgconv"
)

// Todas las queries de comprobantes devuelven el mismo shape (SELECT *), por eso
// alcanza un unico mapper en vez de uno por query.
func aDominio(row comprobantedb.Comprobante) *domain.Comprobante {
	return domain.Reconstruir(domain.EstadoPersistido{
		ID:             row.ID,
		TenantID:       row.TenantID,
		IdempotencyKey: row.IdempotencyKey,
		TipoDoc:        domain.TipoDoc(row.TipoDoc),
		Serie:          row.Serie,
		Correlativo:    row.Correlativo,
		Estado:         domain.Estado(row.Estado),
		Payload:        row.Payload,
		Moneda:         row.Moneda,
		ImporteTotal:   pgconv.NumericToString(row.ImporteTotal),
		FechaEmision:   row.FechaEmision.Time,
		XML:            row.Xml,
		CDR:            row.Cdr,
		Ticket:         deref(row.Ticket),
		CodigoSunat:    deref(row.CodigoSunat),
		MensajeSunat:   deref(row.MensajeSunat),
		Intentos:       int(row.Intentos),
		TomadoAt:       pgconv.TimestamptzToNullableTime(row.TomadoAt),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
		MotivoBaja:     deref(row.MotivoBaja),
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
