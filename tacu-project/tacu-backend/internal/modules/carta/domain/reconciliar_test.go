package domain

import (
	"testing"

	"tacu-backend/internal/kernel/id"
)

func cartaDe(categoria string, nombres ...string) Carta {
	platos := make([]Plato, 0, len(nombres))
	for _, n := range nombres {
		platos = append(platos, Plato{Nombre: n})
	}
	return Carta{Categorias: []Categoria{{Nombre: categoria, Platos: platos}}}
}

// LO QUE ESTO PROTEGE: la foto cuelga del id del plato. Guardar borraba todos
// los platos y los reinsertaba con ids nuevos, asi que releer una carta de 74
// platos tiraba $2.50 en fotos ya pagadas.
func TestElPlatoQueSigueEnLaCartaConservaSuID(t *testing.T) {
	ceviche := id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "Ceviche Mixto", "Tiradito"),
		[]PlatoExistente{{ID: ceviche, Nombre: "Ceviche Mixto", Hoja: "h1"}},
		[]string{"h1"},
	)

	if len(r.Actualizar) != 1 || r.Actualizar[0].ID != ceviche {
		t.Fatalf("actualizar = %+v", r.Actualizar)
	}
	if len(r.Insertar) != 1 || r.Insertar[0].Nombre != "Tiradito" {
		t.Fatalf("insertar = %+v", r.Insertar)
	}
	if len(r.Ausentes) != 0 {
		t.Errorf("ausentes = %v", r.Ausentes)
	}
}

// El nombre se compara normalizado y SIN la categoria: OrganizarCarta renombra
// las secciones en cada lectura, y con la categoria dentro de la clave un
// "CEVICHES" que pasa a "Ceviches" convertiria la carta entera en platos nuevos
// y sin foto.
func TestRenombrarLaSeccionNoConvierteLosPlatosEnNuevos(t *testing.T) {
	ceviche := id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "CEVICHE MIXTO"),
		[]PlatoExistente{{ID: ceviche, Nombre: "Ceviche Mixto", Hoja: "h1"}},
		[]string{"h1"},
	)
	if len(r.Insertar) != 0 || len(r.Actualizar) != 1 {
		t.Fatalf("insertar=%+v actualizar=%+v", r.Insertar, r.Actualizar)
	}
	// Y la categoria nueva SI viaja: es lo impreso, manda la lectura.
	if r.Actualizar[0].Categoria != "Ceviches" {
		t.Errorf("categoria = %q", r.Actualizar[0].Categoria)
	}
}

// No se borra nada por nuestra cuenta: un plato que el dueno ya fotografio no
// puede desaparecer porque una lectura no lo trajera.
func TestElPlatoQueYaNoAparaceSeMarcaPeroNoSeBorra(t *testing.T) {
	viejo := id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "Tiradito"),
		[]PlatoExistente{{ID: viejo, Nombre: "Ceviche Mixto", Hoja: "h1"}},
		[]string{"h1"},
	)
	if len(r.Ausentes) != 1 || r.Ausentes[0] != viejo {
		t.Fatalf("ausentes = %v", r.Ausentes)
	}
}

// EL CASO QUE CIERRA EL FLUJO: el dueno quito la hoja 2 y eligio QUEDARSE con
// sus platos. La lectura siguiente no mira esa hoja, asi que no puede decir que
// esos platos desaparecieron. Sin esto volvian marcados como ausentes treinta
// segundos despues de que el dijera que se quedaban.
func TestLosPlatosDeUnaHojaQuitadaNoSeMarcanAusentes(t *testing.T) {
	deLaUno, deLaDos := id.Nuevo(), id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "Tiradito"),
		[]PlatoExistente{
			{ID: deLaUno, Nombre: "Ceviche Mixto", Hoja: "cartas/x/1.jpg"},
			{ID: deLaDos, Nombre: "Chicharrón", Hoja: "cartas/x/2.jpg"},
		},
		// La 2 ya no esta en la carta: el dueno la quito y mantuvo sus platos.
		[]string{"cartas/x/1.jpg"},
	)
	if len(r.Ausentes) != 1 || r.Ausentes[0] != deLaUno {
		t.Fatalf("ausentes = %v, solo el de la hoja que si se leyo", r.Ausentes)
	}
}

// Un plato sin hoja atribuida tampoco se marca: si no sabemos de donde salio,
// ninguna lectura lo desmiente.
func TestUnPlatoSinHojaNoSeMarcaAusente(t *testing.T) {
	suelto := id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "Tiradito"),
		[]PlatoExistente{{ID: suelto, Nombre: "Ceviche Mixto", Hoja: "h1"}},
		[]string{"cartas/x/1.jpg"},
	)
	if len(r.Ausentes) != 0 {
		t.Fatalf("ausentes = %v", r.Ausentes)
	}
}

