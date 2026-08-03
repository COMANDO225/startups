package command

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
)

// Notificar avisa al cliente que un comprobante llego a estado final. Sin esto
// el 202 obliga a hacer polling.
type Notificar struct {
	repo    domain.Repositorio
	tenants domain.TenantRepositorio
	envio   domain.EnviadorWebhook
}

func NewNotificar(repo domain.Repositorio, tenants domain.TenantRepositorio, envio domain.EnviadorWebhook) *Notificar {
	return &Notificar{repo: repo, tenants: tenants, envio: envio}
}

type EventoWebhook struct {
	Evento        string    `json:"evento"`
	ComprobanteID string    `json:"comprobante_id"`
	Numero        string    `json:"numero"`
	TipoDoc       string    `json:"tipo_doc"`
	Estado        string    `json:"estado"`
	CodigoSunat   string    `json:"codigo_sunat,omitempty"`
	MensajeSunat  string    `json:"mensaje_sunat,omitempty"`
	ImporteTotal  string    `json:"importe_total"`
	OcurridoEn    time.Time `json:"ocurrido_en"`
}

func (uc *Notificar) Execute(ctx context.Context, tenantID, comprobanteID string) error {
	c, err := uc.repo.PorID(ctx, tenantID, comprobanteID)
	if err != nil {
		return err
	}

	// Solo se notifica lo definitivo: avisar de estados intermedios genera
	// ruido y obliga al cliente a distinguirlos.
	if !c.Estado().EsFinal() {
		return nil
	}

	tenant, err := uc.tenants.PorID(ctx, tenantID)
	if err != nil {
		return err
	}

	if tenant.WebhookURL == "" {
		return nil
	}

	cuerpo, err := json.Marshal(EventoWebhook{
		Evento:        "comprobante." + string(c.Estado()),
		ComprobanteID: c.ID(),
		Numero:        c.Numero(),
		TipoDoc:       string(c.TipoDoc()),
		Estado:        string(c.Estado()),
		CodigoSunat:   c.CodigoSunat(),
		MensajeSunat:  c.MensajeSunat(),
		ImporteTotal:  c.ImporteTotal(),
		OcurridoEn:    c.UpdatedAt(),
	})
	if err != nil {
		return err
	}

	if err := uc.envio.Enviar(ctx, tenant.WebhookURL, cuerpo, Firmar(cuerpo, tenant.WebhookSecret)); err != nil {
		return err
	}

	return uc.repo.MarcarWebhookEnviado(ctx, c.ID())
}

// Firmar produce el HMAC que el cliente usa para comprobar que el webhook
// viene de nosotros y no fue alterado.
func Firmar(cuerpo []byte, secreto string) string {
	mac := hmac.New(sha256.New, []byte(secreto))
	mac.Write(cuerpo)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
