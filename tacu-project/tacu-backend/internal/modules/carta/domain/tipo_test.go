package domain_test

import (
	"fmt"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// Los nombres son lo unico que mira InferirTipo.
func carta(categoria string, nombres ...string) domain.Carta {
	cat := domain.Categoria{Nombre: categoria}
	for _, n := range nombres {
		cat.Platos = append(cat.Platos, domain.Plato{Nombre: n})
	}
	return domain.Carta{Categorias: []domain.Categoria{cat}}
}

// Los nombres salen de las cartas reales: ninguna se anuncia como "cevicheria".
func TestInferirTipo(t *testing.T) {
	casos := []struct {
		nombre string
		carta  domain.Carta
		quiere domain.Tipo
	}{
		{
			"La Tribuna del Sur",
			carta("Ceviches", "Ceviche a la Tribuna", "Tiradito a la Bandera",
				"Chicharrón de Pescado", "Leche de Tigre", "Jalea Mixta"),
			domain.Cevicheria,
		},
		{
			"Galponcito",
			carta("Pollos", "1/4 pollo a la brasa", "1/2 pollo", "Pollo entero", "Mostrito"),
			domain.Polleria,
		},
		{
			"chifa de barrio",
			carta("Chifa", "Arroz Chaufa de Pollo", "Wantan Frito", "Tallarin Saltado", "Aeropuerto"),
			domain.Chifa,
		},
		{
			"criollo",
			carta("Segundos", "Lomo Saltado", "Aji de Gallina", "Seco de Res", "Cau Cau"),
			domain.Criollo,
		},
		{
			"pizzeria",
			carta("Pizzas", "Pizza Americana", "Pizza Hawaiana", "Lasagna", "Calzone"),
			domain.Pizzeria,
		},
		// Sin ninguna pista no se adivina: generico, que no inventa guarniciones.
		{
			"sin pistas",
			carta("Varios", "Plato del dia", "Especial de la casa"),
			domain.Generico,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			tipos := domain.InferirTipos(c.carta)
			if len(tipos) == 0 || tipos[0] != c.quiere {
				t.Errorf("InferirTipos dio %v, quiero %q el primero", tipos, c.quiere)
			}
		})
	}
}

// Iterar un mapa es aleatorio: sin orden fijo la misma carta daria cevicheria
// una vez y chifa la siguiente.
func TestInferirTiposEsDeterminista(t *testing.T) {
	// Una cevicheria que tambien vende chaufa, que es lo normal en Lima.
	c := carta("Carta", "Ceviche Mixto", "Tiradito", "Arroz Chaufa de Mariscos", "Jalea")

	primero := fmt.Sprint(domain.InferirTipos(c))
	for range 50 {
		if otro := fmt.Sprint(domain.InferirTipos(c)); otro != primero {
			t.Fatalf("la misma carta dio %v y despues %v", primero, otro)
		}
	}
}

// El caso que obligo a que los tipos fueran varios: la cevicheria del barrio que
// tambien vende pollo a la brasa. Con un solo tipo, media carta salia emplatada
// como no es.
func TestUnNegocioDeDosCocinasProponeLasDos(t *testing.T) {
	c := domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "Ceviches", Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado"}, {Nombre: "Ceviche Mixto"}, {Nombre: "Tiradito"},
		}},
		{Nombre: "Brasas", Platos: []domain.Plato{
			{Nombre: "1/4 pollo a la brasa"}, {Nombre: "1/2 pollo a la brasa"}, {Nombre: "Pollo entero"},
		}},
	}}

	tipos := domain.InferirTipos(c)
	tiene := func(q domain.Tipo) bool {
		for _, tipo := range tipos {
			if tipo == q {
				return true
			}
		}
		return false
	}
	if !tiene(domain.Cevicheria) || !tiene(domain.Polleria) {
		t.Fatalf("tipos = %v, esperaba cevicheria y polleria", tipos)
	}
}

// Un ceviche suelto en la carta de una polleria no convierte al local en
// cevicheria: el corte es un tercio del ganador.
func TestUnPlatoSueltoNoAnadeUnTipo(t *testing.T) {
	c := carta("Carta", "1/4 pollo a la brasa", "1/2 pollo a la brasa", "Pollo entero",
		"Mostrito", "Salchipapa", "Broaster", "Ceviche de Pescado")

	tipos := domain.InferirTipos(c)
	for _, tipo := range tipos {
		if tipo == domain.Cevicheria {
			t.Fatalf("tipos = %v: un ceviche no hace una cevicheria", tipos)
		}
	}
}

