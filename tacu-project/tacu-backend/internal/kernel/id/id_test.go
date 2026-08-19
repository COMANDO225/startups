package id

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCadaIdEsDistinto(t *testing.T) {
	vistos := make(map[ID]bool, 10_000)
	for range 10_000 {
		n := Nuevo()
		if vistos[n] {
			t.Fatalf("id repetido: %s", n)
		}
		vistos[n] = true
	}
}

// Se generan ids desde varios workers de foto a la vez. La version con ULID
// necesitaba un mutex propio; esta no, porque google/uuid ya sincroniza dentro.
// Este test es lo que respalda esa afirmacion, y bajo -race tambien delata si
// alguien mete estado compartido en el paquete.
func TestNuevoEsSeguroDesdeVariasGoroutines(t *testing.T) {
	const goroutines, porGoroutina = 16, 500

	var mu sync.Mutex
	vistos := make(map[ID]bool, goroutines*porGoroutina)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			locales := make([]ID, porGoroutina)
			for i := range locales {
				locales[i] = Nuevo()
			}
			mu.Lock()
			defer mu.Unlock()
			for _, n := range locales {
				if vistos[n] {
					t.Errorf("id repetido entre goroutines: %s", n)
					return
				}
				vistos[n] = true
			}
		}()
	}
	wg.Wait()

	if len(vistos) != goroutines*porGoroutina {
		t.Fatalf("salieron %d ids distintos de %d", len(vistos), goroutines*porGoroutina)
	}
}

// Ordenar por tiempo es lo que hace que un indice sobre (importacion_id, id)
// escriba al final y no en medio, y lo unico que UUIDv7 gana sobre v4.
func TestOrdenanPorTiempoDeCreacion(t *testing.T) {
	anterior := Nuevo().String()
	for range 1_000 {
		siguiente := Nuevo().String()
		if siguiente <= anterior {
			t.Fatalf("%s no es mayor que %s: el orden lexicografico no sigue al tiempo",
				siguiente, anterior)
		}
		anterior = siguiente
	}
}

// La version 7 tiene que estar en el nibble que dice la RFC 9562. Si alguien
// cambia Nuevo() por uuid.New() —que es v4— se pierde el orden y nada mas se
// entera.
func TestEsDeVerdadLaVersion7(t *testing.T) {
	n := Nuevo()
	if n.Version() != 7 {
		t.Fatalf("version = %d, esperaba 7", n.Version())
	}
	if n.Variant() != uuid.RFC4122 {
		t.Fatalf("variante = %v, esperaba la de la RFC", n.Variant())
	}

	// Los primeros 48 bits son el instante en milisegundos: es lo que permite
	// que Postgres le saque la fecha con uuid_extract_timestamp.
	seg, nano := n.Time().UnixTime()
	creado := time.Unix(seg, nano)
	// El margen negativo no es capricho: uuid v7 guarda la submilesima en los
	// bits de precision extra, asi que el instante reconstruido puede caer unos
	// microsegundos DELANTE de time.Now() y el test flaqueaba solo.
	if d := time.Since(creado); d < -time.Millisecond || d > time.Minute {
		t.Fatalf("la marca de tiempo dice %v, que esta a %v de ahora", creado, d)
	}
}

func TestParsearRechazaLoQueNoEsUnID(t *testing.T) {
	n := Nuevo()
	vuelto, ok := Parsear(n.String())
	if !ok || vuelto != n {
		t.Fatalf("ida y vuelta rota: %v %v", vuelto, ok)
	}

	malos := []string{
		"",
		"no-soy-un-uuid",
		"01a00238-918f-70df-baf5", // corto
		"01a00238-918f-70df-baf5-e9c30004039d-extra", // largo
		"01a00238-918f-70df-baf5-e9c30004039z",       // z no es hexadecimal
		"'; DROP TABLE plato; --",
	}
	for _, m := range malos {
		if Valido(m) {
			t.Errorf("%q se acepto como id valido", m)
		}
	}
}

// Un id va en una URL: /i/{id}. No puede traer nada que haya que escapar.
func TestElIdVaLimpioEnUnaURL(t *testing.T) {
	s := Nuevo().String()
	if len(s) != 36 {
		t.Fatalf("largo = %d, esperaba 36", len(s))
	}
	for _, r := range s {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || r == '-'
		if !ok {
			t.Fatalf("%q trae el caracter %q", s, r)
		}
	}
	if strings.ContainsAny(s, "/+= ?&#%") {
		t.Fatalf("%q trae caracteres que hay que escapar en una URL", s)
	}
}
