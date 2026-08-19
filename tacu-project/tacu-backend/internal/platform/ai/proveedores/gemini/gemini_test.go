package gemini

import (
	"errors"
	"strings"
	"testing"

	"tacu-backend/internal/platform/ai"
)

// clasificar decide si una llamada se reintenta o se abandona, y de eso depende
// cuanto se paga: un error transitorio marcado como terminal corta la cadena y
// deja al dueno sin su carta; uno terminal marcado como transitorio quema
// intentos pagados contra una pared.

func clasificarDe(msg string) error {
	return (&Proveedor{}).clasificar(errors.New(msg))
}

func TestLoTerminalNoSeReintenta(t *testing.T) {
	casos := []struct {
		mensaje string
		codigo  string
	}{
		{"Error 400: API key not valid. Please pass a valid API key.", "credencial"},
		{"API_KEY_INVALID", "credencial"},
		{"Error 403, Message: Permission denied", "permiso"},
		{"Error 400, Message: Invalid argument", "peticion_invalida"},
		{"blocked by safety settings", "moderacion"},
	}

	for _, c := range casos {
		t.Run(c.codigo, func(t *testing.T) {
			err := clasificarDe(c.mensaje)
			if !ai.EsTerminal(err) {
				t.Fatalf("%q deberia ser terminal: reintentarlo no lo arregla", c.mensaje)
			}
			var ep *ai.ErrProveedor
			if !errors.As(err, &ep) {
				t.Fatalf("no devolvio un ErrProveedor: %T", err)
			}
			if ep.Codigo != c.codigo {
				t.Fatalf("codigo = %q, esperaba %q", ep.Codigo, c.codigo)
			}
		})
	}
}

func TestLoTransitorioSeReintenta(t *testing.T) {
	casos := []struct {
		mensaje string
		codigo  string
	}{
		{"Error 429, Message: Resource exhausted", "cuota"},
		{"Error 503, Message: Service unavailable", "no_disponible"},
		{"context deadline exceeded", "timeout"},
		{"Error 500, Message: Internal error encountered", "error"},
	}

	for _, c := range casos {
		t.Run(c.codigo, func(t *testing.T) {
			err := clasificarDe(c.mensaje)
			if ai.EsTerminal(err) {
				t.Fatalf("%q no deberia ser terminal: suele pasar solo", c.mensaje)
			}
			var ep *ai.ErrProveedor
			if errors.As(err, &ep) && ep.Codigo != c.codigo {
				t.Fatalf("codigo = %q, esperaba %q", ep.Codigo, c.codigo)
			}
		})
	}
}

// La cuota DIARIA es el caso que obliga a distinguir: es un 429 como los demas,
// pero reintentar en segundos no la arregla. Hay que esperar al reset.
func TestLaCuotaDiariaEsTerminal(t *testing.T) {
	diarios := []string{
		"Error 429: Quota exceeded for quota metric 'Generate requests per day' limit: 0",
		"RESOURCE_EXHAUSTED: GenerateRequestsPerDayPerProjectPerModel",
		"429 quota exceeded: daily limit reached",
		"resource_exhausted: quota per_day exceeded",
	}
	for _, m := range diarios {
		t.Run(m[:min(40, len(m))], func(t *testing.T) {
			err := clasificarDe(m)
			if !ai.EsTerminal(err) {
				t.Fatalf("la cuota diaria deberia ser terminal: %q", m)
			}
			var ep *ai.ErrProveedor
			if errors.As(err, &ep) && ep.Codigo != "cuota_diaria" {
				t.Fatalf("codigo = %q, esperaba cuota_diaria", ep.Codigo)
			}
		})
	}
}

// Ante la duda, transitorio. Equivocarse hacia el reintento cuesta segundos;
// equivocarse hacia terminal cancela una carta que si se podia generar.
func TestUnaCuotaPorMinutoNoSeConfundeConLaDiaria(t *testing.T) {
	porMinuto := []string{
		"Error 429, Message: Resource exhausted",
		"429 quota exceeded for requests per minute",
		"RESOURCE_EXHAUSTED: GenerateRequestsPerMinutePerProject",
		"429 rate limit exceeded",
	}
	for _, m := range porMinuto {
		if ai.EsTerminal(clasificarDe(m)) {
			t.Errorf("%q es por minuto y se marco terminal: se abandona una carta que si se podia generar", m)
		}
	}
}

// La palabra "daily" fuera de un error de cuota no lo convierte en cuota.
func TestSoloUnErrorDeCuotaPuedeSerCuotaDiaria(t *testing.T) {
	err := clasificarDe("Error 500: internal error processing your daily digest")
	if ai.EsTerminal(err) {
		t.Fatal("un 500 con la palabra 'daily' no es una cuota diaria")
	}
}

// El mensaje original tiene que sobrevivir la clasificacion: sin el, depurar un
// fallo de produccion es adivinar.
func TestElMensajeOriginalSeConserva(t *testing.T) {
	original := "Error 429, Message: Resource exhausted, Status: RESOURCE_EXHAUSTED"
	err := clasificarDe(original)
	if !strings.Contains(err.Error(), original) {
		t.Fatalf("el error perdio el mensaje del proveedor: %q", err.Error())
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		t.Error("la causa no se puede desenvolver")
	}
}

func TestNivelDensidadCubreTodosLosNiveles(t *testing.T) {
	casos := map[ai.Densidad]bool{
		ai.DensidadPorDefecto: false, // sin nivel: lo decide el proveedor
		ai.DensidadBaja:       true,
		ai.DensidadMedia:      true,
		ai.DensidadAlta:       true,
		ai.DensidadMaxima:     true,
	}
	vistos := map[string]ai.Densidad{}
	for d, esperaNivel := range casos {
		n := string(nivelDensidad(d))
		if esperaNivel && n == "" {
			t.Errorf("la densidad %q no se tradujo a ningun nivel", d)
		}
		if !esperaNivel && n != "" {
			t.Errorf("la densidad por defecto no deberia fijar nivel, dio %q", n)
		}
		if n == "" {
			continue
		}
		if antes, repetido := vistos[n]; repetido {
			t.Errorf("%q y %q dan el mismo nivel %q", antes, d, n)
		}
		vistos[n] = d
	}
}
