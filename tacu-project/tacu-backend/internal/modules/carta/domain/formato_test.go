package domain_test

import (
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

func TestFormatoDePlato(t *testing.T) {
	casos := []struct {
		tipo      domain.Tipo
		nombre    string
		categoria string
		quiere    string // clave del formato, "" = ninguno
	}{
		// Fuente, como lo escribe La Tribuna del Sur.
		{domain.Cevicheria, "Fuente de Ceviche", "Fuente Familiar", "fuente"},
		{domain.Cevicheria, "Fuente de Chicharrón de Pescado", "Fuente Familiar", "fuente"},
		{domain.Cevicheria, "Ceviche Familiar", "Segundos", "fuente"},
		{domain.Cevicheria, "Parrilla para compartir", "", "fuente"},

		// Trio. El nombre NO dice que sea un trio: lo dice la categoria.
		{domain.Cevicheria, "Ceviche + Arroz c/ Mariscos + Chicharrón Mixto", "Tríos", "trio"},
		{domain.Cevicheria, "Ceviche + Chaufa de Mariscos + Chicharrón de Pescado", "Tríos", "trio"},
		{domain.Cevicheria, "Trio Marino", "Segundos", "trio"},
		// Tres componentes sueltos, sin seccion propia: sigue siendo un trio.
		{domain.Cevicheria, "Ceviche + Arroz + Chicharrón", "Segundos", "trio"},

		// Ronda.
		{domain.Cevicheria, "Ronda Marina", "Segundos", "ronda"},

		// Individuales: la inmensa mayoria de la carta.
		{domain.Cevicheria, "Ceviche de Pescado", "Ceviches", ""},
		{domain.Cevicheria, "Chicharrón de Pescado", "Piqueos", ""},
		// DOS componentes no son un trio: es un duo y va en plato normal.
		{domain.Cevicheria, "Ceviche + Chicharrón de Pescado", "Combinados", ""},
		{domain.Cevicheria, "", "", ""},
	}

	for _, c := range casos {
		t.Run(c.nombre+"/"+c.categoria, func(t *testing.T) {
			f, ok := domain.FormatoDePlato(c.tipo, c.nombre, c.categoria)
			if c.quiere == "" {
				if ok {
					t.Fatalf("%q no deberia tener formato propio, dio %q", c.nombre, f.Clave)
				}
				return
			}
			if !ok {
				t.Fatalf("%q deberia dar formato %q y no dio ninguno", c.nombre, c.quiere)
			}
			if f.Clave != c.quiere {
				t.Errorf("%q dio formato %q, quiero %q", c.nombre, f.Clave, c.quiere)
			}
		})
	}
}

// EL ORDEN DE LA DETECCION. Un trio de S/80 es un trio GRANDE, no una fuente:
// sigue llegando en su bandeja de tres compartimentos. Si "familiar" ganara,
// volverian los tres componentes revueltos en una bandeja ovalada.
func TestUnTrioFamiliarSigueSiendoTrio(t *testing.T) {
	f, ok := domain.FormatoDePlato(domain.Cevicheria, "Trio Marino Familiar", "Tríos")
	if !ok || f.Clave != "trio" {
		t.Fatalf("dio %q, un trio familiar sigue siendo trio", f.Clave)
	}
}

// Cada componente del nombre tiene que acabar en SU compartimento. Sin eso el
// modelo los amontona, que es exactamente como salia la foto mala.
func TestElTrioAsignaUnComponenteACadaCompartimento(t *testing.T) {
	f, _ := domain.FormatoDePlato(domain.Cevicheria, "Ceviche + Arroz c/ Mariscos + Chicharrón Mixto", "Tríos")

	for _, parte := range []string{"Ceviche", "Arroz c/ Mariscos", "Chicharrón Mixto"} {
		if !strings.Contains(f.Receta.Reparto, parte) {
			t.Errorf("el reparto no coloca %q en ningun compartimento", parte)
		}
	}
	for _, posicion := range []string{"LEFT COMPARTMENT", "MIDDLE COMPARTMENT", "RIGHT COMPARTMENT"} {
		if !strings.Contains(f.Receta.Reparto, posicion) {
			t.Errorf("falta %s: sin posiciones el modelo no separa las tres cosas", posicion)
		}
	}
}

// La ronda ES el surtido: sin enumerar lo que lleva, el modelo devolvia una
// sopa de mariscos. Es el unico formato que describe su propia comida.
func TestLaRondaEnumeraSuSurtidoYSuCentro(t *testing.T) {
	f, ok := domain.FormatoDePlato(domain.Cevicheria, "Ronda Marina", "Segundos")
	if !ok {
		t.Fatal("Ronda Marina deberia tener formato propio")
	}

	if !strings.Contains(f.Receta.Recipiente, "wedge-shaped compartments") {
		t.Error("la ronda va en plato redondo con acoples radiales")
	}
	if !strings.Contains(f.Receta.Reparto, "CENTRE WELL") {
		t.Error("a la ronda le falta el centro, que es donde va la causa")
	}
	// Los cuatro que no pueden faltar: crudo, frito, arroz y conchas.
	for _, componente := range []string{"ceviche", "deep-fried", "rice", "mussels"} {
		if !strings.Contains(strings.ToLower(f.Receta.Reparto), componente) {
			t.Errorf("la ronda no incluye %q y sin variedad no es una ronda", componente)
		}
	}
}

func TestComponentes(t *testing.T) {
	casos := []struct {
		nombre string
		quiere int
	}{
		{"Ceviche + Arroz c/ Mariscos + Chicharrón Mixto", 3},
		{"Ceviche + Chicharrón de Pescado", 2},
		{"Ceviche de Pescado", 0},
		{"Ceviche +  + Arroz", 2}, // los vacios no cuentan
	}
	for _, c := range casos {
		if n := len(domain.Componentes(c.nombre)); n != c.quiere {
			t.Errorf("Componentes(%q) dio %d, quiero %d", c.nombre, n, c.quiere)
		}
	}
}
