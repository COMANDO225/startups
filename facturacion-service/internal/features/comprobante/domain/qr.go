package domain

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

// QR arma la cadena obligatoria del codigo QR de la representacion impresa,
// exigida a los emisores del SEE-Contribuyente desde enero de 2019. Los campos
// van separados por "|" y en este orden exacto.
type QR struct {
	RUCEmisor       string
	TipoDoc         TipoDoc
	Serie           string
	Correlativo     int64
	IGV             string
	Total           string
	FechaEmision    time.Time
	TipoDocReceptor string
	NumDocReceptor  string

	// ValorResumen es el DigestValue de la firma. Ata el QR al XML concreto que
	// SUNAT recibio, asi que no puede inventarse ni calcularse por otro lado.
	ValorResumen string
}

func (q QR) Cadena() string {
	return strings.Join([]string{
		q.RUCEmisor,
		string(q.TipoDoc),
		q.Serie,
		strconv.FormatInt(q.Correlativo, 10),
		q.IGV,
		q.Total,
		// La misma derivacion que usa el XML, para que el QR no pueda quedar
		// con una fecha distinta a la del documento firmado.
		FechaEmisionLima(q.FechaEmision).Format("2006-01-02"),
		q.TipoDocReceptor,
		q.NumDocReceptor,
		q.ValorResumen,
	}, "|")
}

// DigestDeXML saca el DigestValue de la firma del XML ya firmado.
//
// Se lee del XML guardado en vez de persistirlo en su propia columna: es un
// dato derivado, y tenerlo por duplicado solo abre la puerta a que discrepen.
func DigestDeXML(firmado []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(firmado))

	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}

		inicio, ok := tok.(xml.StartElement)
		if !ok || inicio.Name.Local != "DigestValue" {
			continue
		}

		var valor string
		if dec.DecodeElement(&valor, &inicio) != nil {
			return ""
		}
		return strings.TrimSpace(valor)
	}
}
