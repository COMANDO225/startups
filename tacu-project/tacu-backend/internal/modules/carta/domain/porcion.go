package domain

import "strings"

// Este archivo existe por un fallo concreto: se le pidio al generador de imagenes
// "1/4 de pollo a la brasa" y dibujo algo del tamano de medio pollo.
//
// La causa no es el modelo. Una fraccion es una operacion aritmetica sobre un
// objeto, y el modelo no ejecuta operaciones: reconoce objetos. El objeto que
// domina sus datos para "chicken" es el ave entera asada, asi que ante una
// fraccion revierte a lo estadisticamente probable y dibuja el pollo grande de
// siempre.
//
// Lo que si son objetos, y por eso si se controlan: las piezas anatomicas
// (una pierna, un muslo, un ala) y cuanto del plato ocupan. La aritmetica la
// hacemos nosotros, una vez, aca.
//
// EL TEXTO VA EN INGLES A PROPOSITO. El resto del proyecto es en espanol, y la
// lectura de cartas tambien —ahi el material es espanol y funciona—. Pero los
// descriptores fotograficos y anatomicos tienen mucha mas densidad en ingles en
// los dos proveedores, y las palabras peruanas clave no sobreviven la
// traduccion: "retazo" no significa nada para el modelo, y "encuentro" es
// ambiguo hasta entre peruanos (en el comercio es un corte y en espanol general
// es el arranque del ala). Se describe la articulacion, no se nombra el corte.

// Porcion es una fraccion de pollo expresada en las piezas que de verdad lleva.
type Porcion struct {
	// Fraccion es como lo escribe la carta: "1/8", "1/4", "1/2", "1".
	Fraccion string

	// Piezas es el inventario anatomico que va al prompt EN LUGAR de la
	// fraccion. Es el campo que arregla el bug.
	Piezas string

	// Escala ancla el tamano contra algo visible en la foto. Sin esto el modelo
	// dibuja la pieza correcta del tamano equivocado, que era la otra mitad del
	// fallo.
	//
	// Nunca se expresa en centimetros ni gramos: se expresa como cuanto del
	// plato ocupa, cuanta loza blanca queda vacia, y como se compara con el
	// monton de papas, que es el objeto de referencia constante de la serie.
	Escala string

	// Reparto sobreescribe como se distribuye la comida entre recipientes.
	// Vacio = el reparto normal: el pollo y las papas comparten UN plato.
	//
	// Existe porque el pollo entero no sigue esa regla: equivale a cuatro
	// cuartos, o sea un plato entero de papas, y por eso el ave va sola en su
	// plato y las papas en otro. Meter esa cantidad de papas en el mismo plato
	// da un monton irreal.
	Reparto string
}

