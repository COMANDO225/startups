package http

import (
	"encoding/base64"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
)

type ComprobanteResponse struct {
	ID           string    `json:"id"`
	Numero       string    `json:"numero"`
	TipoDoc      string    `json:"tipo_doc"`
	Serie        string    `json:"serie"`
	Correlativo  int64     `json:"correlativo"`
	Estado       string    `json:"estado"`
	Moneda       string    `json:"moneda"`
	ImporteTotal string    `json:"importe_total"`
	FechaEmision string    `json:"fecha_emision"`
	CodigoSunat  string    `json:"codigo_sunat,omitempty"`
	MensajeSunat string    `json:"mensaje_sunat,omitempty"`
	Ticket       string    `json:"ticket,omitempty"`
	XMLB64       string    `json:"xml_b64,omitempty"`
	CDRB64       string    `json:"cdr_b64,omitempty"`
	QR           string    `json:"qr,omitempty"`
	Hash         string    `json:"hash,omitempty"`
	Intentos     int       `json:"intentos"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toResponse(c *domain.Comprobante, incluirDocumentos bool) ComprobanteResponse {
	r := ComprobanteResponse{
		ID:           c.ID(),
		Numero:       c.Numero(),
		TipoDoc:      string(c.TipoDoc()),
		Serie:        c.Serie(),
		Correlativo:  c.Correlativo(),
		Estado:       string(c.Estado()),
		Moneda:       c.Moneda(),
		ImporteTotal: c.ImporteTotal(),
		FechaEmision: c.FechaEmision().Format("2006-01-02"),
		CodigoSunat:  c.CodigoSunat(),
		MensajeSunat: c.MensajeSunat(),
		Ticket:       c.Ticket(),
		Intentos:     c.Intentos(),
		CreatedAt:    c.CreatedAt(),
		UpdatedAt:    c.UpdatedAt(),
	}

	if incluirDocumentos {
		if xml := c.XML(); len(xml) > 0 {
			r.XMLB64 = base64.StdEncoding.EncodeToString(xml)
		}
		if cdr := c.CDR(); len(cdr) > 0 {
			r.CDRB64 = base64.StdEncoding.EncodeToString(cdr)
		}
	}

	return r
}
