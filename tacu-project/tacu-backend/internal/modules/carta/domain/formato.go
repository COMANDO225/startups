package domain

import (
	"fmt"
	"strings"
)

// En que recipiente llega el plato. "Fuente", "trio" y "ronda" son palabras de
// negocio que el modelo no puede dibujar, asi que las ignoraba y devolvia el
// plato redondo individual:
//
//	Fuente de Ceviche  S/80  ->  identica al ceviche individual de S/30
//	Trio Marino              ->  los tres componentes revueltos en un plato redondo
//	Ronda Marina             ->  una sopa de mariscos
//
// La deteccion mira el nombre, la CATEGORIA —"Ceviche + Arroz + Chicharron" no
// dice que sea un trio, lo dice su seccion— y el TIPO de restaurante.

// Formato es una Receta parcial: solo recipiente, reparto y escala. Fondo y
// acompanamiento quedan vacios para que las capas del dueno y del tipo
// sobrevivan al plegado.
type Formato struct {
	Clave  string
	Receta Receta
}

// --- fuente ---

// Cuanto mas grande es una fuente, MEDIDO sobre la carta de La Tribuna del Sur:
//
//	Chicharron de Calamar   S/35 -> S/88   x2.51
//	Chicharron de Pescado   S/30 -> S/80   x2.67
//	Jalea de Pescado        S/30 -> S/80   x2.67
//	Ceviche Mixto           S/30 -> S/85   x2.83
//	Chicharron Mixto        S/30 -> S/85   x2.83
//	Jalea Mixta             S/30 -> S/85   x2.83
//
// De ahi sale el "roughly three times". El calculo no vive en el codigo: el
// nombre ya dice "fuente", emparejar por nombre fallo 2 de 9 veces, y el modelo
// no cuenta trozos de pescado.
var fuente = Formato{
	Clave: "fuente",
	Receta: Receta{
		Recipiente: "a single large oval white ceramic serving platter, long and elongated, the " +
			"big kind a Peruvian restaurant carries out to the middle of the table for a whole " +
			"group to share from. Its surface is far larger and much longer than a round dinner plate.",

		Reparto: "the platter is loaded: the food is heaped generously across the entire surface " +
			"of the platter from one end to the other, mounded up high in the middle and spreading " +
			"all the way out to the rim, so the food covers the platter and stands above its edge. " +
			"This is a family-size amount, enough for four to six people to share, roughly three " +
			"times what a single serving holds. The garnishes and side elements are arranged in " +
			"several separate generous piles ringing the food along the edge of the platter, and " +
			"each of those piles is large.",

		Escala: "a short stack of small empty white individual plates stands beside the platter, " +
			"further back and partly inside the frame, waiting for the group to serve themselves " +
			"onto them. The platter is more than twice as wide as one of those small plates.",
	},
}

// --- trio ---

// Los tabiques se describen explicitamente: son lo unico que separa esto de los
// tres componentes revueltos en un plato redondo.
const recipienteTrio = "a single long rectangular white ceramic serving platter divided into THREE " +
	"equal compartments set side by side in a row, like a sectioned tray. The moulded dividing " +
	"walls between the three sections stand up from the base of the platter and are clearly " +
	"visible, so each of the three foods sits in its own separate walled section and they never " +
	"touch or mix with each other."

const escalaTrio = "each of the three compartments holds a full normal restaurant serving of its " +
	"own dish, filled to the top of its walls, so the platter as a whole is a substantial meal. " +
	"The platter is about twice as long as it is wide."

// repartoTrio asigna UN componente a CADA compartimento por su nombre. Sin
// posiciones explicitas el modelo los amontona.
func repartoTrio(componentes []string) string {
	if len(componentes) != 3 {
		// La categoria dice trio pero el nombre no enumera tres cosas.
		return "each of the three compartments holds a different preparation, and the three are " +
			"visibly different from each other in colour and texture: one is a raw cured fish " +
			"preparation, one is golden deep-fried seafood, and one is a cooked rice dish. Every " +
			"compartment is full and none of the three foods spills into another."
	}

	posiciones := []string{
		"THE LEFT COMPARTMENT holds",
		"THE MIDDLE COMPARTMENT holds",
		"THE RIGHT COMPARTMENT holds",
	}
	var b strings.Builder
	for i, c := range componentes {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s one full serving of %s, filling that section on its own.", posiciones[i], c)
	}
	b.WriteString("\n\nThe three sections are visibly different from each other in colour and " +
		"texture, and each food stays inside its own walled compartment.")
	return b.String()
}

// --- ronda marina ---

