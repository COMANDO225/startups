package domain

import (
	"strings"
	"unicode"

	"tacu-backend/internal/kernel/dinero"
)

// PlatoEsperado es una linea de la verdad de referencia de una carta.
//
// La identidad de un plato es (nombre, precio) y no solo el nombre: "1/4 pollo"
// aparece dos veces en la misma carta a precios distintos porque en mesa cuesta
// menos que para llevar, y eso NO es un error que haya que deduplicar.
type PlatoEsperado struct {
	Nombre     string          `json:"nombre"`
	Centimos   dinero.Centimos `json:"centimos"`
	Manuscrito bool            `json:"manuscrito"`
}

// Verdad es la carta de referencia contra la que se puntua una extraccion.
type Verdad struct {
	Platos         []PlatoEsperado `json:"platos"`
	CombosConTexto []string        `json:"_combos_con_contenido"`
}

// Puntaje mide una extraccion contra la verdad.
//
// Lo que importa no es cuantos platos salieron, sino cuantos salieron CON EL
// PRECIO CORRECTO. Un plato con el precio equivocado es peor que un plato
// faltante: el faltante se nota, el precio malo se publica.
type Puntaje struct {
	Esperados int
	Extraidos int

	Aciertos  int // (nombre, precio) que coinciden con la verdad
	Faltantes []PlatoEsperado
	Sobrantes []Linea // extraidos que no estan en la verdad

	// De los que acertaron el nombre, cuantos tenian el precio mal. Es el
	// numero que de verdad duele.
	PrecioErroneo []Discrepancia

	// Deteccion de precios corregidos a mano.
	ManuscritosEsperados int
	ManuscritosAcertados int
	ManuscritosFalsos    int

	// Combos que la carta lista con su contenido y que salieron sin el.
	CombosSinContenido []string

	// Duplicados son los platos que salieron dos veces con el mismo nombre en la
	// misma categoria a distinto precio. Aplanados aciertan todas las lineas, asi
	// que la exactitud no los ve — pero al cliente le dejan dos filas identicas y
	// ninguna forma de elegir. Es un plato con dos precios, no dos platos.
	Duplicados []string
}

type Discrepancia struct {
	Nombre   string
	Esperado dinero.Centimos
	Obtenido dinero.Centimos
}

// Exactitud es la fraccion de platos esperados que salieron con su precio bien.
func (p Puntaje) Exactitud() float64 {
	if p.Esperados == 0 {
		return 0
	}
	return float64(p.Aciertos) / float64(p.Esperados)
}

// Puntuar compara una carta extraida contra la verdad.
//
// Se puntua sobre pares (plato, precio) aplanados: un plato con dos precios en
// la misma fila son dos lineas de la verdad, igual que el mismo plato repetido
// en dos secciones. Asi la verdad de referencia no depende de si el modelo
// modela las variantes como un plato o como dos.
func Puntuar(extraida Carta, v Verdad) Puntaje {
	lineas := aplanar(extraida)

	p := Puntaje{
		Esperados: len(v.Platos),
		Extraidos: len(lineas),
	}

	// La verdad se lleva como slice con marcas de uso, no como mapa. Cada linea
	// esperada se puede acertar UNA sola vez —"1/4 pollo" esta dos veces a
	// precios distintos y las dos cuentan— y el slice ademas conserva el nombre
	// tal como se escribio y hace que el informe salga siempre igual.
	usada := make([]bool, len(v.Platos))
	for _, e := range v.Platos {
		if e.Manuscrito {
			p.ManuscritosEsperados++
		}
	}

	// PRIMERA PASADA: solo aciertos exactos, nombre y precio. Va completa antes
	// de buscar discrepancias porque si no, una linea con el precio mal puede
	// quedarse con la pareja de otra que si acerto, y entonces la que acerto se
	// reporta como faltante.
	var sinPareja []Linea
	for _, x := range lineas {
		i := buscarExacto(v.Platos, usada, x.Nombre, x.Centimos)
		if i < 0 {
			sinPareja = append(sinPareja, x)
			continue
		}
		usada[i] = true
		p.Aciertos++

		if x.Precio.Procedencia == Manuscrito {
			if v.Platos[i].Manuscrito {
				p.ManuscritosAcertados++
			} else {
				p.ManuscritosFalsos++
			}
		}
	}

	// SEGUNDA PASADA: de los que quedaron, el que coincide con una linea AUN
	// LIBRE trae el precio equivocado. El que no coincide con ninguna es un
	// invento. La diferencia importa: el precio equivocado se publica sin que
	// nadie lo note, el invento salta a la vista.
	for _, x := range sinPareja {
		i := buscarPorNombre(v.Platos, usada, x.Nombre)
		if i < 0 {
			p.Sobrantes = append(p.Sobrantes, x)
			continue
		}
		usada[i] = true
		p.PrecioErroneo = append(p.PrecioErroneo, Discrepancia{
			Nombre: x.Nombre, Esperado: v.Platos[i].Centimos, Obtenido: x.Centimos,
		})
	}

	for i, u := range usada {
		if !u {
			p.Faltantes = append(p.Faltantes, v.Platos[i])
		}
	}

	// Un combo sin su contenido no se vende: nadie compra "COMBO 1".
	conContenido := map[string]bool{}
	for _, x := range lineas {
		if strings.TrimSpace(x.Descripcion) != "" {
			conContenido[normalizar(x.Nombre)] = true
		}
	}
	for _, c := range v.CombosConTexto {
		if !conContenido[normalizar(c)] {
			p.CombosSinContenido = append(p.CombosSinContenido, c)
		}
	}

	p.Duplicados = duplicados(extraida)
	return p
}

