package domain

import (
	"slices"
	"testing"
)

// Puntuar es el instrumento con el que se decide que modelo se usa en
// produccion. Si mide mal, todas las comparaciones son mentira — y a diferencia
// del codigo de la carta, aqui un error no lo delata ningun sintoma: solo
// produce un numero equivocado que parece razonable.

func esperado(nombre string, centimos int64) PlatoEsperado {
	return PlatoEsperado{Nombre: nombre, Centimos: dineroDe(centimos)}
}

func verdadCon(platos ...PlatoEsperado) Verdad { return Verdad{Platos: platos} }

func TestPuntuarCartaPerfecta(t *testing.T) {
	c := *cartaCon(
		plato("Ceviche", "S/ 45", 4500),
		plato("Lomo saltado", "S/ 38", 3800),
	)
	v := verdadCon(esperado("Ceviche", 4500), esperado("Lomo saltado", 3800))

	p := Puntuar(c, v)
	if p.Exactitud() != 1 {
		t.Fatalf("exactitud = %.2f, esperaba 1", p.Exactitud())
	}
	if len(p.Faltantes) != 0 || len(p.Sobrantes) != 0 || len(p.PrecioErroneo) != 0 {
		t.Fatalf("carta perfecta con fallos: %d faltan, %d sobran, %d precios mal",
			len(p.Faltantes), len(p.Sobrantes), len(p.PrecioErroneo))
	}
}

// El nombre coincide y el monto no: es el fallo que se publica sin que nadie lo
// note, y tiene que caer en PrecioErroneo, no en Sobrantes.
func TestPrecioErroneoNoEsUnSobrante(t *testing.T) {
	c := *cartaCon(plato("Ceviche", "S/ 54", 5400))
	v := verdadCon(esperado("Ceviche", 4500))

	p := Puntuar(c, v)
	if len(p.PrecioErroneo) != 1 {
		t.Fatalf("PrecioErroneo = %d, esperaba 1", len(p.PrecioErroneo))
	}
	if len(p.Sobrantes) != 0 {
		t.Fatalf("Sobrantes = %d, esperaba 0: el nombre si existe", len(p.Sobrantes))
	}
	d := p.PrecioErroneo[0]
	if d.Esperado != dineroDe(4500) || d.Obtenido != dineroDe(5400) {
		t.Fatalf("discrepancia = esperaba %s obtuvo %s", d.Esperado, d.Obtenido)
	}
}

// Un plato que la carta no tiene es un invento, y es distinto de un precio mal.
func TestPlatoInventadoEsSobrante(t *testing.T) {
	c := *cartaCon(plato("Ceviche", "S/ 45", 4500), plato("Pizza hawaiana", "S/ 30", 3000))
	v := verdadCon(esperado("Ceviche", 4500))

	p := Puntuar(c, v)
	if len(p.Sobrantes) != 1 {
		t.Fatalf("Sobrantes = %d, esperaba 1", len(p.Sobrantes))
	}
	if len(p.PrecioErroneo) != 0 {
		t.Fatalf("PrecioErroneo = %d, esperaba 0", len(p.PrecioErroneo))
	}
}

// El caso real de Galponcito: "1/4 pollo" existe dos veces a precios distintos
// porque en mesa cuesta menos que para llevar. No se deduplica.
func TestMismoNombreADosPreciosNoSeDeduplica(t *testing.T) {
	c := *cartaCon(plato("1/4 pollo", "S/ 11", 1100), plato("1/4 pollo", "S/ 13", 1300))
	v := verdadCon(esperado("1/4 pollo", 1100), esperado("1/4 pollo", 1300))

	p := Puntuar(c, v)
	if p.Aciertos != 2 {
		t.Fatalf("Aciertos = %d, esperaba 2", p.Aciertos)
	}
	if len(p.Sobrantes) != 0 {
		t.Fatalf("Sobrantes = %d, esperaba 0", len(p.Sobrantes))
	}
}

// Con las dos lineas de la verdad ya consumidas, una TERCERA aparicion del mismo
// nombre es un invento del modelo, no un precio equivocado. Confundirlas
// corrompe justo la metrica que decide que modelo se usa.
func TestNombreRepetidoDeMasEsInventoNoPrecioMalo(t *testing.T) {
	c := *cartaCon(
		plato("1/4 pollo", "S/ 11", 1100),
		plato("1/4 pollo", "S/ 13", 1300),
		plato("1/4 pollo", "S/ 99", 9900), // la carta solo lo tiene dos veces
	)
	v := verdadCon(esperado("1/4 pollo", 1100), esperado("1/4 pollo", 1300))

	p := Puntuar(c, v)
	if p.Aciertos != 2 {
		t.Fatalf("Aciertos = %d, esperaba 2", p.Aciertos)
	}
	if len(p.Sobrantes) != 1 {
		t.Fatalf("Sobrantes = %d, esperaba 1: la verdad ya estaba agotada", len(p.Sobrantes))
	}
	if len(p.PrecioErroneo) != 0 {
		t.Fatalf("PrecioErroneo = %d, esperaba 0", len(p.PrecioErroneo))
	}
}