// Los varios tipos solo significan algo si cada plato usa el suyo: el ceviche
// con camote y choclo, el pollo con sus cuatro salsas.
func TestCadaPlatoUsaElTipoQueLeToca(t *testing.T) {
	negocio := []domain.Tipo{domain.Cevicheria, domain.Polleria}

	casos := map[string]struct {
		nombre, categoria string
		quiere            domain.Tipo
	}{
		"el ceviche manda cevicheria": {"Ceviche Mixto", "Ceviches", domain.Cevicheria},
		"el pollo manda polleria":     {"1/4 pollo a la brasa", "Brasas", domain.Polleria},
		"lo que no se parece a nada":  {"Gaseosa 1L", "Bebidas", domain.Cevicheria},
		"la categoria tambien cuenta": {"Familiar", "Pollo a la brasa", domain.Polleria},
	}

	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			if got := domain.TipoDePlato(negocio, c.nombre, c.categoria); got != c.quiere {
				t.Errorf("TipoDePlato dio %q, quiero %q", got, c.quiere)
			}
		})
	}
}

func TestTiposValidosLimpiaLoQueLlegaDeFuera(t *testing.T) {
	tipos := domain.TiposValidos([]string{"cevicheria", "no-existe", "cevicheria", "polleria"})
	if len(tipos) != 2 || tipos[0] != domain.Cevicheria || tipos[1] != domain.Polleria {
		t.Fatalf("tipos = %v", tipos)
	}
}

// Todo tipo valido trae su receta y sus formatos.
func TestCadaTipoConocidoTraeSuReceta(t *testing.T) {
	for _, tipo := range domain.TiposConocidos {
		if !tipo.Valido() {
			t.Errorf("%q esta en TiposConocidos y no es valido", tipo)
		}
		if tipo.Nombre() == "" {
			t.Errorf("%q no tiene nombre para la pantalla", tipo)
		}
		if tipo.Receta().Recipiente == "" {
			t.Errorf("%q no dice en que recipiente sirve", tipo)
		}
		// Vender en tamano familiar es universal.
		if !tipo.PermiteFormato("fuente") {
			t.Errorf("%q deberia permitir el formato fuente", tipo)
		}
	}

	// Trio y ronda son solo de cevicheria.
	for _, tipo := range domain.TiposConocidos {
		if tipo == domain.Cevicheria {
			continue
		}
		for _, f := range []string{"trio", "ronda"} {
			if tipo.PermiteFormato(f) {
				t.Errorf("%q no deberia conocer el formato %q", tipo, f)
			}
		}
	}
}

// Un tipo desconocido cae a generico en vez de tumbar la generacion.
func TestUnTipoDesconocidoCaeAGenerico(t *testing.T) {
	raro := domain.Tipo("marisqueria-fusion")
	if raro.Valido() {
		t.Fatal("ese tipo no deberia ser valido")
	}
	if raro.Receta().Recipiente == "" {
		t.Error("un tipo desconocido tiene que caer a generico, no quedarse sin recipiente")
	}
}

// --- reparto ---

// La carta que motivo todo esto: una cevicheria que tambien vende pollo a la
// brasa, con solo cevicheria marcada. Los pollos no se parecen a cevicheria, asi
// que salen con la guarnicion de la casa equivocada, y eso hay que DECIRLO.
func cartaDeDosCocinas() domain.Carta {
	return domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "Ceviches", Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado"},
			{Nombre: "Ceviche Mixto"},
			{Nombre: "Tiradito"},
			{Nombre: "Jalea Mixta"},
		}},
		{Nombre: "Brasas", Platos: []domain.Plato{
			{Nombre: "1/4 pollo a la brasa"},
			{Nombre: "1/2 pollo a la brasa"},
			{Nombre: "Pollo entero"},
			{Nombre: "Mostrito"},
		}},
		{Nombre: "Bebidas", Platos: []domain.Plato{
			{Nombre: "Gaseosa 1L"},
			{Nombre: "Chicha morada"},
		}},
	}}
}