// El cruce mira TODOS los platos, tengan la hoja que tengan: si no, un plato
// que la atribucion colgo de la hoja 1 se duplicaria al aparecer en la 2.
func TestElCruceMiraTodasLasHojas(t *testing.T) {
	deLaUno := id.Nuevo()
	r := Reconciliar(
		cartaDe("Ceviches", "Ceviche Mixto"),
		[]PlatoExistente{{ID: deLaUno, Nombre: "Ceviche Mixto", Hoja: "cartas/x/1.jpg"}},
		[]string{"cartas/x/1.jpg", "cartas/x/2.jpg"},
	)
	if len(r.Insertar) != 0 {
		t.Fatalf("duplico el plato: %+v", r.Insertar)
	}
	if len(r.Actualizar) != 1 || r.Actualizar[0].ID != deLaUno {
		t.Fatalf("actualizar = %+v", r.Actualizar)
	}
}

// Dos secciones con el mismo nombre de plato: sin la cola, las dos se llevarian
// el mismo id y una acabaria pisando a la otra.
func TestDosPlatosConElMismoNombreNoSeLlevanElMismoID(t *testing.T) {
	a, b := id.Nuevo(), id.Nuevo()
	leida := Carta{Categorias: []Categoria{
		{Nombre: "Entradas", Platos: []Plato{{Nombre: "Chicharrón"}}},
		{Nombre: "Fondos", Platos: []Plato{{Nombre: "Chicharrón"}}},
	}}

	r := Reconciliar(leida, []PlatoExistente{
		{ID: a, Nombre: "Chicharrón", Hoja: "h1"},
		{ID: b, Nombre: "Chicharrón", Hoja: "h1"},
	}, []string{"h1"})

	if len(r.Actualizar) != 2 {
		t.Fatalf("actualizar = %+v", r.Actualizar)
	}
	if r.Actualizar[0].ID == r.Actualizar[1].ID {
		t.Fatal("los dos se llevaron el mismo id")
	}
	if len(r.Ausentes) != 0 {
		t.Errorf("ausentes = %v", r.Ausentes)
	}
}

// La primera lectura: no hay nada guardado, todo es nuevo y nada esta ausente.
func TestLaPrimeraLecturaInsertaTodo(t *testing.T) {
	r := Reconciliar(cartaDe("Ceviches", "Ceviche", "Tiradito"), nil, []string{"h1"})
	if len(r.Insertar) != 2 || len(r.Actualizar) != 0 || len(r.Ausentes) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestLaHojaQueDiceElModeloSeTraduceALaClaveReal(t *testing.T) {
	c := Carta{Categorias: []Categoria{{Nombre: "Ceviches", Platos: []Plato{
		{Nombre: "Ceviche", HojaLeida: 1},
		{Nombre: "Chicharrón", HojaLeida: 2},
	}}}}

	AtribuirHojas(&c, []string{"cartas/x/1.jpg", "cartas/x/2.jpg"})

	if got := c.Categorias[0].Platos[0].Hoja; got != "cartas/x/1.jpg" {
		t.Errorf("hoja = %q", got)
	}
	if got := c.Categorias[0].Platos[1].Hoja; got != "cartas/x/2.jpg" {
		t.Errorf("hoja = %q", got)
	}
}

// Un numero de hoja inventado deja el plato SIN hoja, nunca colgado de una
// equivocada: con una atribucion falsa, quitar una hoja borraria platos de otra.
func TestUnNumeroDeHojaQueNoExisteDejaElPlatoSinHoja(t *testing.T) {
	c := Carta{Categorias: []Categoria{{Nombre: "Ceviches", Platos: []Plato{
		{Nombre: "Ceviche", HojaLeida: 5},
		{Nombre: "Tiradito", HojaLeida: 0},
	}}}}

	AtribuirHojas(&c, []string{"cartas/x/1.jpg"})

	for _, p := range c.Categorias[0].Platos {
		if p.Hoja != "" {
			t.Errorf("%q se colgo de %q", p.Nombre, p.Hoja)
		}
	}
}

// El cliente no puede ver un plato que el restaurante ya quito de su carta: es
// la misma clase de error que publicar un precio viejo.
func TestLaCartaPublicaNoLlevaLosAusentes(t *testing.T) {
	c := Carta{Categorias: []Categoria{
		{Nombre: "Ceviches", Platos: []Plato{
			{Nombre: "Ceviche"},
			{Nombre: "Tiradito", Ausente: true},
		}},
		{Nombre: "Postres", Platos: []Plato{{Nombre: "Suspiro", Ausente: true}}},
	}}

	limpia := c.SinAusentes()
	if len(limpia.Categorias) != 1 {
		t.Fatalf("la seccion que se queda vacia tiene que irse: %+v", limpia.Categorias)
	}
	if len(limpia.Categorias[0].Platos) != 1 || limpia.Categorias[0].Platos[0].Nombre != "Ceviche" {
		t.Fatalf("platos = %+v", limpia.Categorias[0].Platos)
	}
	// Y la del dueno se queda intacta: ahi es donde decide.
	if len(c.Categorias[0].Platos) != 2 {
		t.Error("SinAusentes no puede tocar la carta original")
	}
}
