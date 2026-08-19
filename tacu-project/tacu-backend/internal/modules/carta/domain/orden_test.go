package domain

import (
	"strings"
	"testing"
)

// La carta de Galponcito tal como sale de la extraccion: en orden de lectura,
// con las guarniciones primero y los combos casi al final.
func cartaEnOrdenDeLectura() Carta {
	con := func(nombre string, platos ...string) Categoria {
		c := Categoria{Nombre: nombre}
		for _, p := range platos {
			c.Platos = append(c.Platos, plato(p, "S/ 10", 1000))
		}
		return c
	}
	return Carta{Categorias: []Categoria{
		con("Porciones", "Porcion de papas", "Porcion ensalada", "chaufa de pollo"),
		con("EN MESA", "1/4 pollo", "Mostro", "Mostrito"),
		con("PARA LLEVAR", "1/4 pollo", "1/2 pollo", "1 pollo"),
		con("BEBIDAS", "Agua de mesa", "Litro", "Cerveza Pilsen"),
		con("Combos", "COMBO 1", "COMBO 2", "COMBO GALPONCITO"),
	}}
}

func nombresDeCategorias(c Carta) []string {
	out := make([]string, len(c.Categorias))
	for i, cat := range c.Categorias {
		out[i] = cat.Nombre
	}
	return out
}

func TestReordenarPoneLosCombosPrimero(t *testing.T) {
	c := cartaEnOrdenDeLectura()

	// Lo que devolveria el modelo: combos primero, guarniciones al final.
	nueva, err := c.Reordenar(
		[]int{4, 1, 2, 0, 3},
		[]string{"Combos", "En mesa", "Para llevar", "Porciones", "Bebidas"},
	)
	if err != nil {
		t.Fatalf("Reordenar: %v", err)
	}

	got := nombresDeCategorias(nueva)
	if got[0] != "Combos" {
		t.Fatalf("la primera categoria es %q, esperaba Combos", got[0])
	}
	if got[len(got)-1] != "Bebidas" {
		t.Errorf("la ultima es %q, esperaba Bebidas", got[len(got)-1])
	}

	// Y NINGUN plato se movio de su categoria ni desaparecio.
	if n := len(nueva.Platos()); n != len(c.Platos()) {
		t.Fatalf("quedaron %d platos de %d", n, len(c.Platos()))
	}
	for _, cat := range nueva.Categorias {
		if len(cat.Platos) != 3 {
			t.Errorf("la categoria %q quedo con %d platos", cat.Nombre, len(cat.Platos))
		}
	}
}

// EL GUARD QUE JUSTIFICA TODO ESTO.
//
// Un indice omitido borra TODOS los platos de esa categoria del catalogo
// publicado: sin error, sin log, y sin que nadie lo note hasta que un cliente
// pregunte por un plato que ya no aparece.
func TestUnIndiceQueFaltaNoBorraUnaCategoria(t *testing.T) {
	c := cartaEnOrdenDeLectura()

	// Al modelo se le "olvido" el 3 (BEBIDAS) y repitio el 0.
	_, err := c.Reordenar([]int{4, 1, 2, 0, 0}, nil)
	if err == nil {
		t.Fatal("acepto una permutacion con un indice repetido: duplicaria una categoria y borraria otra")
	}
	if !strings.Contains(err.Error(), "repetido") {
		t.Errorf("el error no dice cual es el problema: %v", err)
	}
}

func TestReordenarRechazaLoQueRomperiaLaCarta(t *testing.T) {
	c := cartaEnOrdenDeLectura() // 5 categorias

	casos := []struct {
		que    string
		orden  []int
		motivo string
	}{
		{"faltan indices", []int{0, 1, 2}, "categorias"},
		{"sobran indices", []int{0, 1, 2, 3, 4, 0}, "categorias"},
		{"un indice fuera de rango", []int{0, 1, 2, 3, 9}, "fuera de rango"},
		{"un indice negativo", []int{0, 1, 2, 3, -1}, "fuera de rango"},
		{"todos repetidos", []int{0, 0, 0, 0, 0}, "repetido"},
		{"vacio", []int{}, "categorias"},
	}

	for _, caso := range casos {
		t.Run(caso.que, func(t *testing.T) {
			_, err := c.Reordenar(caso.orden, nil)
			if err == nil {
				t.Fatalf("acepto %v", caso.orden)
			}
			if !strings.Contains(err.Error(), caso.motivo) {
				t.Errorf("err = %v, esperaba que mencionara %q", err, caso.motivo)
			}
		})
	}
}

