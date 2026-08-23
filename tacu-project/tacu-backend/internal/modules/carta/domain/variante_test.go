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
		if got := domain.VarianteDePlato(c.nombre, ""); !strings.Contains(got, c.espera) {
			t.Errorf("%q no dijo %q: %q", c.nombre, c.espera, got)
		}
	}
}

// Los dos ejes son independientes: uno dice que lleva y el otro cuanto viene.
func TestContenidoYTamanoSePidenALaVez(t *testing.T) {
	got := domain.VarianteDePlato("Jalea Mixta para 2 personas 1/2 doc", "")
	if !strings.Contains(got, "assortment of several") || !strings.Contains(got, "six pieces") {
		t.Errorf("falto uno de los dos ejes: %q", got)
	}
}

// Sin esto, "1L" y "litro" se piden dos veces en el mismo prompt y el modelo
// recibe dos frases compitiendo por el mismo envase.
func TestSoloUnaVariantePorEje(t *testing.T) {
	got := domain.VarianteDePlato("Coca Cola 1L (1 litro)", "")
	if n := strings.Count(got, "SIZE —"); n != 1 {
		t.Errorf("pidio el tamano %d veces: %q", n, got)
	}
}

func TestUnNombreSinVarianteNoInventaNada(t *testing.T) {
	if got := domain.VarianteDePlato("Ceviche Clásico", ""); got != "" {
		t.Errorf("se invento una variante: %q", got)
	}
}

// El banco tiene UNA entrada de chaufa y es la de mariscos, asi que "Fuente de
// Chaufa de Pescado" salia con langostinos y aros de calamar. Y el "de pescado"
// no puede ganarle a una variante que dice mas: un pescado a lo macho va bajo
// su salsa de mariscos, no "solo pescado".
func TestDePescadoEsPescadoYNoLePisaALoMacho(t *testing.T) {
	if got := domain.VarianteDePlato("Chaufa de Pescado", ""); !strings.Contains(got, "only with fish") {
		t.Errorf("el chaufa de pescado sigue siendo de mariscos: %q", got)
	}
	if got := domain.VarianteDePlato("Pescado a lo Macho", ""); !strings.Contains(got, "smothered") {
		t.Errorf("a lo macho perdio su salsa: %q", got)
	}
}

// En la pizarra de jugos, "Surtido" y "Especial" son dos bebidas distintas y se
// distinguen a la vista. Antes los dos caian en la variante de mariscos y salia
// un vaso con calamares dentro.
func TestLosJugosNoSeLlevanLosMariscos(t *testing.T) {
	casos := []struct {
		nombre, curso, espera, jamas string
	}{
		{"Surtido", domain.CursoBebida, "BLEND OF SEVERAL DIFFERENT FRUITS", "seafood"},
		{"Jugo Surtido", domain.CursoBebida, "beetroot", "seafood"},
		{"Especial", domain.CursoBebida, "MILKSHAKE", "seafood"},
		{"Jugo Especial", domain.CursoBebida, "algarrobina", "seafood"},

		// El mismo adjetivo en un plato sigue queriendo decir mariscos.
		{"Ceviche Mixto", domain.CursoFondo, "several different seafoods", "FRUITS"},
		{"Jalea Surtida", domain.CursoFondo, "several different seafoods", "FRUITS"},
	}

	for _, c := range casos {
		t.Run(c.nombre+"/"+c.curso, func(t *testing.T) {
			got := domain.VarianteDePlato(c.nombre, c.curso)
			if !strings.Contains(got, c.espera) {
				t.Errorf("falta %q en:\n%s", c.espera, got)
			}
			if strings.Contains(got, c.jamas) {
				t.Errorf("no tenia que decir %q en:\n%s", c.jamas, got)
			}
		})
	}
}

// El surtido es ROSA por la betarraga y el especial CREMA por la leche: si los
// dos dijeran lo mismo, volverian a ser la misma foto.
func TestElSurtidoYElEspecialNoSeParecen(t *testing.T) {
	surtido := domain.VarianteDePlato("Surtido", domain.CursoBebida)
	especial := domain.VarianteDePlato("Especial", domain.CursoBebida)

	if surtido == especial {
		t.Fatal("los dos jugos dicen lo mismo")
	}
	// Se comprueban el color afirmado Y la negacion del otro. Buscar "beige" a
	// secas no vale: los dos textos lo mencionan, uno para afirmarlo y otro para
	// negarlo, y esa es justo la parte que hace que el generador acierte.
	if !strings.Contains(surtido, "magenta") || !strings.Contains(surtido, "never beige") {
		t.Errorf("el surtido tiene que ser rosa y negar el beige:\n%s", surtido)
	}
	if !strings.Contains(especial, "CREAM COLOURED") || !strings.Contains(especial, "never pink") {
		t.Errorf("el especial tiene que ser crema y negar el rosa:\n%s", especial)
	}
}
