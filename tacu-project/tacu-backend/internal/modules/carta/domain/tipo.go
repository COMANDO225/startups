package domain

import (
	"sort"
	"strings"
)

// El tipo de restaurante decide como emplata el negocio y que formatos existen
// en el: "ronda" en una cevicheria es un plato con acoples radiales, en una
// polleria es una ronda de cervezas.
//
// Los descriptores van en ingles, igual que en porcion.go y formato.go.

type Tipo string

const (
	Cevicheria   Tipo = "cevicheria"
	Polleria     Tipo = "polleria"
	Chifa        Tipo = "chifa"
	Pizzeria     Tipo = "pizzeria"
	Parrilla     Tipo = "parrilla"
	Criollo      Tipo = "criollo"
	Sanguicheria Tipo = "sanguicheria"
	Generico     Tipo = "generico"
)

// TiposConocidos es lo que ofrece el selector, ordenado por lo que mas abunda.
var TiposConocidos = []Tipo{
	Cevicheria, Polleria, Chifa, Criollo, Parrilla, Pizzeria, Sanguicheria, Generico,
}

func (t Tipo) Valido() bool {
	_, ok := tipos[t]
	return ok
}

// Nombre es como se le ensena al dueno. Corto: va en un chip.
func (t Tipo) Nombre() string {
	if d, ok := tipos[t]; ok {
		return d.nombre
	}
	return "Otro"
}

// Descripcion es la linea de ayuda del selector.
func (t Tipo) Descripcion() string {
	if d, ok := tipos[t]; ok {
		return d.descripcion
	}
	return ""
}

// Receta devuelve el acompanamiento de la casa y el recipiente por defecto. El
// dueno puede pisar el recipiente desde su base; el acompanamiento se queda.
func (t Tipo) Receta() Receta {
	d, ok := tipos[t]
	if !ok {
		d = tipos[Generico]
	}
	return Receta{Recipiente: d.recipiente, Acompanamiento: d.acompanamiento}
}

// PermiteFormato dice si un formato tiene sentido en este negocio.
func (t Tipo) PermiteFormato(clave string) bool {
	d, ok := tipos[t]
	if !ok {
		d = tipos[Generico]
	}
	for _, f := range d.formatos {
		if f == clave {
			return true
		}
	}
	return false
}

type definicionTipo struct {
	nombre string

	// descripcion es la linea de abajo en el selector. Existe para que el
	// nombre pueda ser UNA palabra: "Cevicheria / marisqueria" no cabe en un
	// chip y se corta.
	descripcion string

	recipiente     string
	acompanamiento string

	// formatos: claves de FormatoDePlato validas aqui. "trio" y "ronda" solo
	// significan un plato con compartimentos en una cevicheria.
	formatos []string
}

const platoBlanco = "a plain white round ceramic restaurant dinner plate, the standard size, " +
	"with the serving in the middle of it."

var tipos = map[Tipo]definicionTipo{
	Cevicheria: {
		nombre: "Cevichería", descripcion: "ceviches, tiraditos, jaleas y mariscos",
		recipiente: platoBlanco,
		acompanamiento: "PLATED THE PERUVIAN CEVICHERIA WAY — arranged around the main food on the " +
			"same plate, each element in its own small separate pile and none of them mixed into " +
			"the main food: two thick round slices of cooked orange sweet potato, a spoonful of " +
			"large white boiled corn kernels, a small heap of toasted golden corn, a crisp green " +
			"lettuce leaf underneath one edge, and a wedge of lime.",
		formatos: []string{"fuente", "trio", "ronda"},
	},

	Polleria: {
		nombre: "Pollería", descripcion: "pollo a la brasa, broaster, salchipapas",
		recipiente: platoBlanco,
		acompanamiento: "SAUCES — four small individual sauce bowls standing on the surface beside " +
			"the main plate, the ones a Peruvian chicken place serves with fries: one bright red " +
			"tomato ketchup, one thick white mayonnaise, one yellow mustard, and one dark green " +
			"herb chili cream. Each sauce stays inside its own bowl, smooth and untouched, and " +
			"none of them is poured over the food.",
		formatos: []string{"fuente"},
	},

	Chifa: {
		nombre: "Chifa", descripcion: "chaufa, wantán, tallarín saltado",
		recipiente: platoBlanco,
		acompanamiento: "PLATED THE CHIFA WAY — the dish stands on its own on the plate, with a " +
			"small white porcelain bowl of dark soy sauce beside it and a pair of chopsticks " +
			"resting on the rim of that bowl. Nothing is scattered over the main food.",
		formatos: []string{"fuente"},
	},

	Pizzeria: {
		nombre: "Pizzería", descripcion: "pizzas, calzone, pastas",
		recipiente: "a round wooden serving board, plain and unvarnished, with the pizza sitting " +
			"flat on it and already cut into even slices.",
		acompanamiento: "the slices stay together in their round shape on the board, with one slice " +
			"very slightly pulled out from the rest so the cut lines are visible.",
		formatos: []string{"fuente"},
	},

	Parrilla: {
		nombre: "Parrilla", descripcion: "anticuchos, brochetas, churrasco",
		recipiente: "a dark cast-iron griddle plate set on a plain wooden board, with the grilled " +
			"food arranged on the iron.",
		acompanamiento: "beside the grilled meat on the same iron: one whole boiled potato cut in " +
			"half and golden at the edges, a short piece of boiled corn on the cob, and a small " +
			"pot of dark red chili sauce.",
		formatos: []string{"fuente"},
	},

	Criollo: {
		nombre: "Criolla", descripcion: "lomo saltado, ají de gallina, seco",
		recipiente: platoBlanco,
		acompanamiento: "a neat mound of plain white steamed rice sits on one side of the same " +
			"plate, separate from the main food and not mixed into it.",
		formatos: []string{"fuente"},
	},

	Sanguicheria: {
		nombre: "Sanguchería", descripcion: "pan con chicharrón, butifarra, hamburguesas",
		recipiente: "a plain white rectangular ceramic plate, with the sandwich sitting on it cut " +
			"in half so the filling is visible at the cut.",
		acompanamiento: "a small heap of thin crisp golden potato sticks beside the sandwich on the " +
			"same plate, and a little pot of chili sauce.",
		formatos: []string{"fuente"},
	},

	// Cero acompanamiento: inventarle guarniciones a una cocina que no conocemos
	// produce la lechuga decorativa de banco de imagenes.
	Generico: {
		nombre: "Otro", descripcion: "no encaja en ninguna de arriba",
		recipiente:     platoBlanco,
		acompanamiento: "",
		formatos:       []string{"fuente"},
	},
}

