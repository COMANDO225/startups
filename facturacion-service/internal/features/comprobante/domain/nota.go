package domain

import "strings"

// Catalogo 09: motivos de nota de credito. Una nota de credito rebaja o anula
// el comprobante que modifica.
var motivosNotaCredito = map[string]string{
	"01": "anulacion de la operacion",
	"02": "anulacion por error en el RUC",
	"03": "correccion por error en la descripcion",
	"04": "descuento global",
	"05": "descuento por item",
	"06": "devolucion total",
	"07": "devolucion por item",
	"08": "bonificacion",
	"09": "disminucion en el valor",
	"10": "otros conceptos",
	"11": "ajustes de operaciones de exportacion",
	"12": "ajustes afectos al IVAP",
	"13": "correccion del monto neto pendiente de pago",
}

// Catalogo 10: motivos de nota de debito. Es un catalogo distinto del 09 y con
// codigos distintos; usar el equivocado es rechazo seguro de SUNAT.
var motivosNotaDebito = map[string]string{
	"01": "intereses por mora",
	"02": "aumento en el valor",
	"03": "penalidades u otros conceptos al alza",
	"11": "ajustes de operaciones de exportacion",
	"12": "ajustes afectos al IVAP",
}

// MotivosDe devuelve el catalogo que corresponde al tipo de nota.
func MotivosDe(t TipoDoc) map[string]string {
	switch t {
	case TipoNotaCredito:
		return motivosNotaCredito
	case TipoNotaDebito:
		return motivosNotaDebito
	}
	return nil
}

// Nota son los campos que solo llevan las notas de credito y debito.
type Nota struct {
	CodigoMotivo      string
	DescripcionMotivo string
	RefTipoDoc        string
	RefSerieNumero    string
}

// ValidarNota comprueba lo que SUNAT exige de una nota antes de asignar
// correlativo. El motor tambien lo verifica, pero para entonces el numero ya se
// consumio.
func ValidarNota(n Nota, tipoDoc TipoDoc) error {
	catalogo := MotivosDe(tipoDoc)
	if catalogo == nil {
		return nil
	}

	if _, ok := catalogo[n.CodigoMotivo]; !ok {
		return ErrNotaInvalida("codigo_motivo " + n.CodigoMotivo + " no pertenece al catalogo de " + nombreCatalogo(tipoDoc))
	}

	// SUNAT exige el sustento por escrito, no solo el codigo.
	if len(strings.TrimSpace(n.DescripcionMotivo)) < 3 {
		return ErrNotaInvalida("descripcion_motivo es obligatoria y debe explicar el sustento")
	}

	if strings.TrimSpace(n.RefSerieNumero) == "" {
		return ErrNotaInvalida("referencia.serie_numero es obligatoria: una nota siempre modifica otro comprobante")
	}

	// El documento afectado tiene que ser una factura o una boleta.
	switch n.RefTipoDoc {
	case string(TipoFactura), string(TipoBoleta):
	default:
		return ErrNotaInvalida("referencia.tipo_doc debe ser 01 (factura) o 03 (boleta), llego " + n.RefTipoDoc)
	}

	return nil
}

func nombreCatalogo(t TipoDoc) string {
	if t == TipoNotaDebito {
		return "notas de debito (catalogo 10)"
	}
	return "notas de credito (catalogo 09)"
}
