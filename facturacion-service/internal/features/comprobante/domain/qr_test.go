package domain

import (
	"strings"
	"testing"
	"time"
)

const xmlFirmado = `<?xml version="1.0"?>
<Invoice xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
  <ext:UBLExtensions xmlns:ext="urn:x"><ext:UBLExtension><ext:ExtensionContent>
    <ds:Signature Id="GreenterSign"><ds:SignedInfo><ds:Reference URI="">
      <ds:DigestValue>m5UaryxZb/knHNYWVRz4MuTNw7g=</ds:DigestValue>
    </ds:Reference></ds:SignedInfo></ds:Signature>
  </ext:ExtensionContent></ext:UBLExtension></ext:UBLExtensions>
</Invoice>`

func TestDigestDeXML(t *testing.T) {
	if got := DigestDeXML([]byte(xmlFirmado)); got != "m5UaryxZb/knHNYWVRz4MuTNw7g=" {
		t.Fatalf("DigestValue = %q", got)
	}
}

// Un XML sin firma o ilegible no puede tumbar la impresion: devuelve vacio y el
// QR sale sin resumen, que es preferible a no poder emitir el comprobante.
func TestDigestDeXMLSinFirma(t *testing.T) {
	for _, caso := range []string{"", "no es xml", "<Invoice></Invoice>"} {
		if got := DigestDeXML([]byte(caso)); got != "" {
			t.Errorf("DigestDeXML(%q) = %q, esperaba vacio", caso, got)
		}
	}
}

// El orden y el separador los fija SUNAT: cualquier cambio invalida el QR.
func TestCadenaQR(t *testing.T) {
	fecha, _ := time.Parse("2006-01-02", "2026-08-05")

	q := QR{
		RUCEmisor:       "20000000001",
		TipoDoc:         TipoFactura,
		Serie:           "F001",
		Correlativo:     67,
		IGV:             "18.00",
		Total:           "118.00",
		FechaEmision:    fecha,
		TipoDocReceptor: DocRUC,
		NumDocReceptor:  "20100070970",
		ValorResumen:    "m5UaryxZb/knHNYWVRz4MuTNw7g=",
	}

	esperado := "20000000001|01|F001|67|18.00|118.00|2026-08-05|6|20100070970|m5UaryxZb/knHNYWVRz4MuTNw7g="
	if got := q.Cadena(); got != esperado {
		t.Fatalf("cadena QR:\n  obtuve   %s\n  esperaba %s", got, esperado)
	}

	if n := len(strings.Split(esperado, "|")); n != 10 {
		t.Fatalf("la cadena debe tener 10 campos, tiene %d", n)
	}
}
