package domain

import (
	"strings"
	"testing"
)

// El bug que motiva todo: "1/4" es una fraccion, y los generadores de imagen no
// saben dividir un pollo. Traducirla a piezas anatomicas antes de llegar al
// modelo es el arreglo.
func TestLaFraccionSeTraduceAPiezas(t *testing.T) {
	casos := []struct {
		nombre   string
		fraccion string
		// El texto que va al modelo esta en ingles a proposito (ver porcion.go),
		// asi que aca se afirma sobre la pieza anatomica en ingles.
		contiene string
	}{
		// Como lo escriben las cartas reales de Galponcito y las pollerias.
		{"1/8 pollo", "1/8", "drumstick"},
		{"1/4 pollo", "1/4", "thigh"},
		{"1/2 pollo", "1/2", "breast"},
		{"1 pollo", "1", "WHOLE ROASTED CHICKEN"},

		{"1/4 de pollo a la brasa", "1/4", "knee joint"},
		{"Medio pollo a la brasa", "1/2", "half breast"},
		{"Pollo entero a la brasa", "1", "intact"},
		{"UN CUARTO DE POLLO", "1/4", "thigh"},
		{"  1/4 Pollo  ", "1/4", "thigh"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p, ok := PorcionDePollo(Polleria, c.nombre)
			if !ok {
				t.Fatalf("no reconocio %q como porcion de pollo", c.nombre)
			}
			if p.Fraccion != c.fraccion {
				t.Fatalf("fraccion = %q, esperaba %q", p.Fraccion, c.fraccion)
			}
			if !strings.Contains(p.Piezas, c.contiene) {
				t.Fatalf("las piezas no mencionan %q: %q", c.contiene, p.Piezas)
			}
			// Sin escala el modelo dibuja la pieza correcta del tamano equivocado,
			// que es exactamente la mitad del bug original.
			if strings.TrimSpace(p.Escala) == "" {
				t.Error("la porcion no dice nada del tamano")
			}
		})
	}
}

// Media docena de tequenos no es medio pollo, y medio litro tampoco.
func TestLaFraccionSinPolloNoEsUnaPorcion(t *testing.T) {
	noSon := []string{
		"1/2 porcion de papas",
		"1/2 Porcion ensalada",
		"1 1/2 L.",
		"Gaseosa 1 1/2 L.",
		"Tequenos de Queso (1/2 doc.)",
		"1/4 de kilo de chicharron",
		"Ceviche Mixto",
		"Limonada (jarra)",
	}
	for _, n := range noSon {
		if p, ok := PorcionDePollo(Polleria, n); ok {
			t.Errorf("%q se tomo como porcion de pollo (%s)", n, p.Piezas)
		}
	}
}

// El 1/8 es la trampa: es SOLO la pierna, sin retazo. Si el texto no excluye lo
// demas explicitamente, el modelo dibuja un cuarto.
func TestElOctavoNoLlevaRetazo(t *testing.T) {
	p, ok := PorcionDePollo(Polleria, "1/8 pollo")
	if !ok {
		t.Fatal("no reconocio 1/8")
	}
	// El retazo es justo lo que el 1/4 tiene y el 1/8 no.
	if strings.Contains(p.Piezas, "back and hip still attached") {
		t.Fatalf("el 1/8 no lleva retazo, y las piezas dicen: %q", p.Piezas)
	}
	for _, excluido := range []string{"No thigh attached", "no wing", "no breast"} {
		if !strings.Contains(p.Piezas, excluido) {
			t.Errorf("las piezas del 1/8 deberian decir %q: %q", excluido, p.Piezas)
		}
	}
}

// Cada porcion tiene que traer su ancla de escala, y la escala se expresa en
// ocupacion del plato, nunca en centimetros ni gramos: el modelo no sabe medir,
// pero si sabe llenar un plato.
func TestCadaPorcionAnclaLaEscalaAlPlato(t *testing.T) {
	for _, f := range []string{"1/8 pollo", "1/4 pollo", "1/2 pollo", "1 pollo"} {
		p, _ := PorcionDePollo(Polleria, f)
		if !strings.Contains(p.Escala, "plate") {
			t.Errorf("%s: la escala no menciona el plato: %q", f, p.Escala)
		}
		for _, unidad := range []string{"cm", "gram", "inch", "kg"} {
			if strings.Contains(strings.ToLower(p.Escala), unidad) {
				t.Errorf("%s: la escala usa una unidad medible (%q), y el modelo no sabe medir: %q",
					f, unidad, p.Escala)
			}
		}
	}
}

// La fraccion NUNCA puede aparecer en el texto que se le manda al modelo: es
// justo lo que causaba el bug.
func TestLaFraccionNoLlegaAlModelo(t *testing.T) {
	prohibidas := []string{"1/4", "1/8", "1/2", "quarter chicken", "half chicken", "a quarter of"}
	for _, f := range []string{"1/8 pollo", "1/4 pollo", "1/2 pollo", "1 pollo"} {
		p, _ := PorcionDePollo(Polleria, f)
		texto := strings.ToLower(p.Piezas + " " + p.Escala)
		for _, mala := range prohibidas {
			if strings.Contains(texto, mala) {
				t.Errorf("%s: el texto para el modelo contiene la fraccion %q, que es el bug original",
					f, mala)
			}
		}
	}
}

// Cada porcion tiene que ser distinta de las otras, o no sirve de nada
// traducirlas.
func TestLasCuatroPorcionesSonDistintas(t *testing.T) {
	vistas := map[string]string{}
	for _, f := range []string{"1/8 pollo", "1/4 pollo", "1/2 pollo", "1 pollo"} {
		p, ok := PorcionDePollo(Polleria, f)
		if !ok {
			t.Fatalf("no reconocio %q", f)
		}
		if antes, repetida := vistas[p.Piezas]; repetida {
			t.Fatalf("%q y %q describen lo mismo", antes, f)
		}
		vistas[p.Piezas] = f
	}
}

// El pollo entero sale del horno ENTERO, con su forma de ave. Lo tuve mal en la
// primera version —"abierto y aplanado"— y el dueno lo corrigio: en la polleria
// se parte solo si el cliente lo pide.
func TestElPolloEnteroNoVaPartido(t *testing.T) {
	p, ok := PorcionDePollo(Polleria, "1 pollo")
	if !ok {
		t.Fatal("no reconocio el pollo entero")
	}
	for _, prohibido := range []string{"opened flat", "spread open", "cut lengthwise"} {
		// Solo cuenta como error si lo AFIRMA; el texto niega esas formas a
		// proposito ("not opened flat"), y esa negacion tiene que quedarse.
		if strings.Contains(p.Piezas, prohibido) && !strings.Contains(p.Piezas, "not "+prohibido) {
			t.Errorf("el pollo entero no va partido, y las piezas dicen %q: %q", prohibido, p.Piezas)
		}
	}
	for _, exigido := range []string{"intact", "not butterflied", "not carved"} {
		if !strings.Contains(p.Piezas, exigido) {
			t.Errorf("las piezas del pollo entero deberian decir %q: %q", exigido, p.Piezas)
		}
	}
}
