package postgres

import (
	"context"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/features/comprobante/infra/postgres/comprobantedb"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/crypto"
	"facturacion-service/pkg/pgxerr"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantRepo struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

func NewTenantRepo(pool *pgxpool.Pool, cipher *crypto.Cipher) *TenantRepo {
	return &TenantRepo{pool: pool, cipher: cipher}
}

func (r *TenantRepo) PorID(ctx context.Context, id string) (*domain.Tenant, error) {
	row, err := comprobantedb.New(transaction.Querier(ctx, r.pool)).TenantPorID(ctx, id)
	if err != nil {
		return nil, pgxerr.MapError(err, domain.ErrTenantNoEncontrado())
	}
	return r.aDominio(row)
}

func (r *TenantRepo) PorAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	hash := crypto.SHA256Hash(apiKey)

	row, err := comprobantedb.New(transaction.Querier(ctx, r.pool)).TenantPorAPIKeyHash(ctx, hash)
	if err != nil {
		return nil, pgxerr.MapError(err, domain.ErrTenantNoEncontrado())
	}
	return r.aDominio(row)
}

// El certificado y la clave SOL se descifran aqui y solo viven en memoria
// mientras dura el request al motor.
func (r *TenantRepo) aDominio(row comprobantedb.Tenant) (*domain.Tenant, error) {
	cert, err := r.cipher.Decrypt(row.CertCifrado)
	if err != nil {
		return nil, err
	}

	solPass, err := r.cipher.Decrypt(row.SolPassCifrado)
	if err != nil {
		return nil, err
	}

	var webhookSecret []byte
	if len(row.WebhookSecretCifrado) > 0 {
		webhookSecret, err = r.cipher.Decrypt(row.WebhookSecretCifrado)
		if err != nil {
			return nil, err
		}
	}

	return &domain.Tenant{
		ID:              row.ID,
		RUC:             row.Ruc,
		RazonSocial:     row.RazonSocial,
		NombreComercial: deref(row.NombreComercial),
		Direccion:       row.Direccion,
		Ubigeo:          row.Ubigeo,
		Departamento:    row.Departamento,
		Provincia:       row.Provincia,
		Distrito:        row.Distrito,
		CertPEM:         string(cert),
		SolUser:         row.SolUser,
		SolPass:         string(solPass),
		Produccion:      row.Produccion,
		WebhookURL:      deref(row.WebhookUrl),
		WebhookSecret:   string(webhookSecret),
	}, nil
}

// CrearTenant cifra el certificado, la Clave SOL y el secreto del webhook antes
// de escribirlos: en la base nunca hay un secreto en claro.
func (r *TenantRepo) CrearTenant(ctx context.Context, nuevo domain.NuevoTenant) error {
	q := comprobantedb.New(transaction.Querier(ctx, r.pool))
	t := nuevo.Tenant

	certCifrado, err := r.cipher.Encrypt([]byte(t.CertPEM))
	if err != nil {
		return err
	}

	passCifrado, err := r.cipher.Encrypt([]byte(t.SolPass))
	if err != nil {
		return err
	}

	secretoCifrado, err := r.cipher.Encrypt([]byte(t.WebhookSecret))
	if err != nil {
		return err
	}

	err = q.CrearTenant(ctx, comprobantedb.CrearTenantParams{
		ID:                   t.ID,
		Ruc:                  t.RUC,
		RazonSocial:          t.RazonSocial,
		NombreComercial:      ptrSiNoVacio(t.NombreComercial),
		Direccion:            t.Direccion,
		Ubigeo:               t.Ubigeo,
		Departamento:         t.Departamento,
		Provincia:            t.Provincia,
		Distrito:             t.Distrito,
		CertCifrado:          certCifrado,
		SolUser:              t.SolUser,
		SolPassCifrado:       passCifrado,
		Produccion:           t.Produccion,
		ApiKeyHash:           nuevo.APIKeyHash,
		WebhookUrl:           ptrSiNoVacio(t.WebhookURL),
		WebhookSecretCifrado: secretoCifrado,
	})
	if err != nil {
		if pgxerr.IsUniqueViolation(err) {
			return domain.ErrEmisorDuplicado(t.RUC)
		}
		return err
	}

	for _, s := range nuevo.Series {
		if err := q.CrearSerie(ctx, comprobantedb.CrearSerieParams{
			TenantID:    t.ID,
			TipoDoc:     string(s.TipoDoc),
			Serie:       s.Serie,
			Correlativo: 0,
		}); err != nil {
			return err
		}
	}

	return nil
}

func ptrSiNoVacio(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