// Los nombres bailan entre corridas ("Porciones" / "PORCIONES Y CHAUFAS"), asi
// que el modelo tambien los fija. Pero un nombre vacio no puede borrar el que
// habia: es peor una categoria sin titulo que una con el titulo original.
func TestUnNombreVacioNoBorraElQueHabia(t *testing.T) {
	c := cartaEnOrdenDeLectura()

	nueva, err := c.Reordenar(
		[]int{0, 1, 2, 3, 4},
		[]string{"Porciones y chaufas", "", "   ", "Bebidas", "Combos"},
	)
	if err != nil {
		t.Fatal(err)
	}

	got := nombresDeCategorias(nueva)
	if got[0] != "Porciones y chaufas" {
		t.Errorf("no aplico el nombre nuevo: %q", got[0])
	}
	if got[1] != "EN MESA" {
		t.Errorf("un nombre vacio piso el original: %q", got[1])
	}
	if got[2] != "PARA LLEVAR" {
		t.Errorf("un nombre con solo espacios piso el original: %q", got[2])
	}
}

// Sin nombres, solo se reordena. Es el caso de "el modelo fallo pero el orden
// que dio sirve".
func TestReordenarSinNombres(t *testing.T) {
	c := cartaEnOrdenDeLectura()

	nueva, err := c.Reordenar([]int{4, 3, 2, 1, 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if nueva.Categorias[0].Nombre != "Combos" {
		t.Errorf("primera = %q", nueva.Categorias[0].Nombre)
	}
	if nueva.Categorias[4].Nombre != "Porciones" {
		t.Errorf("ultima = %q", nueva.Categorias[4].Nombre)
	}
}

// La cantidad de nombres tiene que cuadrar con la de categorias, o se estarian
// aplicando nombres corridos de posicion.
func TestLosNombresTienenQueCuadrar(t *testing.T) {
	c := cartaEnOrdenDeLectura()

	if _, err := c.Reordenar([]int{0, 1, 2, 3, 4}, []string{"A", "B"}); err == nil {
		t.Fatal("acepto 2 nombres para 5 categorias")
	}
}

// Reordenar no puede tocar la carta original: si el resultado se descarta —porque
// el modelo fallo— la de partida tiene que seguir intacta.
func TestReordenarNoTocaLaCartaOriginal(t *testing.T) {
	c := cartaEnOrdenDeLectura()
	antes := nombresDeCategorias(c)

	if _, err := c.Reordenar([]int{4, 3, 2, 1, 0}, []string{"a", "b", "c", "d", "e"}); err != nil {
		t.Fatal(err)
	}

	despues := nombresDeCategorias(c)
	for i := range antes {
		if antes[i] != despues[i] {
			t.Fatalf("la carta original cambio: %v -> %v", antes, despues)
		}
	}
}

func TestUnaCartaVaciaSeReordenaSinDrama(t *testing.T) {
	if _, err := (Carta{}).Reordenar(nil, nil); err != nil {
		t.Fatalf("una carta vacia deberia reordenarse sin error: %v", err)
	}
}

// Resumir es lo unico que ve el modelo. Si trajera los platos enteros, podria
// devolverlos cambiados — y ademas costaria bastante mas en tokens.
func TestResumirNoLlevaLosPlatosEnteros(t *testing.T) {
	c := cartaEnOrdenDeLectura()
	r := c.Resumir()

	if len(r) != 5 {
		t.Fatalf("%d categorias resumidas, esperaba 5", len(r))
	}
	for i, cat := range r {
		if cat.Indice != i {
			t.Errorf("indice = %d, esperaba %d", cat.Indice, i)
		}
		if cat.NPlatos != 3 {
			t.Errorf("%s: n_platos = %d", cat.Nombre, cat.NPlatos)
		}
		if len(cat.Ejemplos) > 3 {
			t.Errorf("%s: %d ejemplos, el tope son 3", cat.Nombre, len(cat.Ejemplos))
		}
	}
	if r[4].Nombre != "Combos" {
		t.Errorf("el resumen no conserva los nombres: %q", r[4].Nombre)
	}
}

func TestResumirTopeaLosEjemplos(t *testing.T) {
	var cat Categoria
	cat.Nombre = "BEBIDAS"
	for i := range 13 { // las 13 bebidas de La Tribuna
		cat.Platos = append(cat.Platos, plato("bebida", "S/ 8", 800+int64(i)))
	}

	r := Carta{Categorias: []Categoria{cat}}.Resumir()
	if len(r[0].Ejemplos) != 3 {
		t.Fatalf("%d ejemplos, esperaba 3", len(r[0].Ejemplos))
	}
	if r[0].NPlatos != 13 {
		t.Errorf("n_platos = %d, esperaba 13: el conteo si tiene que ser real", r[0].NPlatos)
	}
}