func TestElRepartoDiceQueTipoSeEstaQuedandoFuera(t *testing.T) {
	r := domain.RepartirCarta(cartaDeDosCocinas(), []domain.Tipo{domain.Cevicheria})

	if r.Total != 10 {
		t.Fatalf("total = %d, esperaba 10", r.Total)
	}
	if len(r.PorTipo) != 1 || r.PorTipo[0].Tipo != domain.Cevicheria || r.PorTipo[0].Platos != 4 {
		t.Fatalf("porTipo = %+v, esperaba 4 ceviches", r.PorTipo)
	}
	// Los 4 pollos y las 2 bebidas no se parecen a cevicheria: los toma el
	// primero, que es justo lo que el dueno tiene que saber.
	if r.SinPistas != 6 {
		t.Errorf("sinPistas = %d, esperaba 6", r.SinPistas)
	}
	if len(r.Faltan) != 1 || r.Faltan[0].Tipo != domain.Polleria || r.Faltan[0].Platos != 4 {
		t.Fatalf("faltan = %+v, esperaba polleria con 4", r.Faltan)
	}
}

// Con los dos elegidos ya no falta nadie y cada plato va a su sitio.
func TestConLosDosTiposElegidosNoFaltaNinguno(t *testing.T) {
	r := domain.RepartirCarta(cartaDeDosCocinas(), []domain.Tipo{domain.Cevicheria, domain.Polleria})

	if len(r.Faltan) != 0 {
		t.Fatalf("faltan = %+v, no deberia faltar ninguno", r.Faltan)
	}
	conteo := map[domain.Tipo]int{}
	for _, c := range r.PorTipo {
		conteo[c.Tipo] = c.Platos
	}
	if conteo[domain.Cevicheria] != 4 || conteo[domain.Polleria] != 4 {
		t.Fatalf("porTipo = %+v", r.PorTipo)
	}
	// Las bebidas siguen sin parecerse a nada: las toma el primero.
	if r.SinPistas != 2 {
		t.Errorf("sinPistas = %d, esperaba las 2 bebidas", r.SinPistas)
	}
}

// Un plato suelto no justifica avisar de un tipo entero.
func TestUnPlatoSueltoNoSaleComoTipoQueFalta(t *testing.T) {
	c := domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "Ceviches", Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado"}, {Nombre: "Ceviche Mixto"}, {Nombre: "Tiradito"},
			{Nombre: "Jalea"}, {Nombre: "Chicharron de Pescado"}, {Nombre: "Arroz Chaufa de Mariscos"},
		}},
		{Nombre: "Otros", Platos: []domain.Plato{{Nombre: "Lomo Saltado"}}},
	}}

	r := domain.RepartirCarta(c, []domain.Tipo{domain.Cevicheria})
	for _, f := range r.Faltan {
		if f.Tipo == domain.Criollo {
			t.Fatalf("faltan = %+v: un lomo saltado no justifica avisar de criolla", r.Faltan)
		}
	}
}

// Sin ninguno elegido, ninguna foto lleva guarnicion de la casa: todo cae en
// "no se parece a ninguno".
func TestSinTiposElegidosTodoCaeEnSinPistas(t *testing.T) {
	r := domain.RepartirCarta(cartaDeDosCocinas(), nil)

	if r.SinPistas != r.Total || len(r.PorTipo) != 0 {
		t.Fatalf("reparto = %+v", r)
	}
	if len(r.Faltan) == 0 {
		t.Error("con nada elegido, los dos tipos de la carta tendrian que salir como que faltan")
	}
}

// El reparto usa las MISMAS pistas que deciden la foto: si divergieran, la
// pantalla diria una cosa y el generador haria otra.
func TestElRepartoCuadraConTipoDePlato(t *testing.T) {
	elegidos := []domain.Tipo{domain.Cevicheria, domain.Polleria}
	c := cartaDeDosCocinas()

	conteo := map[domain.Tipo]int{}
	for _, cat := range c.Categorias {
		for _, p := range cat.Platos {
			conteo[domain.TipoDePlato(elegidos, p.Nombre, cat.Nombre)]++
		}
	}

	r := domain.RepartirCarta(c, elegidos)
	for _, c := range r.PorTipo {
		esperado := conteo[c.Tipo]
		if c.Tipo == elegidos[0] {
			// Al primero le caen ademas los que no se parecen a ninguno.
			esperado -= r.SinPistas
		}
		if c.Platos != esperado {
			t.Errorf("%s: reparto dice %d y TipoDePlato %d", c.Tipo, c.Platos, esperado)
		}
	}
}