func buscarExacto(esperados []PlatoEsperado, usada []bool, nombre string, c dinero.Centimos) int {
	n := normalizar(nombre)
	for i, e := range esperados {
		if !usada[i] && e.Centimos == c && normalizar(e.Nombre) == n {
			return i
		}
	}
	return -1
}

func buscarPorNombre(esperados []PlatoEsperado, usada []bool, nombre string) int {
	n := normalizar(nombre)
	for i, e := range esperados {
		if !usada[i] && normalizar(e.Nombre) == n {
			return i
		}
	}
	return -1
}

// duplicados busca el defecto que la exactitud no puede ver: el mismo plato
// listado dos veces en la misma seccion a distinto precio. Aplanado acierta
// todas las lineas, pero publicado son dos filas iguales sin forma de elegir.
//
// En dos secciones distintas NO es un duplicado: "1/4 pollo" cuesta menos en
// mesa que para llevar, y eso la carta lo dice a proposito.
func duplicados(c Carta) []string {
	var out []string
	for _, cat := range c.Categorias {
		veces := map[string]int{}
		for _, p := range cat.Platos {
			veces[normalizar(p.Nombre)]++
		}
		emitido := map[string]bool{}
		for _, p := range cat.Platos {
			n := normalizar(p.Nombre)
			if veces[n] > 1 && !emitido[n] {
				emitido[n] = true
				out = append(out, p.Nombre)
			}
		}
	}
	return out
}

// Linea es un par (plato, precio) aplanado, que es la unidad que se puntua.
type Linea struct {
	Plato
	Precio
}

func aplanar(c Carta) []Linea {
	var out []Linea
	for _, p := range c.Platos() {
		for _, pr := range p.Precios {
			out = append(out, Linea{Plato: p, Precio: pr})
		}
	}
	return out
}

// normalizar hace comparables dos nombres escritos distinto: quita tildes,
// mayusculas, espacios de sobra y signos. "1/4 POLLO" y "1/4 pollo" son el
// mismo plato; "Broaster+Chaufa" y "Broaster + Chaufa" tambien.
func normalizar(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(sinTilde(r))
		case r == '/' || r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

var tildes = map[rune]rune{
	'á': 'a', 'é': 'e', 'í': 'i', 'ó': 'o', 'ú': 'u',
	'ü': 'u', 'ñ': 'n', 'à': 'a', 'è': 'e', 'ì': 'i', 'ò': 'o', 'ù': 'u',
}

func sinTilde(r rune) rune {
	if s, ok := tildes[r]; ok {
		return s
	}
	return r
}