// La ronda es el unico formato que enumera su comida: el surtido ES la
// definicion del plato. El molde no cambia entre restaurantes — crudo, frito,
// arroz, conchas y algo alto en el centro.
var ronda = Formato{
	Clave: "ronda",
	Receta: Receta{
		// CINCO espacios y no los que salgan: cuatro alrededor y uno en el
		// centro. Es el molde real, y sin decir el numero el generador reparte
		// seis, ocho o diez gajos y cada uno sale con una racion de probar.
		Recipiente: "a single large ROUND white ceramic platter divided into exactly FIVE spaces: " +
			"FOUR wedge-shaped compartments of the same size arranged like four quarters around " +
			"the rim, plus ONE round well in the very centre — five spaces in total and not one " +
			"more. It is the sectioned party platter a Peruvian cevicheria brings out for a table " +
			"to share. The moulded dividing walls between the four wedges stand up from the base " +
			"and are clearly visible, so each preparation sits in its own separate walled wedge.",

		Reparto: "the whole point of this dish is VARIETY AND ABUNDANCE: each of the four wedges " +
			"holds a different Peruvian seafood preparation, they all look different from each " +
			"other in colour and texture, and every one of them is heaped so full that the food " +
			"mounds up above the walls of its wedge. Nothing on this platter is a small tasting " +
			"portion.\n\n" +
			"FIRST WEDGE — ceviche: raw white fish cut in cubes, cured pale and opaque, heaped up " +
			"and topped with a thick nest of thin sliced purple-red onion, chopped coriander and " +
			"bright red chilli rings, with a thick slice of orange sweet potato and a spoonful of " +
			"big white corn kernels pressed in beside it.\n\n" +
			"SECOND WEDGE — golden deep-fried seafood piled into a high mound: crisp craggy " +
			"battered chunks of fish, whole rings of squid and prawns, deep golden brown, with a " +
			"nest of purple-red onion salsa criolla on top of the pile and a couple of thick " +
			"golden sticks of fried cassava tucked in at the side.\n\n" +
			"THIRD WEDGE — seafood rice: a generous mound of rice stained deep orange-red, with " +
			"prawns, squid rings and mussels through it and shredded white cheese over it.\n\n" +
			"FOURTH WEDGE — chinese-style fried rice, the grains loose and brown with soy sauce " +
			"and tossed with strips of omelette and sliced spring onion, clearly a different rice " +
			"from the orange one.\n\n" +
			"THE CENTRE WELL — the tall piece the whole platter is built around: a wide stemmed " +
			"glass goblet filled with thick creamy peach-orange leche de tigre, crowned with " +
			"crisp golden fried squid and a yellow plantain chip standing up out of it.\n\n" +
			"Crisp green lettuce leaves, slices of lime and a scatter of toasted golden corn fill " +
			"the gaps along the rim between the four wedges. Every one of these things sits ON " +
			"the platter itself: there are no separate little bowls standing around it on the " +
			"surface.",

		Escala: "this is a platter for two to four people to share and it is wide: a short stack of " +
			"small empty white individual plates stands beside it, further back and partly inside " +
			"the frame, and the round platter is more than twice as wide as one of those small " +
			"plates. Every single wedge is loaded to the brim and the platter looks like it can " +
			"barely hold everything on it; not one wedge is half empty.",
	},
}

// --- deteccion ---

// Como la carta dice "esto es para compartir".
var palabrasFuente = []string{"fuente", "familiar", "para compartir", "personas"}

// Componentes parte "Ceviche + Arroz + Chicharron" en sus partes. El signo mas
// es la unica pista fiable de cuantas cosas trae el plato.
func Componentes(nombre string) []string {
	if !strings.Contains(nombre, "+") {
		return nil
	}
	partes := strings.Split(nombre, "+")
	fuera := make([]string, 0, len(partes))
	for _, p := range partes {
		if p = strings.TrimSpace(p); p != "" {
			fuera = append(fuera, p)
		}
	}
	return fuera
}

// FormatoDePlato decide el recipiente. El tipo filtra, y el ORDEN importa: un
// trio de S/80 es un trio grande, no una fuente, asi que trio se comprueba
// antes.
func FormatoDePlato(tipo Tipo, nombre, categoria string) (Formato, bool) {
	n := strings.ToLower(strings.TrimSpace(nombre))
	c := strings.ToLower(strings.TrimSpace(categoria))

	if strings.Contains(n, "ronda") && tipo.PermiteFormato("ronda") {
		return ronda, true
	}

	// Un trio lo dice la cabecera de la seccion o el propio nombre. El tercer
	// caso —dos signos mas, o sea tres componentes— cubre la carta que pone los
	// combinados sueltos sin seccion propia.
	esTrio := strings.Contains(c, "trio") || strings.Contains(c, "trío") ||
		strings.Contains(n, "trio") || strings.Contains(n, "trío") ||
		len(Componentes(nombre)) == 3
	if esTrio && tipo.PermiteFormato("trio") {
		return Formato{
			Clave: "trio",
			Receta: Receta{
				Recipiente: recipienteTrio,
				Reparto:    repartoTrio(Componentes(nombre)),
				Escala:     escalaTrio,
			},
		}, true
	}

	if tipo.PermiteFormato("fuente") {
		for _, p := range palabrasFuente {
			if strings.Contains(n, p) || strings.Contains(c, p) {
				return fuente, true
			}
		}
	}
	return Formato{}, false
}
