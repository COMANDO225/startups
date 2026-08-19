package dinero

import "testing"

// Los numeros reales del producto, medidos: si estas conversiones se desvian,
// el presupuesto por importacion deja de significar lo que dice.
func TestLosCostosRealesConviertenExacto(t *testing.T) {
	casos := []struct {
		que      string
		dolares  float64
		esperado MicrosUSD
	}{
		{"presupuesto por importacion", 3.00, 3_000_000},
		{"una foto con la lite", 0.0336, 33_600},
		{"una foto con flash-image", 0.067, 67_000},
		{"una foto con gpt-image-2", 0.053, 53_000},
		{"leer la carta de galponcito", 0.017, 17_000},
		{"cero", 0, 0},
	}
	for _, c := range casos {
		if got := USD(c.dolares); got != c.esperado {
			t.Errorf("%s: USD(%v) = %d, esperaba %d", c.que, c.dolares, got, c.esperado)
		}
	}
}

// 60 fotos tienen que dar exactamente $2.016 y entrar en el presupuesto. Con
// float64 acumulado esto arrastra error; con enteros no.
func TestSesentaFotosEntranEnElPresupuesto(t *testing.T) {
	const presupuesto = MicrosUSD(3_000_000)
	foto := USD(0.0336)

	var total MicrosUSD
	for range 60 {
		total += foto
	}

	if total != 2_016_000 {
		t.Fatalf("60 fotos = %d micros, esperaba 2016000", total)
	}
	if total > presupuesto {
		t.Fatalf("60 fotos (%s) no entran en el presupuesto (%s)", total, presupuesto)
	}

	// Y con la extraccion incluida tambien.
	if conCarta := total + USD(0.017); conCarta > presupuesto {
		t.Fatalf("carta + 60 fotos = %s, se pasa de %s", conCarta, presupuesto)
	}
}

// Cuantas fotos caben antes de agotar el presupuesto. Es el numero que decide
// que ve el dueno cuando se le acaba.
func TestCuantasFotosCabenEnElPresupuesto(t *testing.T) {
	caben := int(MicrosUSD(3_000_000) / USD(0.0336))
	if caben != 89 {
		t.Fatalf("caben %d fotos, esperaba 89", caben)
	}
}

func TestElFormatoNoEscondeCentavos(t *testing.T) {
	casos := map[MicrosUSD]string{
		0:         "$0",
		33_600:    "$0.0336", // una foto: redondear a $0.03 son 12 centavos por carta
		17_000:    "$0.017",
		2_016_000: "$2.016",
		3_000_000: "$3.00",
		1_500_000: "$1.50",
		1:         "$0.000001",
	}
	for micros, esperado := range casos {
		if got := micros.String(); got != esperado {
			t.Errorf("%d micros = %q, esperaba %q", micros, got, esperado)
		}
	}
}

// El redondeo tiene que ser al mas cercano, no truncar: truncar sistematicamente
// hacia abajo hace que el presupuesto parezca alcanzar mas de lo que alcanza.
func TestUSDRedondeaAlMasCercano(t *testing.T) {
	casos := map[float64]MicrosUSD{
		0.0000004: 0,
		0.0000006: 1,
		0.0000015: 2,
	}
	for d, esperado := range casos {
		if got := USD(d); got != esperado {
			t.Errorf("USD(%v) = %d, esperaba %d", d, got, esperado)
		}
	}
}

func TestDolaresVuelveAlOrigen(t *testing.T) {
	if d := USD(0.0336).Dolares(); d != 0.0336 {
		t.Errorf("ida y vuelta = %v, esperaba 0.0336", d)
	}
}