// Cuando quedan lineas de la verdad sin usar, la discrepancia tiene que
// compararse contra una que SIGA disponible. Reportar contra una ya acertada le
// dice al que lee "esperaba 11, dijo 99" cuando el 11 salio perfecto.
func TestDiscrepanciaSeComparaContraUnaLineaNoUsada(t *testing.T) {
	c := *cartaCon(
		plato("1/4 pollo", "S/ 11", 1100), // exacto
		plato("1/4 pollo", "S/ 99", 9900), // deberia compararse contra el de 1300
	)
	v := verdadCon(esperado("1/4 pollo", 1100), esperado("1/4 pollo", 1300))

	p := Puntuar(c, v)
	if len(p.PrecioErroneo) != 1 {
		t.Fatalf("PrecioErroneo = %d, esperaba 1", len(p.PrecioErroneo))
	}
	if got := p.PrecioErroneo[0].Esperado; got != dineroDe(1300) {
		t.Fatalf("esperado = %s, deberia ser S/ 13.00: el de S/ 11.00 ya se acerto", got)
	}
}

// Los faltantes se le muestran a una persona. Con el nombre normalizado dice
// "2pollospapasensalada" en vez de "2 pollos + papas + ensalada".
func TestFaltantesConservanElNombreLegible(t *testing.T) {
	c := *cartaCon(plato("Ceviche", "S/ 45", 4500))
	v := verdadCon(esperado("Ceviche", 4500), esperado("2 pollos + papas + ensalada", 6700))

	p := Puntuar(c, v)
	if len(p.Faltantes) != 1 {
		t.Fatalf("Faltantes = %d, esperaba 1", len(p.Faltantes))
	}
	if got := p.Faltantes[0].Nombre; got != "2 pollos + papas + ensalada" {
		t.Fatalf("nombre = %q, esperaba el original sin normalizar", got)
	}
}

// Dos corridas del mismo puntaje tienen que dar el mismo informe. Recorrer un
// mapa devuelve orden aleatorio y hace que dos informes identicos se vean
// distintos.
func TestElInformeEsEstable(t *testing.T) {
	c := *cartaCon(plato("Ceviche", "S/ 45", 4500))
	v := verdadCon(
		esperado("Ceviche", 4500),
		esperado("Aji de gallina", 3200),
		esperado("Lomo saltado", 3800),
		esperado("Arroz con mariscos", 4200),
		esperado("Causa", 1800),
	)

	primero := nombresDe(Puntuar(c, v).Faltantes)
	for range 20 {
		if got := nombresDe(Puntuar(c, v).Faltantes); !slices.Equal(got, primero) {
			t.Fatalf("el orden de los faltantes cambia entre corridas:\n%v\n%v", primero, got)
		}
	}
}

func nombresDe(fs []PlatoEsperado) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Nombre
	}
	return out
}

// La verdad no distingue si el modelo agrupo dos montos en un plato o los partio
// en dos platos identicos: aplanados dan las mismas lineas. Pero para el cliente
// son cosas distintas, asi que el puntaje tiene que verlo.
func TestPlatosDuplicadosSeDetectan(t *testing.T) {
	partido := *cartaCon(
		plato("Trio marino", "S/ 45", 4500),
		plato("Trio marino", "S/ 80", 8000),
	)
	agrupado := *cartaCon(conPrecios("Trio marino",
		pr("", "S/ 45", 4500),
		pr("", "S/ 80", 8000),
	))
	v := verdadCon(esperado("Trio marino", 4500), esperado("Trio marino", 8000))

	pPartido := Puntuar(partido, v)
	pAgrupado := Puntuar(agrupado, v)

	if pPartido.Exactitud() != 1 || pAgrupado.Exactitud() != 1 {
		t.Fatal("las dos formas aciertan las lineas; eso es correcto")
	}
	if len(pPartido.Duplicados) != 1 {
		t.Fatalf("Duplicados = %d, esperaba 1: dos filas identicas a distinto precio",
			len(pPartido.Duplicados))
	}
	if len(pAgrupado.Duplicados) != 0 {
		t.Fatalf("Duplicados = %d, esperaba 0: es un plato con dos precios",
			len(pAgrupado.Duplicados))
	}
}