// pistas: gana el tipo que mas acumule.
var pistas = map[Tipo][]string{
	Cevicheria:   {"ceviche", "cebiche", "tiradito", "chicharron de pescado", "jalea", "leche de tigre", "conchas", "pulpo", "langostino", "calamar", "marisco", "chaufa de mariscos", "sudado"},
	Polleria:     {"pollo a la brasa", "1/4 pollo", "1/2 pollo", "broaster", "brasa", "pollo entero", "mostrito", "salchipapa"},
	Chifa:        {"chaufa", "wantan", "wanton", "tallarin saltado", "aeropuerto", "kam lu", "chi jau kay", "sillao"},
	Pizzeria:     {"pizza", "calzone", "lasagna", "lasaña", "spaghetti", "fetuccini"},
	Parrilla:     {"anticucho", "parrilla", "rachi", "pancita", "churrasco", "chorizo", "brocheta"},
	Criollo:      {"lomo saltado", "aji de gallina", "seco de", "carapulcra", "arroz con pollo", "cau cau", "tacu tacu", "papa a la huancaina", "rocoto relleno"},
	Sanguicheria: {"pan con", "butifarra", "sanguche", "sandwich", "hamburguesa"},
}

// InferirTipos propone los tipos mirando los nombres de los platos. Es una
// propuesta, no un veredicto: el dueno la cambia.
//
// Devuelve VARIOS porque un negocio peruano suele ser varios: la cevicheria del
// barrio que ademas vende pollo a la brasa. Con uno solo, media carta salia
// emplatada como no es.
//
// El corte es un tercio del ganador: un negocio que tiene DOS cocinas de verdad
// acumula pistas de las dos, y un ceviche suelto en la carta de una polleria no
// convierte al local en cevicheria.
//
// En Go y no con IA: determinista, gratis y testeable, y una carta con ceviche y
// tiradito no admite discusion.
func InferirTipos(c Carta) []Tipo {
	conteo := contarPistas(c)

	max := 0
	for _, t := range TiposConocidos {
		if conteo[t] > max {
			max = conteo[t]
		}
	}
	if max == 0 {
		return []Tipo{Generico}
	}

	corte := max / 3
	if corte < 1 {
		corte = 1
	}

	// Se recorre TiposConocidos y no el mapa: iterar un mapa es aleatorio y la
	// misma carta daria tipos distintos entre corridas. Dentro del corte se
	// ordena por conteo para que el principal quede primero.
	var fuera []Tipo
	for _, t := range TiposConocidos {
		if conteo[t] >= corte {
			fuera = append(fuera, t)
		}
	}
	sort.SliceStable(fuera, func(i, j int) bool { return conteo[fuera[i]] > conteo[fuera[j]] })
	return fuera
}

func contarPistas(c Carta) map[Tipo]int {
	conteo := map[Tipo]int{}
	for _, cat := range c.Categorias {
		anotar(conteo, cat.Nombre)
		for _, plato := range cat.Platos {
			anotar(conteo, plato.Nombre)
		}
	}
	return conteo
}

