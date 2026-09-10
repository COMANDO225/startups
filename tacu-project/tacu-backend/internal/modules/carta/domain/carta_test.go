package domain

import "testing"

func plato(nombre, texto string, centimos int64) Plato {
	return Plato{Nombre: nombre, Precios: []Precio{
		{Texto: texto, Centimos: dineroDe(centimos)},
	}}
}

// conPrecios arma un plato con varias formas de pedirlo. La etiqueta vacia es el
// caso real: la carta pone dos montos y no dice de que son.
func conPrecios(nombre string, precios ...Precio) Plato {
	return Plato{Nombre: nombre, Precios: precios}
}

func pr(etiqueta, texto string, centimos int64) Precio {
	return Precio{Etiqueta: etiqueta, Texto: texto, Centimos: dineroDe(centimos)}
}

func cartaCon(platos ...Plato) *Carta {
	return &Carta{Categorias: []Categoria{{Nombre: "Segundos", Platos: platos}}}
}

// El caso que motiva todo el diseno: el modelo devuelve un JSON perfecto, que
// valida contra el schema, con un precio que NO es el que esta impreso. Sin la
// verificacion cruzada eso se publica y nadie se entera.
func TestPrecioQueElModeloInventoSeMarca(t *testing.T) {
	c := cartaCon(
		plato("Ceviche mixto", "S/ 45.00", 4500), // coincide
		plato("Lomo saltado", "S/ 32.00", 5200),  // el modelo "corrigio" 32 -> 52
	)

	if m := c.Verificar(); m.Revisar != 1 {
		t.Fatalf("marco %d platos, esperaba 1", m.Revisar)
	}

	if c.Categorias[0].Platos[0].NecesitaRevision() {
		t.Error("marco un plato cuyo precio SI coincide")
	}

	malo := c.Categorias[0].Platos[1]
	if malo.Revisar != PrecioDiscordante {
		t.Fatalf("motivo = %q, esperaba precio_discordante", malo.Revisar)
	}
}

func TestMotivosDeRevision(t *testing.T) {
	casos := []struct {
		nombre   string
		p        Plato
		esperado MotivoRevision
	}{
		{"todo bien", plato("Ceviche", "S/ 45.00", 4500), SinRevision},
		{"sin simbolo, coincide", plato("Ceviche", "45", 4500), SinRevision},
		{"coma decimal, coincide", plato("Ceviche", "45,00", 4500), SinRevision},

		{"precio discordante", plato("Ceviche", "S/ 45.00", 5400), PrecioDiscordante},
		{"texto ambiguo", plato("Menu", "12 a 15", 1200), PrecioIlegible},
		{"rango de precios", plato("Parrilla", "desde 35", 3500), PrecioIlegible},
		{"sin precio impreso ni numero", plato("Pescado", "", 0), PrecioAusente},
		{"sin precio impreso pero con numero", plato("Pescado", "", 3000), PrecioDiscordante},
		{"sin nombre", plato("", "S/ 20.00", 2000), NombreVacio},
		{"nombre en blanco", plato("   ", "S/ 20.00", 2000), NombreVacio},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			c := cartaCon(caso.p)
			c.Verificar()

			got := c.Categorias[0].Platos[0].Revisar
			if got != caso.esperado {
				t.Fatalf("motivo = %q, esperaba %q", got, caso.esperado)
			}
		})
	}
}

// Un plato marcado tiene que poder explicarse al dueno en la pantalla.
func TestTodoMotivoSeExplica(t *testing.T) {
	motivos := []MotivoRevision{PrecioIlegible, PrecioDiscordante, PrecioAusente,
		NombreVacio, PrecioManuscrito, VariantesSinNombre}
	for _, m := range motivos {
		if m.Explicacion() == "" {
			t.Errorf("el motivo %q no tiene explicacion para el dueno", m)
		}
	}
	if SinRevision.Explicacion() != "" {
		t.Error("SinRevision no deberia tener explicacion")
	}
}

func TestParaRevisarSoloDevuelveLosMarcados(t *testing.T) {
	c := &Carta{Categorias: []Categoria{
		{Nombre: "Entradas", Platos: []Plato{
			plato("Causa", "S/ 18.00", 1800),
			plato("Papa a la huancaina", "S/ 15.00", 9900),
		}},
		{Nombre: "Segundos", Platos: []Plato{
			plato("Arroz con pollo", "S/ 22.00", 2200),
			plato("", "S/ 30.00", 3000),
		}},
	}}

	if m := c.Verificar(); m.Revisar != 2 {
		t.Fatalf("marco %d, esperaba 2", m.Revisar)
	}
	if n := len(c.Platos()); n != 4 {
		t.Fatalf("Platos() devolvio %d, esperaba 4", n)
	}
	if n := len(c.ParaRevisar()); n != 2 {
		t.Fatalf("ParaRevisar() devolvio %d, esperaba 2", n)
	}
}

