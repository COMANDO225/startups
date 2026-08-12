package http

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"facturacion-service/internal/features/comprobante/application/command"
	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"

	"github.com/gofiber/fiber/v3"
)

type CrearEmisorRequest struct {
	RUC             string `json:"ruc" validate:"required,len=11,numeric"`
	RazonSocial     string `json:"razon_social" validate:"required,min=3,max=100"`
	NombreComercial string `json:"nombre_comercial" validate:"omitempty,max=100"`
	Direccion       string `json:"direccion" validate:"required,max=200"`
	Ubigeo          string `json:"ubigeo" validate:"omitempty,len=6,numeric"`
	Departamento    string `json:"departamento" validate:"omitempty,max=60"`
	Provincia       string `json:"provincia" validate:"omitempty,max=60"`
	Distrito        string `json:"distrito" validate:"omitempty,max=60"`

	// El PEM viaja en base64 para no pelear con los saltos de linea en JSON.
	CertPEMB64 string `json:"cert_pem_b64" validate:"required"`
	SolUser    string `json:"sol_user" validate:"required,max=60"`
	SolPass    string `json:"sol_pass" validate:"required,max=100"`
	Produccion bool   `json:"produccion"`
	WebhookURL string `json:"webhook_url" validate:"omitempty,url"`

	Series []struct {
		TipoDoc string `json:"tipo_doc" validate:"required,oneof=01 03 07 08"`
		Serie   string `json:"serie" validate:"required,len=4"`
	} `json:"series" validate:"omitempty,dive"`
}

// EmisorResponse muestra los secretos una sola vez. La API key no vuelve a estar
// disponible: en la base solo queda su hash.
type EmisorResponse struct {
	TenantID      string       `json:"tenant_id"`
	RUC           string       `json:"ruc"`
	APIKey        string       `json:"api_key"`
	WebhookSecret string       `json:"webhook_secret,omitempty"`
	Certificado   certResponse `json:"certificado"`
	Series        []serieResp  `json:"series"`
	Aviso         string       `json:"aviso"`
}

type certResponse struct {
	Titular   string `json:"titular"`
	Emisor    string `json:"emisor"`
	VenceEl   string `json:"vence_el"`
	PorVencer bool   `json:"por_vencer"`
}

type serieResp struct {
	TipoDoc string `json:"tipo_doc"`
	Serie   string `json:"serie"`
}

// CrearEmisor da de alta un emisor. Va detras de un token de administracion
// aparte de las API keys: quien puede llamarlo puede registrar certificados y
// Claves SOL de terceros.
func CrearEmisor(uc *command.CrearEmisor, tokenAdmin string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := autorizarAdmin(c, tokenAdmin); err != nil {
			return err
		}

		var req CrearEmisorRequest
		if err := c.Bind().Body(&req); err != nil {
			return err
		}

		pem, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.CertPEMB64))
		if err != nil {
			return domain.ErrCertificadoInvalido("cert_pem_b64 no es base64 valido")
		}

		series := make([]domain.SerieConfig, len(req.Series))
		for i, s := range req.Series {
			series[i] = domain.SerieConfig{TipoDoc: domain.TipoDoc(s.TipoDoc), Serie: strings.ToUpper(s.Serie)}
		}

		creado, err := uc.Execute(c.Context(), command.CrearEmisorCmd{
			RUC:             req.RUC,
			RazonSocial:     req.RazonSocial,
			NombreComercial: req.NombreComercial,
			Direccion:       req.Direccion,
			Ubigeo:          req.Ubigeo,
			Departamento:    req.Departamento,
			Provincia:       req.Provincia,
			Distrito:        req.Distrito,
			CertPEM:         string(pem),
			SolUser:         req.SolUser,
			SolPass:         req.SolPass,
			Produccion:      req.Produccion,
			WebhookURL:      req.WebhookURL,
			Series:          series,
		})
		if err != nil {
			return err
		}

		resp := EmisorResponse{
			TenantID:      creado.TenantID,
			RUC:           req.RUC,
			APIKey:        creado.APIKey,
			WebhookSecret: creado.WebhookSecret,
			Certificado: certResponse{
				Titular:   creado.Certificado.Titular,
				Emisor:    creado.Certificado.Emisor,
				VenceEl:   creado.Certificado.Hasta.Format("2006-01-02"),
				PorVencer: creado.Certificado.PorVencer(),
			},
			Aviso: "Guarde la api_key ahora: no vuelve a mostrarse.",
		}

		if creado.Certificado.PorVencer() {
			resp.Aviso += " El certificado vence el " + resp.Certificado.VenceEl + ": renuevelo antes."
		}

		for _, s := range creado.Series {
			resp.Series = append(resp.Series, serieResp{TipoDoc: string(s.TipoDoc), Serie: s.Serie})
		}

		return c.Status(fiber.StatusCreated).JSON(resp)
	}
}

// Sin token configurado el alta queda deshabilitada. Dejarla abierta permitiria
// a cualquiera registrar emisores en el servicio.
func autorizarAdmin(c fiber.Ctx, token string) error {
	if token == "" {
		return domainerr.Authentication("El alta de emisores esta deshabilitada").
			WithCode("ALTA_DESHABILITADA").
			WithSuggestion("Configure ADMIN_TOKEN para habilitarla")
	}

	recibido := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(recibido), []byte(token)) != 1 {
		return domainerr.Authentication("Token de administracion invalido").WithCode("ADMIN_TOKEN_INVALIDO")
	}
	return nil
}
