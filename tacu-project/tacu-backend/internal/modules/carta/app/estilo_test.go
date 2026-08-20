package app

import (
	"fmt"
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// El fallo que este test existe para atrapar no da error: da 74 fotos con el
// mantel donde iba el plato. Las imagenes viajan a Gemini como partes de entrada
// sin nombre, y lo unico que dice cual es cual es su POSICION contra el parrafo
// del prompt. Si las dos listas se escriben en sitios distintos, el dia que
// alguien anada una clase de referencia se desalinean en silencio.
func TestElOrdenDeLasImagenesEsElQueDiceElPrompt(t *testing.T) {
	casos := []struct {
		nombre string
		receta domain.Receta
		quiere []string
	}{
		{"sin nada", domain.Receta{}, nil},
		{
			"solo la vajilla",
			domain.Receta{FotoVajilla: "v.jpg"},
			[]string{"v.jpg"},
		},
		{
			"solo el fondo",
			domain.Receta{FotoFondo: "f.jpg"},
			[]string{"f.jpg"},
		},
		{
			"las dos ranuras y el plato",
			domain.Receta{
				FotoVajilla: "v.jpg",
				FotoFondo:   "f.jpg",
				Referencias: []string{"p1.jpg", "p2.jpg"},
			},
			[]string{"v.jpg", "f.jpg", "p1.jpg", "p2.jpg"},
		},
		{
			// El hueco importa: sin vajilla, el fondo pasa a ser la imagen 1 y
			// el parrafo tiene que decir 1, no 2.
			"el fondo sin vajilla es la primera",
			domain.Receta{FotoFondo: "f.jpg", Referencias: []string{"p1.jpg"}},
			[]string{"f.jpg", "p1.jpg"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			claves, dice := referenciasOrdenadas(c.receta)

			if len(claves) != len(c.quiere) {
				t.Fatalf("claves = %v, quiere %v", claves, c.quiere)
			}
			for i := range claves {
				if claves[i] != c.quiere[i] {
					t.Fatalf("clave %d = %q, quiere %q", i, claves[i], c.quiere[i])
				}
			}

			if len(claves) == 0 {
				if dice != "" {
					t.Fatalf("sin imagenes no puede haber parrafo, y hay: %q", dice)
				}
				return
			}

			// Cada posicion se nombra UNA vez, y ninguna de mas: un "reference
			// image 3" con dos imagenes le pide al modelo algo que no existe.
			for i := range claves {
				etiqueta := fmt.Sprintf("Reference image %d ", i+1)
				if n := strings.Count(dice, etiqueta); n != 1 {
					t.Fatalf("el parrafo nombra %q %d veces, quiere 1:\n%s", etiqueta, n, dice)
				}
			}
			sobra := fmt.Sprintf("Reference image %d ", len(claves)+1)
			if strings.Contains(dice, sobra) {
				t.Fatalf("el parrafo nombra una imagen que no se manda (%q):\n%s", sobra, dice)
			}
		})
	}
}

// La foto del plato del dueno solo vale donde vale su plato. Cuando el banco o
// el formato ponen el recipiente, una foto del plato individual CONTRADICE al
// texto, y entre un texto y una imagen el modelo copia la imagen: la ronda
// marina saldria en un plato redondo.
func TestLaFotoDeVajillaSeVaConElRecipienteQueLaPisa(t *testing.T) {
	base := domain.Estilo{
		Vajilla: domain.Ranura{Texto: "plato de barro", Foto: "vajilla.jpg"},
		Fondo:   domain.Ranura{Texto: "mesa de madera", Foto: "fondo.jpg"},
	}.Receta()

	t.Run("un plato normal se la queda", func(t *testing.T) {
		r := domain.Ensamblar(domain.Receta{}, base, domain.Receta{Sujeto: "one serving"}, domain.Receta{})
		if r.FotoVajilla != "vajilla.jpg" {
			t.Fatalf("FotoVajilla = %q, quiere que sobreviva", r.FotoVajilla)
		}
	})

	t.Run("una fuente la pierde", func(t *testing.T) {
		fuente := domain.Receta{Recipiente: "a large oval serving platter"}
		r := domain.Ensamblar(domain.Receta{}, base, fuente, domain.Receta{})
		if r.FotoVajilla != "" {
			t.Fatalf("FotoVajilla = %q, quiere vacia: el formato pone el recipiente", r.FotoVajilla)
		}
	})

	t.Run("el fondo sobrevive siempre", func(t *testing.T) {
		fuente := domain.Receta{Recipiente: "a large oval serving platter"}
		r := domain.Ensamblar(domain.Receta{}, base, fuente, domain.Receta{})
		if r.FotoFondo != "fondo.jpg" {
			t.Fatalf("FotoFondo = %q: ni el tipo ni el formato tocan el fondo", r.FotoFondo)
		}
	})

	t.Run("una bebida la pierde", func(t *testing.T) {
		bebida := domain.PlatoTipico{Curso: domain.CursoBebida}
		r := bebida.ConLaBaseDelDueno(base)
		if r.FotoVajilla != "" {
			t.Fatalf("FotoVajilla = %q, quiere vacia: una bebida va en su vaso", r.FotoVajilla)
		}
		if r.FotoFondo != "fondo.jpg" {
			t.Fatalf("FotoFondo = %q: el fondo del dueno vale tambien para las bebidas", r.FotoFondo)
		}
	})
}

// Las tres maneras de llenar una ranura son EXCLUYENTES y hay un orden: la foto
// del dueno gana, porque es lo mas exacto que puede darnos y ademas es gratis.
func TestLaFotoDelDuenoGanaAlDibujo(t *testing.T) {
	casos := []struct {
		nombre string
		r      domain.Ranura
		quiere string
	}{
		{"vacia usa la de por defecto", domain.Ranura{}, ""},
		{"solo dibujo", domain.Ranura{Texto: "barro", Vista: "dibujo.jpg"}, "dibujo.jpg"},
		{"la foto gana", domain.Ranura{Texto: "barro", Vista: "dibujo.jpg", Foto: "mia.jpg"}, "mia.jpg"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.r.VistaActual(); got != c.quiere {
				t.Fatalf("VistaActual = %q, quiere %q", got, c.quiere)
			}
		})
	}
}