func anotar(conteo map[Tipo]int, texto string) {
	t := strings.ToLower(texto)
	for tipo, palabras := range pistas {
		for _, p := range palabras {
			if strings.Contains(t, p) {
				conteo[tipo]++
			}
		}
	}
}

// TipoDePlato elige cual de los tipos del negocio manda en ESTE plato.
//
// Es lo que hace que los varios tipos signifiquen algo: en una cevicheria-
// polleria, el ceviche se emplata con camote y choclo y el pollo con sus cuatro
// salsas. Sin esto, la mitad de la carta sale con la guarnicion de la otra.
//
// Si el plato no se parece a ninguno, manda el PRIMERO: es el principal, el que
// el dueno puso delante.
func TipoDePlato(tipos []Tipo, nombre, categoria string) Tipo {
	tipo, _ := tipoConPistas(tipos, nombre, categoria)
	return tipo
}

// tipoConPistas es TipoDePlato mas el dato que hace falta para explicarselo al
// dueno: si el plato se parecio a alguno de verdad, o si le toco el primero por
// descarte.
func tipoConPistas(tipos []Tipo, nombre, categoria string) (Tipo, bool) {
	if len(tipos) == 0 {
		return Generico, false
	}

	conteo := map[Tipo]int{}
	anotar(conteo, nombre)
	anotar(conteo, categoria)

	mejor, max := tipos[0], 0
	for _, t := range tipos {
		if conteo[t] > max {
			mejor, max = t, conteo[t]
		}
	}
	return mejor, max > 0
}

// ConteoDeTipo son los platos que le tocan a un tipo en una carta concreta.
type ConteoDeTipo struct {
	Tipo   Tipo `json:"-"`
	Platos int  `json:"-"`
}

// Reparto es lo que le pasa a la carta DE VERDAD con los tipos elegidos.
//
// Existe porque la frase de ayuda no puede ser una plantilla: "cada plato usa el
// que le toca" no le dice nada a nadie. Esto si: cuantos platos toma cada tipo,
// cuantos no se parecen a ninguno —y por tanto salen con la guarnicion del
// primero— y que tipo esta quedando fuera teniendo platos que son suyos.
//
// Se calcula con las MISMAS pistas que deciden la foto, asi que lo que dice la
// pantalla es literalmente lo que va a pasar al generar.
type Reparto struct {
	Total     int
	PorTipo   []ConteoDeTipo
	SinPistas int

	// Faltan son los tipos NO elegidos que serian el mejor tipo de algun plato.
	// Es el aviso que evita el error caro: una polleria-cevicheria con solo
	// cevicheria marcada saca 18 pollos con camote y choclo al lado.
	Faltan []ConteoDeTipo
}

// RepartirCarta reparte los platos entre los tipos elegidos.
func RepartirCarta(c Carta, elegidos []Tipo) Reparto {
	r := Reparto{PorTipo: make([]ConteoDeTipo, 0, len(elegidos))}

	tocan := map[Tipo]int{}
	fuera := map[Tipo]int{}
	elegido := map[Tipo]bool{}
	for _, t := range elegidos {
		elegido[t] = true
	}

	for _, cat := range c.Categorias {
		for _, p := range cat.Platos {
			r.Total++

			if tipo, hayPistas := tipoConPistas(elegidos, p.Nombre, cat.Nombre); hayPistas {
				tocan[tipo]++
			} else {
				r.SinPistas++
			}

			// El mejor entre TODOS los conocidos: si ese no esta elegido, es un
			// tipo que se esta quedando fuera.
			if mejor, hayPistas := tipoConPistas(TiposConocidos, p.Nombre, cat.Nombre); hayPistas && !elegido[mejor] {
				fuera[mejor]++
			}
		}
	}

	for _, t := range elegidos {
		r.PorTipo = append(r.PorTipo, ConteoDeTipo{Tipo: t, Platos: tocan[t]})
	}

	// Un plato suelto no justifica un tipo entero. El umbral sube con la carta:
	// tres platos en una de 40 dicen algo, tres en una de 200 son ruido.
	umbral := max(3, r.Total/20)
	for _, t := range TiposConocidos {
		if fuera[t] >= umbral {
			r.Faltan = append(r.Faltan, ConteoDeTipo{Tipo: t, Platos: fuera[t]})
		}
	}
	sort.SliceStable(r.Faltan, func(i, j int) bool { return r.Faltan[i].Platos > r.Faltan[j].Platos })

	return r
}

// TiposValidos limpia lo que llega de fuera: quita lo que no existe y lo
// repetido, y conserva el orden que puso el dueno.
func TiposValidos(claves []string) []Tipo {
	vistos := map[Tipo]bool{}
	var fuera []Tipo
	for _, c := range claves {
		t := Tipo(c)
		if !t.Valido() || vistos[t] {
			continue
		}
		vistos[t] = true
		fuera = append(fuera, t)
	}
	return fuera
}