// Verificar debe poder correrse dos veces sin acumular marcas fantasma.
func TestVerificarEsIdempotente(t *testing.T) {
	c := cartaCon(plato("Ceviche", "S/ 45.00", 4500))

	if n := c.Verificar().Total(); n != 0 {
		t.Fatalf("primera pasada marco %d", n)
	}
	if n := c.Verificar().Total(); n != 0 {
		t.Fatalf("segunda pasada marco %d: no es idempotente", n)
	}
}

// El caso de la cevicheria: la carta escribe UNA fila con dos montos y sin
// encabezado que diga de que es cada uno.
//
//	Ceviche + Arroz c/ Mariscos + Chicharron Mixto    S/ 45   S/ 80
//
// Los dos precios cuadran perfecto por separado, asi que el cruce de precios no
// ve nada. El defecto esta ENTRE los dos, y publicado le deja al cliente dos
// montos sin forma de elegir.
func TestVariosPreciosSinEtiquetaVanARevision(t *testing.T) {
	c := cartaCon(conPrecios("Ceviche + Arroz c/ Mariscos",
		pr("", "S/ 45", 4500),
		pr("", "S/ 80", 8000),
	))

	if n := c.Verificar().Total(); n != 1 {
		t.Fatalf("marco %d, esperaba 1", n)
	}
	if m := c.Categorias[0].Platos[0].Revisar; m != VariantesSinNombre {
		t.Fatalf("motivo = %q, esperaba variantes_sin_nombre", m)
	}
}

// Con la etiqueta impresa no hay nada que preguntar: la carta ya lo dijo.
func TestVariosPreciosConEtiquetaNoMolestan(t *testing.T) {
	c := cartaCon(conPrecios("Limonada",
		pr("vaso", "S/ 8", 800),
		pr("jarra", "S/ 15", 1500),
	))

	if n := c.Verificar().Total(); n != 0 {
		t.Fatalf("marco %d, esperaba 0: la carta dice de que es cada precio", n)
	}
}

// Un precio malo dentro de una variante no se puede perder entre los buenos.
func TestUnaVarianteMalaMarcaElPlato(t *testing.T) {
	c := cartaCon(conPrecios("Chicharron",
		pr("personal", "S/ 30", 3000),
		pr("fuente", "S/ 80", 9900), // el modelo se invento el numero
	))

	if n := c.Verificar().Total(); n != 1 {
		t.Fatalf("marco %d, esperaba 1", n)
	}
	if m := c.Categorias[0].Platos[0].Revisar; m != PrecioDiscordante {
		t.Fatalf("motivo = %q, esperaba precio_discordante", m)
	}
}

// Lo grave gana: un precio inventado importa mas que uno manuscrito por
// confirmar, y es lo que tiene que ver el dueno.
func TestGanaElMotivoMasGrave(t *testing.T) {
	c := cartaCon(Plato{Nombre: "Pollo", Precios: []Precio{
		{Texto: "13.00", Centimos: dineroDe(1300), Procedencia: Manuscrito},
		{Etiqueta: "familiar", Texto: "S/ 43", Centimos: dineroDe(9900)},
	}})

	c.Verificar()
	if m := c.Categorias[0].Platos[0].Revisar; m != PrecioDiscordante {
		t.Fatalf("motivo = %q, esperaba que lo discordante tape a lo manuscrito", m)
	}
}

// En la tarjeta se muestra "desde S/ 45", no el precio de la primera variante
// que haya devuelto el modelo.
func TestDesdeEsElMasBarato(t *testing.T) {
	p := conPrecios("Trio", pr("", "S/ 80", 8000), pr("", "S/ 45", 4500))
	if d := p.Desde(); d != dineroDe(4500) {
		t.Fatalf("Desde() = %s, esperaba S/ 45.00", d)
	}
	if d := (Plato{}).Desde(); d != 0 {
		t.Fatalf("un plato sin precios deberia dar 0, dio %s", d)
	}
}

