package domain

import "testing"

func notaValida() Nota {
	return Nota{
		CodigoMotivo:      "01",
		DescripcionMotivo: "anulacion por error de digitacion",
		RefTipoDoc:        "01",
		RefSerieNumero:    "F001-67",
	}
}

// Los catalogos 09 y 10 son distintos. Usar el codigo de uno en el otro es
// rechazo seguro de SUNAT, y el correlativo se pierde igual.
func TestCatalogosDeNotaNoSeMezclan(t *testing.T) {
	casos := []struct {
		nombre string
		codigo string
		tipo   TipoDoc
		valido bool
	}{
		{"credito con motivo de credito", "06", TipoNotaCredito, true},
		{"credito con devolucion por item", "07", TipoNotaCredito, true},
		{"credito con correccion de monto neto", "13", TipoNotaCredito, true},
		{"credito con codigo inexistente", "14", TipoNotaCredito, false},

		{"debito con intereses por mora", "01", TipoNotaDebito, true},
		{"debito con penalidades", "03", TipoNotaDebito, true},
		{"debito con ajuste de exportacion", "11", TipoNotaDebito, true},

		// 06 (devolucion total) existe en el catalogo 09 pero no en el 10.
		{"debito con motivo de credito", "06", TipoNotaDebito, false},
		{"debito con codigo inexistente", "99", TipoNotaDebito, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			n := notaValida()
			n.CodigoMotivo = c.codigo

			err := ValidarNota(n, c.tipo)
			if c.valido && err != nil {
				t.Fatalf("rechazo un motivo valido: %v", err)
			}
			if !c.valido && err == nil {
				t.Fatal("acepto un motivo que no esta en el catalogo")
			}
		})
	}
}

func TestNotaExigeSustentoYReferencia(t *testing.T) {
	casos := []struct {
		nombre string
		mutar  func(*Nota)
	}{
		{"sin descripcion", func(n *Nota) { n.DescripcionMotivo = "" }},
		{"descripcion de un caracter", func(n *Nota) { n.DescripcionMotivo = "x" }},
		{"descripcion en blanco", func(n *Nota) { n.DescripcionMotivo = "   " }},
		{"sin serie del documento afectado", func(n *Nota) { n.RefSerieNumero = "" }},
		{"documento afectado que es otra nota", func(n *Nota) { n.RefTipoDoc = "07" }},
		{"documento afectado sin tipo", func(n *Nota) { n.RefTipoDoc = "" }},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			n := notaValida()
			c.mutar(&n)

			if err := ValidarNota(n, TipoNotaCredito); err == nil {
				t.Fatal("acepto una nota que SUNAT rechazaria")
			}
		})
	}
}

// Una factura o boleta no lleva motivo: la validacion no debe estorbarles.
func TestValidarNotaNoAplicaAFacturasNiBoletas(t *testing.T) {
	vacia := Nota{}

	for _, tipo := range []TipoDoc{TipoFactura, TipoBoleta} {
		if err := ValidarNota(vacia, tipo); err != nil {
			t.Fatalf("exigio motivo de nota a un %q: %v", tipo, err)
		}
	}
}

// Una nota sobre boleta viaja por Resumen Diario; una sobre factura va directa.
func TestNotaHeredaElCaminoDelDocumentoAfectado(t *testing.T) {
	casos := []struct {
		tipo        TipoDoc
		serie       string
		porResumen  bool
		descripcion string
	}{
		{TipoNotaCredito, "F001", false, "nota sobre factura va directa a SUNAT"},
		{TipoNotaCredito, "B001", true, "nota sobre boleta viaja en el resumen"},
		{TipoNotaDebito, "F001", false, "nota de debito sobre factura va directa"},
		{TipoNotaDebito, "B001", true, "nota de debito sobre boleta va en resumen"},
	}

	for _, c := range casos {
		if got := ViajaPorResumen(c.tipo, c.serie); got != c.porResumen {
			t.Errorf("%s: ViajaPorResumen(%q, %q) = %v", c.descripcion, c.tipo, c.serie, got)
		}
	}
}
