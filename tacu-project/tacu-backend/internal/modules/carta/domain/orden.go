package domain

import (
	"fmt"
	"strings"
)

// Reordenar aplica una permutacion de categorias y las renombra.
//
// Existe porque la carta sale en orden de LECTURA y hay que publicarla en orden
// de VENTA. Galponcito empieza por "Porciones" —las guarniciones— y deja
// "Combos" en el puesto 6 de 7, cuando la carta fisica les da media hoja en
// recuadros grandes. Y los nombres bailan entre corridas: "Porciones" una vez,
// "PORCIONES Y CHAUFAS" la siguiente.
//
// EL MODELO NUNCA VE LOS PLATOS. Solo devuelve indices y nombres de categoria, y
// esta funcion los aplica. Es lo que hace la operacion segura por construccion:
// estructuralmente no puede perder un plato, duplicarlo ni moverle el precio.
//
// El guard de permutacion NO es defensivo, es obligatorio. Si el modelo devuelve
// un indice repetido, la categoria sale dos veces; si omite uno, TODOS los platos
// de esa categoria desaparecen del catalogo publicado — sin error, sin log, y sin
// que nadie lo note hasta que un cliente pregunte por un plato que ya no esta.
func (c Carta) Reordenar(orden []int, nombres []string) (Carta, error) {
	n := len(c.Categorias)

	if len(orden) != n {
		return Carta{}, fmt.Errorf("el orden trae %d categorias y la carta tiene %d", len(orden), n)
	}
	if len(nombres) != 0 && len(nombres) != n {
		return Carta{}, fmt.Errorf("llegaron %d nombres para %d categorias", len(nombres), n)
	}

	visto := make([]bool, n)
	for _, i := range orden {
		if i < 0 || i >= n {
			return Carta{}, fmt.Errorf("el indice %d esta fuera de rango (0..%d)", i, n-1)
		}
		if visto[i] {
			return Carta{}, fmt.Errorf("el indice %d viene repetido: duplicaria esa categoria", i)
		}
		visto[i] = true
	}
	// Con la misma cantidad, sin repetidos y todos en rango, no puede faltar
	// ninguno. La comprobacion queda igual porque el dia que alguien afloje una
	// de las tres condiciones, esta es la que sigue atrapando la perdida.
	for i, v := range visto {
		if !v {
			return Carta{}, fmt.Errorf("falta el indice %d: sus platos desaparecerian", i)
		}
	}

	nueva := Carta{Categorias: make([]Categoria, 0, n)}
	for pos, i := range orden {
		cat := c.Categorias[i]
		if len(nombres) == n {
			if nombre := strings.TrimSpace(nombres[pos]); nombre != "" {
				cat.Nombre = nombre
			}
		}
		nueva.Categorias = append(nueva.Categorias, cat)
	}
	return nueva, nil
}

// CategoriasResumidas describe la carta sin los platos, que es lo unico que se
// le manda al modelo para que decida el orden.
//
// Se mandan tres nombres de ejemplo y no la categoria entera por dos motivos:
// una carta de 77 platos costaria bastante mas en tokens, y sobre todo el modelo
// no puede tocar lo que no ve.
type CategoriaResumida struct {
	Indice   int      `json:"indice"`
	Nombre   string   `json:"nombre"`
	NPlatos  int      `json:"n_platos"`
	Ejemplos []string `json:"ejemplos"`
}

func (c Carta) Resumir() []CategoriaResumida {
	out := make([]CategoriaResumida, 0, len(c.Categorias))
	for i, cat := range c.Categorias {
		r := CategoriaResumida{Indice: i, Nombre: cat.Nombre, NPlatos: len(cat.Platos)}
		for j, p := range cat.Platos {
			if j == 3 {
				break
			}
			r.Ejemplos = append(r.Ejemplos, p.Nombre)
		}
		out = append(out, r)
	}
	return out
}
