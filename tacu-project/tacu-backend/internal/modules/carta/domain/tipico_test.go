package domain_test

import (
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// El banco de mentira: dos entradas del canon con descripcion y dos de la
// cosecha sin ella, que es la mezcla real de la tabla.
func banco() []domain.PlatoTipico {
	return []domain.PlatoTipico{
		{
			Clave: "ceviche", Nombre: "Ceviche", Curso: domain.CursoFondo,
			Aspecto:  "raw white fish cubes in lime marinade",
			Jamas:    "never cooked, never breaded",
			Patrones: []string{"ceviche", "cebiche"},
		},
		{
			Clave: "chicharron-de-pescado", Nombre: "Chicharron de Pescado",
			Curso: domain.CursoFondo, Aspecto: "golden fried fish chunks",
			Patrones: []string{"chicharron-de-pescado"},
		},
		{
			Clave: "chicharron", Nombre: "Chicharron", Curso: domain.CursoFondo,
			Aspecto: "fried pork", Patrones: []string{"chicharron"},
		},
		{
			Clave: "ceviche-clasico", Nombre: "Ceviche Clasico", Curso: domain.CursoFondo,
			Patrones: []string{"ceviche-clasico"},
		},
		{
			Clave: "inca-kola", Nombre: "Inca Kola", Curso: domain.CursoBebida,
			Patrones: []string{"inca-kola", "gaseosa"},
		},
		{
			Clave: "panqueques", Nombre: "Panqueques", Curso: domain.CursoPostre,
			Patrones: []string{"panqueques"},
		},
	}
}

func TestElPatronMasLargoGanaAlMasCorto(t *testing.T) {
	// Sin esto, "chicharron" se lleva el plato y sale cerdo frito en vez de
	// pescado.
	got, ok := domain.EmparejarTipico("Chicharrón de Pescado", banco())
	if !ok || got.Clave != "chicharron-de-pescado" {
		t.Fatalf("emparejo %q", got.Clave)
	}
}

// La regla que sostiene la mezcla de canon y cosecha: "ceviche-clasico" es mas
// largo, pero no sabe como se ve un ceviche.
func TestElDescritoGanaAunqueSuPatronSeaMasCorto(t *testing.T) {
	got, ok := domain.EmparejarTipico("Ceviche Clásico", banco())
	if !ok || got.Clave != "ceviche" {
		t.Fatalf("emparejo %q, esperaba el descrito", got.Clave)
	}
}

// Los slugs se comparan por tokens: sin los guiones de los extremos, "pan"
// emparejaria "panqueques" y un postre saldria como sandwich.
func TestUnPatronNoEmparejaAMitadDeUnaPalabra(t *testing.T) {
	conPan := append(banco(), domain.PlatoTipico{
		Clave: "pan-con-chicharron", Nombre: "Pan con chicharron",
		Curso: domain.CursoFondo, Aspecto: "bread roll", Patrones: []string{"pan"},
	})

	got, ok := domain.EmparejarTipico("Panqueques", conPan)
	if !ok || got.Clave != "panqueques" {
		t.Fatalf("emparejo %q", got.Clave)
	}
}

func TestUnNombreQueNoEstaEnElBancoNoEmpareja(t *testing.T) {
	if _, ok := domain.EmparejarTipico("Combo Tribuna", banco()); ok {
		t.Fatal("no deberia emparejar nada")
	}
}

func TestLasTildesYLasMayusculasNoEstorban(t *testing.T) {
	for _, n := range []string{"CEVICHE MIXTO", "cebiche de conchas", "Ceviche  a  la  Tribuna"} {
		if got, ok := domain.EmparejarTipico(n, banco()); !ok || got.Clave != "ceviche" {
			t.Errorf("%q emparejo %q", n, got.Clave)
		}
	}
}

// EL ARREGLO: una gaseosa no lleva la guarnicion de la cevicheria. Es lo que
// llego gratis con el curso, sin escribir una sola descripcion.
func TestUnaBebidaNoLlevaLaGuarnicionDeLaCocina(t *testing.T) {
	gaseosa, ok := domain.EmparejarTipico("Gaseosa 1L", banco())
	if !ok {
		t.Fatal("la gaseosa tiene que emparejar")
	}

	cevicheria := domain.Cevicheria.Receta()
	if !strings.Contains(strings.ToLower(cevicheria.Acompanamiento), "sweet potato") {
		t.Fatal("la cevicheria tendria que traer camote; cambio la receta del tipo")
	}

	r := domain.Ensamblar(cevicheria, gaseosa.Receta(), domain.Receta{}, domain.Receta{})
	if strings.Contains(strings.ToLower(r.Acompanamiento), "sweet potato") {
		t.Errorf("la gaseosa sigue saliendo con camote al lado: %q", r.Acompanamiento)
	}
	if !strings.Contains(strings.ToLower(r.Recipiente), "glass") {
		t.Errorf("una bebida va en vaso o botella, no en plato: %q", r.Recipiente)
	}
}

func TestUnaSopaVaEnBolYNoEnPlatoLlano(t *testing.T) {
	sopa := domain.PlatoTipico{Clave: "chupe", Curso: domain.CursoSopa}

	r := domain.Ensamblar(domain.Cevicheria.Receta(), sopa.Receta(),
		domain.Receta{}, domain.Receta{})
	if !strings.Contains(strings.ToLower(r.Recipiente), "bowl") {
		t.Errorf("recipiente = %q", r.Recipiente)
	}
}

// Un plato de fondo NO pisa a la cocina: una entrada de cevicheria si lleva lo
// de la cevicheria, y ese es justo el conocimiento que no hay que tirar.
func TestUnPlatoDeFondoSinDescribirDejaMandarALaCocina(t *testing.T) {
	fondo := domain.PlatoTipico{Clave: "sudado", Curso: domain.CursoFondo}

	cevicheria := domain.Cevicheria.Receta()
	r := domain.Ensamblar(cevicheria, fondo.Receta(), domain.Receta{}, domain.Receta{})
	if r.Acompanamiento != cevicheria.Acompanamiento {
		t.Errorf("un fondo sin describir no tenia que tocar la guarnicion de la cocina")
	}
}

// Lo que NO es va pegado a como se ve, igual que en el pollo a la brasa.
func TestElJamasViajaDentroDeLaIdentidad(t *testing.T) {
	r := banco()[0].Receta()
	if !strings.Contains(r.Identidad, "raw white fish") || !strings.Contains(r.Identidad, "never cooked") {
		t.Fatalf("identidad = %q", r.Identidad)
	}
}

// POR QUE la identidad tiene ranura propia: el formato se pliega ENCIMA del
// banco y escribe el sujeto. Compartiendo campo, una fuente de ceviche perderia
// que es un ceviche y saldria una bandeja de comida cualquiera.
func TestUnaFuenteDeCevicheSigueSabiendoQueEsUnCeviche(t *testing.T) {
	ceviche := banco()[0]
	fuente, ok := domain.FormatoDePlato(domain.Cevicheria, "Fuente de Ceviche", "Fuentes")
	if !ok {
		t.Fatal("una fuente de ceviche tendria que dar formato fuente")
	}

	r := domain.Ensamblar(domain.Cevicheria.Receta(), ceviche.Receta(),
		domain.Receta{}, fuente.Receta)

	if !strings.Contains(r.Identidad, "raw white fish") {
		t.Errorf("la fuente se llevo por delante que es un ceviche: %q", r.Identidad)
	}
	if !strings.Contains(strings.ToLower(r.Recipiente), "platter") {
		t.Errorf("y aun asi tiene que ir en bandeja: %q", r.Recipiente)
	}
}

// Un producto envasado es la UNICA excepcion al "no text and no logo" de la
// plantilla: el restaurante lo compra cerrado y lo revende, asi que taparle la
// etiqueta seria fotografiar una botella que no es la que le van a dar.
func TestUnProductoEnvasadoConservaSuMarca(t *testing.T) {
	gaseosa := domain.PlatoTipico{
		Clave: "gaseosa", Curso: domain.CursoBebida, Envasado: true,
		Aspecto: "a chilled bottle", Jamas: "no food",
	}
	if gaseosa.Receta().Marca == "" {
		t.Fatal("un envasado tiene que traer su marca")
	}

	// Y la comida NO: ahi la prohibicion se queda entera.
	ceviche := domain.PlatoTipico{
		Clave: "ceviche", Curso: domain.CursoFondo,
		Aspecto: "raw fish", Jamas: "never cooked",
	}
	if ceviche.Receta().Marca != "" {
		t.Error("un plato de comida no puede levantar la prohibicion de letreros")
	}
}

// EL CASO DEL CAFE: el dueno escribe "plato redondo blanco" en su estilo y esa
// preferencia se llevaba por delante el recipiente de TODO, asi que salia cafe
// vertido en un plato y gaseosa servida en la vajilla del restaurante.
//
// La vajilla es una preferencia; que una bebida vaya en vaso es un hecho.
func TestLaVajillaDelDuenoNoAlcanzaALasBebidas(t *testing.T) {
	base := domain.Receta{
		Recipiente: "a round white ceramic plate",
		Fondo:      "on a dark wooden table",
	}

	cafe := domain.PlatoTipico{
		Clave: "cafe", Curso: domain.CursoBebida,
		Aspecto: "black coffee", Recipiente: "a small white cup on its saucer",
	}
	r := cafe.ConLaBaseDelDueno(base)
	if strings.Contains(r.Recipiente, "plate") {
		t.Errorf("el cafe volvio al plato del dueno: %q", r.Recipiente)
	}
	// Y el fondo del dueno SI sobrevive: de eso el banco no dice nada.
	if r.Fondo != base.Fondo {
		t.Errorf("se perdio el fondo del dueno: %q", r.Fondo)
	}

	// Lo que va en plato si es suyo: para eso lo configuro.
	ceviche := domain.PlatoTipico{
		Clave: "ceviche", Curso: domain.CursoFondo,
		Aspecto: "raw fish", Recipiente: "a wide shallow white bowl",
	}
	if got := ceviche.ConLaBaseDelDueno(base).Recipiente; got != base.Recipiente {
		t.Errorf("el dueno tenia que mandar en un plato de fondo: %q", got)
	}
}

// Un plato que el banco no conoce se queda como antes de que el banco
// existiera: manda el dueno.
func TestSinBancoDetrasSigueMandandoElDueno(t *testing.T) {
	base := domain.Receta{Recipiente: "a wooden board"}
	if got := (domain.PlatoTipico{}).ConLaBaseDelDueno(base); got.Recipiente != base.Recipiente {
		t.Errorf("recipiente = %q", got.Recipiente)
	}
}