// El plegado es por RANURA y campo por campo: una categoria puede cambiar solo
// su fondo sin perder la vajilla de la general. Y el dibujo viaja pegado a su
// texto, o la categoria enseniaria el dibujo de una descripcion que no es suya.
func TestLaCategoriaHeredaLoQueNoRedefine(t *testing.T) {
	general := domain.Estilo{
		Vajilla: domain.Ranura{Texto: "plato de barro", Vista: "barro.jpg"},
		Fondo:   domain.Ranura{Texto: "mesa oscura", Vista: "mesa.jpg"},
	}
	categoria := domain.Estilo{
		Fondo: domain.Ranura{Texto: "en la playa", Vista: "playa.jpg"},
	}

	e := categoria.Sobre(general)

	if e.Vajilla.Texto != "plato de barro" || e.Vajilla.Vista != "barro.jpg" {
		t.Fatalf("la vajilla no se hereda entera: %+v", e.Vajilla)
	}
	if e.Fondo.Texto != "en la playa" || e.Fondo.Vista != "playa.jpg" {
		t.Fatalf("el fondo propio no gana con su dibujo: %+v", e.Fondo)
	}
}

// El por defecto tiene que decir lo MISMO en la vista previa que en las fotos:
// si no, el dueno ve un plato blanco de muestra y sus 74 platos salen en otro.
func TestElPorDefectoEsElMismoEnLosDosPrompts(t *testing.T) {
	vacio := domain.Estilo{}

	if !strings.Contains(PromptDeVajilla(vacio), vajillaPorDefecto) {
		t.Fatal("la vista previa de la vajilla no usa el recipiente por defecto")
	}
	if !strings.Contains(PromptDeFondo(vacio), fondoPorDefecto) {
		t.Fatal("la vista previa del fondo no usa el fondo por defecto")
	}

	// Y lo escrito manda sobre el por defecto, en las dos ranuras.
	mio := domain.Estilo{
		Vajilla: domain.Ranura{Texto: "bandeja de madera"},
		Fondo:   domain.Ranura{Texto: "en la playa"},
	}
	if !strings.Contains(PromptDeVajilla(mio), "bandeja de madera") {
		t.Fatal("la vista previa ignora lo que escribio el dueno")
	}
	if !strings.Contains(PromptDeFondo(mio), "en la playa") {
		t.Fatal("la vista previa del fondo ignora lo que escribio el dueno")
	}
}