// El mismo nombre en DOS SECCIONES distintas no es un duplicado: es lo que hace
// Galponcito con "1/4 pollo" en mesa y para llevar.
func TestMismoNombreEnDosCategoriasNoEsDuplicado(t *testing.T) {
	c := Carta{Categorias: []Categoria{
		{Nombre: "En mesa", Platos: []Plato{plato("1/4 pollo", "S/ 11", 1100)}},
		{Nombre: "Para llevar", Platos: []Plato{plato("1/4 pollo", "S/ 13", 1300)}},
	}}
	v := verdadCon(esperado("1/4 pollo", 1100), esperado("1/4 pollo", 1300))

	if n := len(Puntuar(c, v).Duplicados); n != 0 {
		t.Fatalf("Duplicados = %d, esperaba 0: estan en secciones distintas", n)
	}
}

func TestManuscritosSeCuentanYLosFalsosTambien(t *testing.T) {
	c := *cartaCon(
		Plato{Nombre: "1/4 pollo", Precios: []Precio{
			{Texto: "13.00", Centimos: dineroDe(1300), Procedencia: Manuscrito}}},
		Plato{Nombre: "Ceviche", Precios: []Precio{
			{Texto: "S/ 45", Centimos: dineroDe(4500), Procedencia: Manuscrito}}},
	)
	v := Verdad{Platos: []PlatoEsperado{
		{Nombre: "1/4 pollo", Centimos: dineroDe(1300), Manuscrito: true},
		{Nombre: "Ceviche", Centimos: dineroDe(4500), Manuscrito: false},
	}}

	p := Puntuar(c, v)
	if p.ManuscritosEsperados != 1 {
		t.Fatalf("ManuscritosEsperados = %d, esperaba 1", p.ManuscritosEsperados)
	}
	if p.ManuscritosAcertados != 1 {
		t.Fatalf("ManuscritosAcertados = %d, esperaba 1", p.ManuscritosAcertados)
	}
	if p.ManuscritosFalsos != 1 {
		t.Fatalf("ManuscritosFalsos = %d, esperaba 1: el ceviche es impreso", p.ManuscritosFalsos)
	}
}

// "COMBO 1" sin lo que trae no lo compra nadie.
func TestCombosSinContenido(t *testing.T) {
	c := *cartaCon(
		Plato{Nombre: "COMBO 1", Descripcion: "pollo + papas", Precios: []Precio{
			{Texto: "S/ 49", Centimos: dineroDe(4900)}}},
		plato("COMBO 2", "S/ 49", 4900),
	)
	v := Verdad{
		Platos: []PlatoEsperado{
			{Nombre: "COMBO 1", Centimos: dineroDe(4900)},
			{Nombre: "COMBO 2", Centimos: dineroDe(4900)},
		},
		CombosConTexto: []string{"COMBO 1", "COMBO 2"},
	}

	p := Puntuar(c, v)
	if !slices.Equal(p.CombosSinContenido, []string{"COMBO 2"}) {
		t.Fatalf("CombosSinContenido = %v, esperaba [COMBO 2]", p.CombosSinContenido)
	}
}

func TestNormalizarIgualaLoQueEsElMismoPlato(t *testing.T) {
	iguales := [][2]string{
		{"1/4 POLLO", "1/4 pollo"},
		{"Broaster+Chaufa", "Broaster + Chaufa"},
		{"  Ceviche  ", "Ceviche"},
		{"Ají de gallina", "Aji de gallina"},
		{"Piña", "Pina"},
		{"COMBO GALPONCITO", "Combo Galponcito"},
	}
	for _, par := range iguales {
		if normalizar(par[0]) != normalizar(par[1]) {
			t.Errorf("%q y %q deberian ser el mismo plato (%q vs %q)",
				par[0], par[1], normalizar(par[0]), normalizar(par[1]))
		}
	}

	distintos := [][2]string{
		{"Ceviche mixto", "Ceviche"},
		{"1/4 pollo", "1/8 pollo"},
		{"Chicharron de pescado", "Chicharron de calamar"},
	}
	for _, par := range distintos {
		if normalizar(par[0]) == normalizar(par[1]) {
			t.Errorf("%q y %q NO son el mismo plato pero normalizan igual", par[0], par[1])
		}
	}
}

// Una verdad vacia no debe dar 100% ni dividir por cero.
func TestVerdadVaciaNoDaCienPorCiento(t *testing.T) {
	p := Puntuar(*cartaCon(plato("Ceviche", "S/ 45", 4500)), Verdad{})
	if p.Exactitud() != 0 {
		t.Fatalf("exactitud = %.2f con una verdad vacia, esperaba 0", p.Exactitud())
	}
}
