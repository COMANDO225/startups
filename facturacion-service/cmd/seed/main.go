package main

import (
	"context"
	"fmt"
	"os"

	"facturacion-service/internal/features/comprobante/infra/postgres/comprobantedb"
	"facturacion-service/internal/shared/config"
	"facturacion-service/pkg/crypto"
	"facturacion-service/pkg/ulid"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Crea un emisor de pruebas contra el entorno BETA de SUNAT, cuyas credenciales
// son publicas y no requieren certificado registrado.
func main() {
	certPath := os.Args[1]
	apiKey := "test-api-key"

	cfg, err := config.Load()
	must(err)

	certPEM, err := os.ReadFile(certPath)
	must(err)

	cipher, err := crypto.NewCipher(cfg.Crypto.MasterKey)
	must(err)

	certCifrado, err := cipher.Encrypt(certPEM)
	must(err)

	passCifrado, err := cipher.Encrypt([]byte("moddatos"))
	must(err)

	webhookURL := ""
	if len(os.Args) > 2 {
		webhookURL = os.Args[2]
	}
	secretoCifrado, err := cipher.Encrypt([]byte("secreto-de-prueba"))
	must(err)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	must(err)
	defer pool.Close()

	q := comprobantedb.New(pool)
	tenantID := string(ulid.New())

	must(q.CrearTenant(ctx, comprobantedb.CrearTenantParams{
		ID:                   tenantID,
		Ruc:                  "20000000001",
		RazonSocial:          "EMPRESA DE PRUEBA SAC",
		NombreComercial:      ptr("TACU TEST"),
		Direccion:            "AV. PRUEBA 123",
		Ubigeo:               "150101",
		Departamento:         "LIMA",
		Provincia:            "LIMA",
		Distrito:             "LIMA",
		CertCifrado:          certCifrado,
		SolUser:              "MODDATOS",
		SolPassCifrado:       passCifrado,
		Produccion:           false,
		ApiKeyHash:           crypto.SHA256Hash(apiKey),
		WebhookUrl:           ptrSiNoVacio(webhookURL),
		WebhookSecretCifrado: secretoCifrado,
	}))

	for _, s := range []struct{ tipo, serie string }{{"01", "F001"}, {"03", "B001"}} {
		must(q.CrearSerie(ctx, comprobantedb.CrearSerieParams{
			TenantID:    tenantID,
			TipoDoc:     s.tipo,
			Serie:       s.serie,
			Correlativo: 0,
		}))
	}

	fmt.Printf("tenant_id : %s\napi_key   : %s\nseries    : F001 (factura), B001 (boleta)\nwebhook   : %s\n",
		tenantID, apiKey, webhookURL)
}

func ptr(s string) *string { return &s }

func ptrSiNoVacio(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
