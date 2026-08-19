package dinero

import "testing"

func TestParsearFormatosDeCartaPeruana(t *testing.T) {
	casos := []struct {
		texto    string
		esperado Centimos
	}{
		{"S/ 12.50", 1250},
		{"S/12.50", 1250},
		{"S/. 12.50", 1250},
		{"s/ 12.50", 1250},
		{"12.50", 1250},
		{"12,50", 1250}, // coma decimal
		{"25", 2500},    // sin decimales: la tendencia de las cartas nuevas
		{"S/ 25", 2500},
		{"  8.9  ", 890}, // un solo decimal
		{"1,250.00", 125000},
		{"PEN 45.00", 4500},
		{"0.50", 50},
	}

	for _, c := range casos {
		got, err := Parsear(c.texto)
		if err != nil {
			t.Errorf("Parsear(%q): %v", c.texto, err)
			continue
		}
		if got != c.esperado {
			t.Errorf("Parsear(%q) = %d, esperaba %d", c.texto, got, c.esperado)
		}
	}
}

// El parser NO adivina. Su trabajo es decir "no se" para que el precio caiga a
// revision humana; un parser generoso reintroduce el problema que existe para
// detectar.
func TestParsearRechazaLoAmbiguo(t *testing.T) {
	ambiguos := []string{
		"",
		"   ",
		"S/",
		"gratis",
		"12 a 15",  // rango
		"12/15",    // dos precios
		"desde 12", // texto
		"12.505",   // tres decimales: no es un precio de carta
		"--",
		"S/ .",
		"1.2.3",
		"999999.00", // por encima del tope razonable
	}

	for _, s := range ambiguos {
		if got, err := Parsear(s); err == nil {
			t.Errorf("Parsear(%q) devolvio %d; deberia rechazar para que vaya a revision", s, got)
		}
	}
}

func TestFormateo(t *testing.T) {
	casos := []struct {
		c     Centimos
		texto string
		soles string
	}{
		{1250, "S/ 12.50", "12.50"},
		{2500, "S/ 25.00", "25.00"},
		{50, "S/ 0.50", "0.50"},
		{125000, "S/ 1250.00", "1250.00"},
		{-1250, "-S/ 12.50", "-12.50"},
	}

	for _, c := range casos {
		if got := c.c.String(); got != c.texto {
			t.Errorf("Centimos(%d).String() = %q, esperaba %q", c.c, got, c.texto)
		}
		if got := c.c.Soles(); got != c.soles {
			t.Errorf("Centimos(%d).Soles() = %q, esperaba %q", c.c, got, c.soles)
		}
	}
}

// La ida y vuelta tiene que ser exacta: es la propiedad que float64 no da.
func TestIdaYVuelta(t *testing.T) {
	for c := Centimos(0); c < 10_000; c += 7 {
		vuelta, err := Parsear(c.Soles())
		if err != nil {
			t.Fatalf("Parsear(%q): %v", c.Soles(), err)
		}
		if vuelta != c {
			t.Fatalf("%d -> %q -> %d", c, c.Soles(), vuelta)
		}
	}
}
