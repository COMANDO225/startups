package domain

import "testing"

// El default de ClasificarCodigo debe ser rechazado: dar por bueno un
// comprobante que SUNAT no acepto es mucho peor que lo contrario.
func TestClasificarCodigo(t *testing.T) {
	casos := []struct {
		codigo   string
		esperado Estado
		porque   string
	}{
		{"0", EstadoAceptado, "aceptacion limpia"},
		{"4000", EstadoObservado, "4xxx es aceptado con advertencias"},
		{"4321", EstadoObservado, "4xxx es aceptado con advertencias"},
		{"2027", EstadoRechazado, "2xxx rechaza"},
		{"3105", EstadoRechazado, "3xxx rechaza"},
		{"1001", EstadoRechazado, "1xxx rechaza"},
		{"1033", EstadoDuplicado, "ya registrado previamente"},
		{"2109", EstadoDuplicado, "ya registrado previamente"},
		{"99999", EstadoRechazado, "codigo fuera de rango conocido"},

		// Greenter deja el codigo crudo cuando no encuentra digitos en el
		// SoapFault: eso significa que SUNAT nunca dictamino el documento.
		{"HTTP", EstadoError, "un 401 es transporte, no rechazo (visto en produccion)"},
		{"SOAP-ENV:Server", EstadoError, "fallo del servidor, reintentable"},
		{"", EstadoError, "sin codigo no sabemos que dijo SUNAT"},
		{"desconocido", EstadoError, "no es un codigo de SUNAT"},
	}

	for _, c := range casos {
		if got := ClasificarCodigo(c.codigo); got != c.esperado {
			t.Errorf("ClasificarCodigo(%q) = %q, esperaba %q (%s)", c.codigo, got, c.esperado, c.porque)
		}
	}
}

// El bug que motivo la separacion: bajo carga, SUNAT devolvio 401 y el
// comprobante quedaba rechazado, terminal, con el correlativo quemado. Un fallo
// de red no puede tener ese efecto.
func TestFalloDeTransporteDejaElComprobanteRetomable(t *testing.T) {
	c := New(NuevoComprobante{ID: "1", TipoDoc: TipoFactura, Serie: "F001", Correlativo: 1})

	c.Resolver("HTTP", "Unauthorized", nil, nil)

	if c.Estado().EsFinal() {
		t.Fatalf("estado %q es final: un fallo de red mataria el comprobante", c.Estado())
	}

	tomables := map[string]bool{}
	for _, e := range EstadosTomables() {
		tomables[e] = true
	}
	if !tomables[string(c.Estado())] {
		t.Fatalf("estado %q no es tomable: nadie reintentaria el envio", c.Estado())
	}
}

func TestEstadosFinalesNoSonTomables(t *testing.T) {
	tomables := map[string]bool{}
	for _, e := range EstadosTomables() {
		tomables[e] = true
	}

	finales := []Estado{EstadoAceptado, EstadoObservado, EstadoRechazado, EstadoDuplicado}
	for _, e := range finales {
		if tomables[string(e)] {
			t.Errorf("%q es final y no deberia poder retomarse: se reenviaria a SUNAT", e)
		}
	}
}

func TestPrefijoSerieValido(t *testing.T) {
	casos := []struct {
		tipo     TipoDoc
		serie    string
		esperado bool
	}{
		{TipoFactura, "F001", true},
		{TipoFactura, "f001", true},
		{TipoFactura, "B001", false},
		{TipoBoleta, "B001", true},
		{TipoBoleta, "F001", false},
		{TipoNotaCredito, "F001", true},
		{TipoNotaCredito, "B001", true},
		{TipoNotaCredito, "X001", false},
		{TipoFactura, "F1", false},
		{TipoFactura, "F0001", false},
		{TipoFactura, "", false},
	}

	for _, c := range casos {
		if got := c.tipo.PrefijoSerieValido(c.serie); got != c.esperado {
			t.Errorf("TipoDoc(%q).PrefijoSerieValido(%q) = %v, esperaba %v", c.tipo, c.serie, got, c.esperado)
		}
	}
}

// Resolver no debe borrar el XML ya guardado si SUNAT responde sin el.
func TestResolverConservaXMLPrevio(t *testing.T) {
	c := New(NuevoComprobante{ID: "1", TipoDoc: TipoFactura, Serie: "F001", Correlativo: 1})
	c.MarcarTicket("ticket-1", []byte("<xml firmado/>"))

	c.Resolver("0", "aceptada", nil, []byte("cdr"))

	if len(c.XML()) == 0 {
		t.Fatal("se perdio el XML firmado al resolver")
	}
	if c.Estado() != EstadoAceptado {
		t.Fatalf("estado = %q", c.Estado())
	}
}

func TestDuplicadoRequiereRevision(t *testing.T) {
	if !EstadoDuplicado.RequiereRevision() {
		t.Fatal("un duplicado debe marcarse para revision: SUNAT ya lo tiene")
	}
	if EstadoAceptado.RequiereRevision() {
		t.Fatal("un aceptado no requiere revision")
	}
}
