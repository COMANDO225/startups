package app

import (
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

func bancoDePrueba() []domain.PlatoTipico {
	return []domain.PlatoTipico{
		{
			Clave: "ceviche", Nombre: "Ceviche", Curso: domain.CursoFondo,
			Aspecto: "raw white fish cubes", Jamas: "never cooked",
			Patrones: []string{"ceviche"},
		},
		{
			// Cosechada: tiene ingredientes y nadie la ha descrito. Es el caso de
			// 413 de las 437 filas del banco.
			Clave: "patarashca", Nombre: "Patarashca", Curso: domain.CursoFondo,
			Patrones:     []string{"patarashca"},
			Ingredientes: []string{"Pescado de rio", "Hoja de bijao", "Sacha culantro"},
		},
	}
}

func cartaCon(nombres ...string) domain.Carta {
	var cat domain.Categoria
	cat.Nombre = "Carta"
	for _, n := range nombres {
		cat.Platos = append(cat.Platos, domain.Plato{Nombre: n})
	}
	return domain.Carta{Categorias: []domain.Categoria{cat}}
}

// loQueFalta es lo que decide si se gasta la llamada y que se pregunta. Los dos
// huecos son distintos: uno no empareja con nada, el otro empareja con una fila
// que nadie describio.
func TestLoQueFaltaSeparaLoDesconocidoDeLoQueFaltaDescribir(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Ceviche de Pescado", "Patarashca de Paiche", "Combo Tribuna")
	c.EmparejarConElBanco(banco)

	desconocidos, aMedias := loQueFalta(c, banco)

	if len(desconocidos) != 1 || desconocidos[0].Nombre != "Combo Tribuna" {
		t.Errorf("desconocidos = %+v", desconocidos)
	}
	// La seccion viaja con el nombre: es el contexto que evita que "Surtido" en
	// la seccion de jugos acabe siendo un ceviche.
	if desconocidos[0].Seccion != "Carta" {
		t.Errorf("el nombre viajo sin su seccion: %+v", desconocidos[0])
	}
	if len(aMedias) != 1 || aMedias[0].Clave != "patarashca" {
		t.Fatalf("a medias = %+v", aMedias)
	}
	// El ceviche ya esta descrito: preguntar por el seria pagar por lo que ya
	// tenemos escrito y medido a mano.
	for _, p := range aMedias {
		if p.Clave == "ceviche" {
			t.Error("no hay que volver a preguntar por lo que ya esta descrito")
		}
	}
}

// Los ingredientes de la cosecha son el CONTEXTO: sin ellos, "Patarashca" es un
// nombre y el modelo tiene que adivinar.
func TestLoQueFaltaDescribirViajaConSusIngredientes(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Patarashca de Paiche")
	c.EmparejarConElBanco(banco)

	_, aMedias := loQueFalta(c, banco)
	texto := porDescribir(aMedias)

	for _, quiero := range []string{"patarashca", "Hoja de bijao", "Sacha culantro"} {
		if !strings.Contains(texto, quiero) {
			t.Errorf("el prompt no lleva %q:\n%s", quiero, texto)
		}
	}
	if !strings.Contains(texto, "no inventes una nueva") {
		t.Error("hay que decirle que devuelva la clave tal cual, o creara un duplicado")
	}
}

// Sin huecos no se gasta una llamada.
func TestSinHuecosNoSePreguntaNada(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Ceviche Mixto")
	c.EmparejarConElBanco(banco)

	desconocidos, aMedias := loQueFalta(c, banco)
	if len(desconocidos) != 0 || len(aMedias) != 0 {
		t.Fatalf("desconocidos = %v, a medias = %+v", desconocidos, aMedias)
	}
}

// El schema valida la FORMA y no los valores: lo que no sirve para una foto no
// puede entrar al banco y ocupar una clave.
func TestNoEntraAlBancoLoQueNoSirveParaUnaFoto(t *testing.T) {
	completa := domain.PlatoTipico{
		Clave: "x", Nombre: "X", Curso: domain.CursoFondo,
		Aspecto: "algo", Jamas: "no es otra cosa", Patrones: []string{"x"},
	}
	if !entradaUsable(completa) {
		t.Fatal("esta entrada si sirve")
	}

	casos := map[string]func(*domain.PlatoTipico){
		"sin aspecto":  func(p *domain.PlatoTipico) { p.Aspecto = "" },
		"sin jamas":    func(p *domain.PlatoTipico) { p.Jamas = "" },
		"sin patrones": func(p *domain.PlatoTipico) { p.Patrones = nil },
		"sin nombre":   func(p *domain.PlatoTipico) { p.Nombre = "" },
		"curso raro":   func(p *domain.PlatoTipico) { p.Curso = "plato-fuerte" },
	}
	for nombre, romper := range casos {
		rota := completa
		romper(&rota)
		if entradaUsable(rota) {
			t.Errorf("%s: entro al banco y no deberia", nombre)
		}
	}
}

// La clave la normaliza Go, nunca el modelo: sin esto "Ceviche Mixto" y
// "ceviche-mixto" serian dos entradas distintas del mismo plato.
func TestLaClavePropiaSiempreEsUnPatron(t *testing.T) {
	p := patronesLimpios([]string{"Combo Tribuna", "combo tribuna", ""}, "combo-tribuna")
	if len(p) != 1 || p[0] != "combo-tribuna" {
		t.Fatalf("patrones = %v", p)
	}
}

// Las dos guardas que salieron de la primera corrida contra La Tribuna: el
// modelo creo "ceviche-mixto" teniendo "ceviche" delante, y le colgo el patron
// "surtido" — que en esa carta es un JUGO.
func TestNoSeCreaUnDuplicadoDeAlgoYaDescrito(t *testing.T) {
	banco := bancoDePrueba()
	preguntados := []string{domain.Slug("Ceviche Mixto")}

	quedan := patronesDeFiar([]string{"ceviche-mixto", "ceviche"}, banco, preguntados)
	if len(quedan) != 0 {
		t.Fatalf("patrones = %v: el canon ya cubre esos nombres", quedan)
	}
}

func TestUnPatronQueNadiePreguntoNoEntra(t *testing.T) {
	banco := bancoDePrueba()
	preguntados := []string{domain.Slug("Combo Tribuna")}

	quedan := patronesDeFiar([]string{"combo-tribuna", "surtido"}, banco, preguntados)
	if len(quedan) != 1 || quedan[0] != "combo-tribuna" {
		t.Fatalf("patrones = %v: solo vale lo que aparece en un nombre preguntado", quedan)
	}
}

// El tope existe porque el banco lo comparten todos los restaurantes: una carta
// con veinte "Promocion N" no puede meter veinte entradas inventadas.
func TestElTopePorCartaEsRazonable(t *testing.T) {
	if maxAprendidosPorCarta < 3 || maxAprendidosPorCarta > 20 {
		t.Fatalf("tope = %d", maxAprendidosPorCarta)
	}
}

// Cuando el modelo dice "Combo Tribuna es un ceviche", eso no lo descubre ningun
// patron: es la mitad del trabajo que antes se tiraba.
func TestElEnlaceDelModeloLlegaAlPlato(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Combo Tribuna", "Ceviche de Pescado")
	c.EmparejarConElBanco(banco)

	n := aplicarEnlaces(&c, map[string]string{"combo-tribuna": "ceviche"}, banco)
	if n != 1 {
		t.Fatalf("enlazo %d", n)
	}
	platos := c.Platos()
	if platos[0].Tipico != "ceviche" || platos[0].TipicoOrigen != domain.TipicoPorIA {
		t.Errorf("el combo quedo %+v", platos[0])
	}
}

// Una opinion del modelo no pisa un patron del canon, que es determinista y esta
// medido.
func TestElEnlaceNoPisaLoQueYaEmparejo(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Ceviche de Pescado")
	c.EmparejarConElBanco(banco)

	aplicarEnlaces(&c, map[string]string{"ceviche-de-pescado": "patarashca"}, banco)
	if got := c.Platos()[0].Tipico; got != "ceviche" {
		t.Errorf("tipico = %q", got)
	}
}

// Ni pisa lo que el dueno corrigio a mano.
func TestElEnlaceNoPisaLaCorreccionDelDueno(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Combo Tribuna")
	c.Categorias[0].Platos[0].TipicoOrigen = domain.TipicoDelDueno

	aplicarEnlaces(&c, map[string]string{"combo-tribuna": "ceviche"}, banco)
	if got := c.Platos()[0].Tipico; got != "" {
		t.Errorf("tipico = %q", got)
	}
}

// Una clave que no existe en el banco no se pega al plato: dejaria un plato
// apuntando a nada y su foto sin identidad.
func TestUnEnlaceAUnaClaveInventadaSeIgnora(t *testing.T) {
	banco := bancoDePrueba()
	c := cartaCon("Combo Tribuna")

	if n := aplicarEnlaces(&c, map[string]string{"combo-tribuna": "no-existe"}, banco); n != 0 {
		t.Fatalf("enlazo %d", n)
	}
}
