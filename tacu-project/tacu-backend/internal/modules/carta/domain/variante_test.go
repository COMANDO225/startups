package domain_test

import (
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// EL DEFECTO MEDIDO: de 23 fotos de laboratorio, la unica clase de error
// sistematica fue esta. "Chicharron Mixto" salio con pescado y nada mas, y
// "Coca Cola 1L" salio en la botellita personal de vidrio.
func TestLaVarianteDelNombreLlegaAlPrompt(t *testing.T) {
	casos := []struct {
		nombre string
		espera string
	}{
		{"Chicharrón Mixto", "assortment of several different seafoods"},
		{"Ceviche de Conchas Negras", "black clams"},
		{"Coca Cola 1L", "one-litre bottle"},
		{"Coca Cola Personal", "small individual single-serve"},
		{"Jarra de Chicha Morada", "served by the jug"},
		{"Conchas a la Parmesana 1/2 doc", "exactly six pieces"},
		{"Chita a lo Macho", "smothered in a thick"},
	}

	for _, c := range casos {
		if got := domain.VarianteDePlato(c.nombre); !strings.Contains(got, c.espera) {
			t.Errorf("%q no dijo %q: %q", c.nombre, c.espera, got)
		}
	}
}

// Los dos ejes son independientes: uno dice que lleva y el otro cuanto viene.
func TestContenidoYTamanoSePidenALaVez(t *testing.T) {
	got := domain.VarianteDePlato("Jalea Mixta para 2 personas 1/2 doc")
	if !strings.Contains(got, "assortment of several") || !strings.Contains(got, "six pieces") {
		t.Errorf("falto uno de los dos ejes: %q", got)
	}
}

// Sin esto, "1L" y "litro" se piden dos veces en el mismo prompt y el modelo
// recibe dos frases compitiendo por el mismo envase.
func TestSoloUnaVariantePorEje(t *testing.T) {
	got := domain.VarianteDePlato("Coca Cola 1L (1 litro)")
	if n := strings.Count(got, "SIZE —"); n != 1 {
		t.Errorf("pidio el tamano %d veces: %q", n, got)
	}
}

func TestUnNombreSinVarianteNoInventaNada(t *testing.T) {
	if got := domain.VarianteDePlato("Ceviche Clásico"); got != "" {
		t.Errorf("se invento una variante: %q", got)
	}
}

// El banco tiene UNA entrada de chaufa y es la de mariscos, asi que "Fuente de
// Chaufa de Pescado" salia con langostinos y aros de calamar. Y el "de pescado"
// no puede ganarle a una variante que dice mas: un pescado a lo macho va bajo
// su salsa de mariscos, no "solo pescado".
func TestDePescadoEsPescadoYNoLePisaALoMacho(t *testing.T) {
	if got := domain.VarianteDePlato("Chaufa de Pescado"); !strings.Contains(got, "only with fish") {
		t.Errorf("el chaufa de pescado sigue siendo de mariscos: %q", got)
	}
	if got := domain.VarianteDePlato("Pescado a lo Macho"); !strings.Contains(got, "smothered") {
		t.Errorf("a lo macho perdio su salsa: %q", got)
	}
}
