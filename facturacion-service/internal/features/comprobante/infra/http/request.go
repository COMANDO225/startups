package http

import "encoding/json"

type EmitirRequest struct {
	TipoDoc      string          `json:"tipo_doc" validate:"required,oneof=01 03 07 08"`
	Serie        string          `json:"serie" validate:"required,min=4,max=4"`
	Moneda       string          `json:"moneda" validate:"omitempty,len=3"`
	ImporteTotal string          `json:"importe_total" validate:"required,numeric"`
	FechaEmision string          `json:"fecha_emision" validate:"omitempty,datetime=2006-01-02"`
	Comprobante  json.RawMessage `json:"comprobante" validate:"required"`
}

type AnularRequest struct {
	Motivo string `json:"motivo" validate:"required,min=3,max=100"`
}