// Un plato sin ningun precio no puede publicarse.
func TestPlatoSinPreciosSeMarca(t *testing.T) {
	c := cartaCon(Plato{Nombre: "Pescado del dia"})
	if n := c.Verificar().Total(); n != 1 {
		t.Fatalf("marco %d, esperaba 1", n)
	}
	if m := c.Categorias[0].Platos[0].Revisar; m != PrecioAusente {
		t.Fatalf("motivo = %q, esperaba precio_ausente", m)
	}
}

// Los dos niveles: lo que no cuadra se separa de lo que solo hay que mirar.
//
// Galponcito tiene 19 precios corregidos a mano donde las dos lecturas
// coinciden. Meterlos en la misma lista que un precio inventado deja al dueno
// con media carta en amarillo y ningun aviso que senale algo roto.
func TestManuscritoQueCuadraNoEsUnError(t *testing.T) {
	c := cartaCon(
		Plato{Nombre: "1/4 pollo", Precios: []Precio{
			{Texto: "13.00", Centimos: dineroDe(1300), Procedencia: Manuscrito}}},
		plato("Ceviche", "S/ 45", 9900), // este si esta roto
	)

	m := c.Verificar()
	if m.Revisar != 1 {
		t.Fatalf("Revisar = %d, esperaba 1: solo el precio discordante", m.Revisar)
	}
	if m.Confirmar != 1 {
		t.Fatalf("Confirmar = %d, esperaba 1: el manuscrito que cuadra", m.Confirmar)
	}
	if n := len(c.ParaRevisar()); n != 1 {
		t.Fatalf("ParaRevisar() = %d, esperaba 1", n)
	}
	if n := len(c.ParaConfirmar()); n != 1 {
		t.Fatalf("ParaConfirmar() = %d, esperaba 1", n)
	}
}

// Todo motivo cae en un nivel, y SinRevision en ninguno. Si se agrega un motivo
// nuevo y se olvida clasificarlo, cae por defecto en Revisar — que es el lado
// seguro: molesta al dueno en vez de publicar algo sin mirar.
func TestTodoMotivoTieneNivel(t *testing.T) {
	if SinRevision.Nivel() != NivelNinguno {
		t.Error("SinRevision deberia ser NivelNinguno")
	}
	if PrecioManuscrito.Nivel() != NivelConfirmar {
		t.Error("un manuscrito que cuadra solo se confirma, no es un error")
	}
	for _, m := range []MotivoRevision{PrecioIlegible, PrecioDiscordante,
		PrecioAusente, NombreVacio, VariantesSinNombre} {
		if m.Nivel() != NivelRevisar {
			t.Errorf("%q deberia ser NivelRevisar", m)
		}
	}
}

// Un precio corregido por el dueno NO se vuelve a cruzar contra el texto
// impreso. Sin esto, arreglar a mano el precio que el modelo leyo mal dejaba el
// plato marcado igual —el texto sigue diciendo lo que decia la carta— y publicar
// seguia bloqueado justo despues de arreglarlo.
func TestElPrecioDelDuenoNoSeDiscute(t *testing.T) {
	casos := []struct {
		nombre string
		precio Precio
		quiero MotivoRevision
	}{
		{
			"el modelo leyo 45 donde dice 48",
			Precio{Texto: "S/ 48", Centimos: 4500, Procedencia: Impreso},
			PrecioDiscordante,
		},
		{
			"el dueno lo corrigio a 48",
			Precio{Texto: "S/ 48", Centimos: 4800, Procedencia: DelDueno},
			SinRevision,
		},
		{
			// El caso que importa: el dueno pone un numero que NO coincide con lo
			// impreso, porque la carta tiene un sticker encima o el precio subio.
			"el dueno pone otro numero del que dice la carta",
			Precio{Texto: "S/ 48", Centimos: 5500, Procedencia: DelDueno},
			SinRevision,
		},
		{
			"un texto ilegible que el dueno resolvio",
			Precio{Texto: "12 a 15", Centimos: 1500, Procedencia: DelDueno},
			SinRevision,
		},
		{
			"sin texto, pero el dueno lo puso",
			Precio{Texto: "", Centimos: 3000, Procedencia: DelDueno},
			SinRevision,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p := Plato{Nombre: "Ceviche", Precios: []Precio{c.precio}}
			carta := Carta{Categorias: []Categoria{{Nombre: "Ceviches", Platos: []Plato{p}}}}
			carta.Verificar()
			if got := carta.Categorias[0].Platos[0].Revisar; got != c.quiero {
				t.Errorf("revisar = %q, esperaba %q", got, c.quiero)
			}
		})
	}
}