// porciones de pollo, tal como las corta una polleria peruana.
//
// Las definiciones anatomicas las dio el dueno del producto, que es peruano y
// conoce el rubro. Si alguna vez hay que corregirlas, se corrigen preguntandole
// a el o a un pollero, no buscando en internet: la investigacion no pudo abrir
// ni una carta de cadena (Norky's, Roky's, Pardos y Don Belisario son SPA y
// devolvieron paginas vacias).
var porciones = map[string]Porcion{
	"1/8": {
		Fraccion: "1/8",
		Piezas: "ONE single chicken drumstick. One piece of chicken only: the lower leg " +
			"with its bone and skin, on its own. No thigh attached to it, no wing, no breast, " +
			"no back or rib piece. Just the one drumstick.",
		Escala: "This is the smallest serving on the menu. The single drumstick is shorter " +
			"than the heap of fries beside it, covers roughly one quarter of the plate, and " +
			"more than half of the white plate stays visible and empty around it.",
	},

	// La carta dice "1/4" y no aclara cual, porque en la polleria se sirve
	// indistintamente el cuarto de pierna o el de pecho. Se toma el de pierna
	// por ser el mas comun. Si algun dia hace falta ofrecer los dos, es una
	// entrada mas en este mapa y un selector en el preview, no un cambio de
	// diseno.
	"1/4": {
		Fraccion: "1/4",
		Piezas: "ONE chicken leg quarter, kept whole as a single connected piece: the " +
			"drumstick and its thigh still joined at the knee joint, skin on, bone in, with " +
			"the small piece of back and hip still attached to the thigh. One piece of " +
			"chicken only, not two separate pieces, not pulled apart.",
		Escala: "A single-person serving. The leg quarter is about as long as the heap of " +
			"fries is wide, it covers roughly one third of the plate, and a clear band of " +
			"empty white plate stays visible around it.",
	},

	"1/2": {
		Fraccion: "1/2",
		Piezas: "HALF A CHICKEN, cut lengthwise down the bird and kept in one long connected " +
			"piece: one half breast with its wing attached at the shoulder, joined to the leg " +
			"quarter of that same side, thigh and drumstick, all skin on and bone in. One long " +
			"half-bird lying on the plate, not two pieces.",
		Escala: "This is a large, obviously bigger serving, meant for two people. The half " +
			"bird stretches almost the full width of the plate from rim to rim, it is roughly " +
			"three times the length of a single drumstick, it visibly crowds and overlaps the " +
			"heap of fries, and almost no empty white plate is left showing.",
	},

	// El pollo entero va INTACTO, no abierto ni aplanado. Lo tuve mal desde la
	// primera version: en la polleria el entero sale como salio del horno, con
	// su forma de ave, y recien se parte si el cliente lo pide.
	"1": {
		Fraccion: "1",
		Piezas: "A WHOLE ROASTED CHICKEN, the entire bird intact and in one piece, still in " +
			"its natural rounded shape with the breast facing up, the wings folded against the " +
			"body and the legs pointing back. Skin on all over. Not cut, not split, not opened " +
			"flat, not carved, not butterflied. One whole bird sitting upright on the plate.",
		Escala: "This is the family-size serving, for a whole family to share. The bird fills " +
			"its plate from rim to rim and is roughly six times the size of a single drumstick.",

		Reparto: "MAIN PLATE — the whole chicken has this plate to itself: a plain white round " +
			"ceramic serving plate, with the whole bird sitting alone in the middle of it. " +
			"There are no fries on this plate, no salad and no sauce; the bird is the only " +
			"thing on it and the rest of the plate is bare white ceramic.\n\n" +
			"SECOND PLATE — a separate plain white plate of its own, beside the chicken plate " +
			"and fully inside the frame, heaped high with a large family-size mountain of " +
			"thick-cut golden french fries, filling that whole second plate. A whole chicken " +
			"is four quarter servings, so this is four times the fries of a single portion.\n\n" +
			"THIRD CONTAINER — a small plain white bowl further back holding fresh salad, " +
			"shredded lettuce, tomato slices, red onion and a little grated carrot. The salad " +
			"lives only in that bowl.",
	},
}

// fracciones reconoce como escribe la carta cada porcion. Las pollerias no son
// consistentes: la misma carta pone "1/4 pollo", "1/4 de pollo" y "Un cuarto".
var fracciones = []struct {
	escrituras []string
	clave      string
}{
	{[]string{"1/8", "1 / 8", "un octavo", "octavo de pollo"}, "1/8"},
	{[]string{"1/4", "1 / 4", "un cuarto", "cuarto de pollo"}, "1/4"},
	{[]string{"1/2", "1 / 2", "medio pollo", "media pollo"}, "1/2"},
	{[]string{"1 pollo", "pollo entero", "pollo completo"}, "1"},
}

// PorcionDePollo detecta si el nombre de un plato es una fraccion de pollo y
// devuelve las piezas que la componen.
//
// Solo aplica a pollo: "1/2 litro" no es media ave, y "Tequenos (1/2 doc.)"
// tampoco. Por eso se exige que el nombre mencione pollo, brasa o broaster.
//
// Y solo aplica en una POLLERIA. "1/2" en una pizzeria es media pizza y en una
// cevicheria puede ser media fuente; dibujar medio ave en cualquiera de las dos
// seria el mismo error que este archivo existe para evitar, del reves.
func PorcionDePollo(tipo Tipo, nombre string) (Porcion, bool) {
	if tipo != Polleria {
		return Porcion{}, false
	}

	n := strings.ToLower(strings.TrimSpace(nombre))

	if !strings.Contains(n, "pollo") && !strings.Contains(n, "brasa") && !strings.Contains(n, "broaster") {
		return Porcion{}, false
	}

	for _, f := range fracciones {
		for _, e := range f.escrituras {
			if strings.Contains(n, e) {
				return porciones[f.clave], true
			}
		}
	}
	return Porcion{}, false
}
