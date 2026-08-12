package command

import (
	"context"
	"strings"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/crypto"
	"facturacion-service/pkg/ulid"
)

// CrearEmisor da de alta un emisor con su certificado y su Clave SOL. Hasta aqui
// solo existia cmd/seed, que corre desde una terminal con acceso a la base.
type CrearEmisor struct {
	alta domain.AltaEmisor
	tx   transaction.Transactor
}

func NewCrearEmisor(alta domain.AltaEmisor, tx transaction.Transactor) *CrearEmisor {
	return &CrearEmisor{alta: alta, tx: tx}
}

type CrearEmisorCmd struct {
	RUC             string
	RazonSocial     string
	NombreComercial string
	Direccion       string
	Ubigeo          string
	Departamento    string
	Provincia       string
	Distrito        string
	CertPEM         string
	SolUser         string
	SolPass         string
	Produccion      bool
	WebhookURL      string
	Series          []domain.SerieConfig
}

// EmisorCreado incluye los secretos generados. Es la unica vez que se ven: en
// la base solo queda el hash de la API key.
type EmisorCreado struct {
	TenantID      string
	APIKey        string
	WebhookSecret string
	Certificado   domain.DatosCertificado
	Series        []domain.SerieConfig
}

// seriesPorDefecto: un restaurante necesita al menos poder emitir facturas y
// boletas desde el primer dia.
var seriesPorDefecto = []domain.SerieConfig{
	{TipoDoc: domain.TipoFactura, Serie: "F001"},
	{TipoDoc: domain.TipoBoleta, Serie: "B001"},
}

func (uc *CrearEmisor) Execute(ctx context.Context, cmd CrearEmisorCmd) (*EmisorCreado, error) {
	datosCert, err := validarEmisor(&cmd)
	if err != nil {
		return nil, err
	}

	apiKey, err := crypto.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	secretoWebhook, err := crypto.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	nuevo := domain.NuevoTenant{
		Tenant: domain.Tenant{
			ID:              string(ulid.New()),
			RUC:             cmd.RUC,
			RazonSocial:     cmd.RazonSocial,
			NombreComercial: cmd.NombreComercial,
			Direccion:       cmd.Direccion,
			Ubigeo:          cmd.Ubigeo,
			Departamento:    cmd.Departamento,
			Provincia:       cmd.Provincia,
			Distrito:        cmd.Distrito,
			CertPEM:         cmd.CertPEM,
			SolUser:         cmd.SolUser,
			SolPass:         cmd.SolPass,
			Produccion:      cmd.Produccion,
			WebhookURL:      cmd.WebhookURL,
			WebhookSecret:   secretoWebhook,
		},
		APIKeyHash: crypto.SHA256Hash(apiKey),
		Series:     cmd.Series,
	}

	// El emisor y sus series se crean juntos: uno sin series no puede emitir
	// nada y quedaria a medio configurar.
	err = uc.tx.RunInTx(ctx, func(ctx context.Context) error {
		return uc.alta.CrearTenant(ctx, nuevo)
	})
	if err != nil {
		return nil, err
	}

	return &EmisorCreado{
		TenantID:      nuevo.Tenant.ID,
		APIKey:        apiKey,
		WebhookSecret: secretoWebhook,
		Certificado:   datosCert,
		Series:        cmd.Series,
	}, nil
}

func validarEmisor(cmd *CrearEmisorCmd) (domain.DatosCertificado, error) {
	cmd.RUC = strings.TrimSpace(cmd.RUC)

	// El RUC del emisor es la identidad ante SUNAT: si esta mal, no hay
	// comprobante que salga bien.
	if !domain.RUCValido(cmd.RUC) {
		return domain.DatosCertificado{}, domain.ErrReceptorInvalido(
			"el RUC del emisor " + cmd.RUC + " no es valido")
	}

	if n := len(strings.TrimSpace(cmd.RazonSocial)); n < 3 || n > 100 {
		return domain.DatosCertificado{}, domain.ErrReceptorInvalido(
			"razon_social debe tener entre 3 y 100 caracteres")
	}

	if strings.TrimSpace(cmd.Direccion) == "" {
		return domain.DatosCertificado{}, domain.ErrReceptorInvalido("direccion es obligatoria")
	}

	if strings.TrimSpace(cmd.SolUser) == "" || strings.TrimSpace(cmd.SolPass) == "" {
		return domain.DatosCertificado{}, domain.ErrReceptorInvalido(
			"sol_user y sol_pass son obligatorios: sin Clave SOL no se puede enviar a SUNAT")
	}

	datos, err := domain.ValidarCertificado(cmd.CertPEM)
	if err != nil {
		return domain.DatosCertificado{}, err
	}

	if len(cmd.Series) == 0 {
		cmd.Series = seriesPorDefecto
	}

	for _, s := range cmd.Series {
		if !s.TipoDoc.Valido() {
			return domain.DatosCertificado{}, domain.ErrTipoDocInvalido(string(s.TipoDoc))
		}
		if !s.TipoDoc.PrefijoSerieValido(s.Serie) {
			return domain.DatosCertificado{}, domain.ErrSerieIncoherente(s.Serie, string(s.TipoDoc))
		}
	}

	aplicarDefaults(cmd)
	return datos, nil
}

// SUNAT exige ubigeo y division politica. Lima es lo razonable por defecto para
// el mercado al que apunta esto.
func aplicarDefaults(cmd *CrearEmisorCmd) {
	if cmd.Ubigeo == "" {
		cmd.Ubigeo = "150101"
	}
	if cmd.Departamento == "" {
		cmd.Departamento = "LIMA"
	}
	if cmd.Provincia == "" {
		cmd.Provincia = "LIMA"
	}
	if cmd.Distrito == "" {
		cmd.Distrito = "LIMA"
	}
	if cmd.NombreComercial == "" {
		cmd.NombreComercial = cmd.RazonSocial
	}
}
