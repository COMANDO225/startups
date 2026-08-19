package domain

import "strings"

// PlatoTipico es lo que el banco sabe de un plato: que es y como se sirve.
//
// Existe porque hasta ahora la guarnicion la ponia el TIPO DE NEGOCIO, y eso
// pone camote y choclo alrededor de los 74 platos de una cevicheria — el sudado,
// el postre y la gaseosa incluidos. El camote es del CEVICHE, no de la
// cevicheria.
type PlatoTipico struct {
	Clave  string
	Nombre string
	Cocina string
	Curso  string

	// Los cuatro que deciden la foto, en ingles como el resto de descriptores.
	// Vacios mientras nadie los haya escrito y medido: entonces manda el curso.
	Aspecto    string
	Recipiente string
	Guarnicion string
	Jamas      string

	// Envasado marca lo que el restaurante COMPRA y revende cerrado: una
	// gaseosa, una cerveza. Se fotografia en su envase y CON su marca, que es
	// justo lo que la plantilla prohibe para la comida.
	Envasado bool

	// Patrones son los trozos de nombre impreso que disparan este plato, ya
	// normalizados como slug.
	Patrones []string

	// Ingredientes es lo que trajo la cosecha. No entra en el prompt de la foto
	// —un generador no dibuja una lista de la compra— pero es el CONTEXTO con el
	// que se le pide al modelo que describa un plato que todavia nadie describio.
	Ingredientes []string
}

// Los cursos que trajo la cosecha.
const (
	CursoEntrada    = "entrada"
	CursoFondo      = "fondo"
	CursoSopa       = "sopa"
	CursoPostre     = "postre"
	CursoBebida     = "bebida"
	CursoGuarnicion = "guarnicion"
)

// recetasDeCurso es el suelo: lo minimo para no fotografiar un plato como lo que
// no es, sin haber escrito una sola descripcion todavia.
//
// Son frases POSITIVAS y no ranuras vacias a proposito. Receta.Sobre hereda lo
// vacio, asi que un "" no borra la guarnicion de la cocina: hay que decir lo que
// SI hay. Y a un generador le funciona mejor que le digan lo que hay que lo que
// no.
//
// entrada, fondo y otro no aparecen: ahi la cocina sigue mandando, que para eso
// una entrada de cevicheria SI lleva lo de la cevicheria.
var recetasDeCurso = map[string]Receta{
	CursoBebida: {
		Recipiente: "a tall clear drinking glass, or the sealed bottle it is sold in, " +
			"standing upright on the surface.",
		Acompanamiento: "the drink stands completely on its own. There is no food, no plate " +
			"and no side dish anywhere in the picture: only the glass or the bottle.",
	},

	CursoSopa: {
		Recipiente: "a wide deep white ceramic soup bowl, the broth filling it up close to the " +
			"rim so the surface of the liquid and what floats in it are both clearly visible.",
		Acompanamiento: "the bowl stands on its own. Only the soup and the pieces cooked inside " +
			"it are in the picture; nothing is arranged around the bowl.",
	},

	CursoPostre: {
		Recipiente: "a small white dessert plate, or a clear glass cup when the sweet is soft " +
			"or poured.",
		Acompanamiento: "the dessert stands on its own. There is no savoury food and no salad " +
			"around it.",
	},

	CursoGuarnicion: {
		Acompanamiento: "the side dish stands on its own in its own small dish, with nothing " +
			"else around it.",
	},
}

// Receta traduce lo que el banco sabe a las ranuras de la foto: primero el suelo
// del curso y encima lo escrito para ESE plato.
func (p PlatoTipico) Receta() Receta {
	propia := Receta{
		Identidad:      p.identidad(),
		Recipiente:     p.Recipiente,
		Acompanamiento: p.Guarnicion,
	}
	if p.Envasado {
		propia.Marca = marcaVisible
	}
	return propia.Sobre(recetasDeCurso[p.Curso])
}

// ConLaBaseDelDueno pliega el estilo que configuro el dueno sobre lo que el
// banco sabe de este plato.
//
// EL RECIPIENTE DEL DUENO SOLO VALE PARA LO QUE VA EN PLATO. Antes se plegaba
// campo por campo y siempre ganaba el dueno, asi que quien escribia "plato
// redondo blanco" en su estilo conseguia el cafe vertido en un plato blanco y
// la gaseosa servida en la vajilla del restaurante. La vajilla es una
// preferencia; que una bebida vaya en vaso es un hecho.
//
// El fondo y las fotos de ejemplo del dueno sobreviven en los dos casos: de eso
// el banco no dice nada, asi que no hay con que pisarlos.
func (p PlatoTipico) ConLaBaseDelDueno(base Receta) Receta {
	if p.VaEnPlato() {
		return base.Sobre(p.Receta())
	}
	return p.Receta().Sobre(base)
}

// VaEnPlato dice si este plato acepta la vajilla del dueno.
//
// Una bebida va en su vaso o en su botella, una sopa en bol hondo y un postre
// en su copa: en los tres el recipiente es parte de lo que ES, no de como le
// gusta servir al restaurante. Lo que se sirve en plato —una entrada, un fondo,
// una guarnicion, una fuente— si es suyo.
//
// Sin curso, o sea un plato que no esta en el banco, manda el dueno: es
// exactamente lo que pasaba antes de que el banco existiera.
func (p PlatoTipico) VaEnPlato() bool {
	switch p.Curso {
	case CursoBebida, CursoSopa, CursoPostre:
		return false
	}
	return true
}

// marcaVisible es lo unico que levanta el "no text and no logo" de la plantilla,
// y solo para el envase.
//
// La comida sigue sin poder llevar carteles: lo que se permite es la etiqueta
// que el producto YA trae de fabrica, porque el restaurante lo compro asi y asi
// lo vende.
const marcaVisible = "this is a sealed commercial product exactly as the shop resells it: it keeps " +
	"its own printed label and brand on the bottle, can or packet, legible and undamaged, " +
	"the way it looks on the shelf. The brand is the one named in the dish."

// identidad junta como SE VE con lo que NO es. Van pegados porque los dos describen
// la misma cosa, igual que en el pollo a la brasa: "never sliced, never carved"
// vive dentro de su identidad, no en un parrafo aparte.
func (p PlatoTipico) identidad() string {
	aspecto := strings.TrimSpace(p.Aspecto)
	jamas := strings.TrimSpace(p.Jamas)
	switch {
	case aspecto == "":
		return ""
	case jamas == "":
		return aspecto
	}
	return aspecto + "\n\n" + jamas
}

// Descrito dice si a este plato ya le escribieron como se ve. Lo cosechado nace
// sin describir y sirve igual —trae su curso—, pero no puede taparle el sitio a
// una entrada del canon que si esta descrita.
func (p PlatoTipico) Descrito() bool { return strings.TrimSpace(p.Aspecto) != "" }

// EmparejarTipico busca a que plato del banco corresponde un nombre impreso.
//
// Gana el patron mas LARGO, para que "chicharron de pescado" no se lo lleve
// "chicharron". Y a igualdad gana el DESCRITO: "Ceviche Clasico" salio de la
// cosecha sin descripcion y su patron es mas largo que "ceviche", asi que sin
// esta regla le robaria el plato a la entrada del canon que si sabe como se ve
// un ceviche.
//
// La comparacion es por TOKENS y no por subcadena: sin los guiones de los
// extremos, el patron "pan" emparejaria "panqueques".
func EmparejarTipico(nombre string, banco []PlatoTipico) (PlatoTipico, bool) {
	aguja := "-" + Slug(nombre) + "-"

	var mejor PlatoTipico
	mejorLargo := 0
	encontrado := false

	for _, t := range banco {
		for _, p := range t.Patrones {
			p = strings.TrimSpace(p)
			if p == "" || !strings.Contains(aguja, "-"+p+"-") {
				continue
			}
			if !encontrado || prefiere(t, len(p), mejor, mejorLargo) {
				mejor, mejorLargo, encontrado = t, len(p), true
			}
		}
	}
	return mejor, encontrado
}

// prefiere decide entre dos candidatos: primero manda estar DESCRITO y solo
// entre iguales gana el patron mas largo.
//
// En este orden y no al reves: "ceviche-clasico" es mas largo que "ceviche" pero
// salio de la cosecha sin saber como se ve un ceviche, asi que ganar por
// longitud seria cambiar conocimiento por precision de nombre.
func prefiere(nuevo PlatoTipico, largoNuevo int, viejo PlatoTipico, largoViejo int) bool {
	if nuevo.Descrito() != viejo.Descrito() {
		return nuevo.Descrito()
	}
	return largoNuevo > largoViejo
}

// Quien decidio que plato tipico es este. El del dueno no lo pisa nadie: si
// corrigio a mano, una ampliacion del banco no puede deshacerlo.
const (
	TipicoPorRegla = "regla"
	TipicoPorIA    = "ia"
	TipicoDelDueno = "dueno"
)

// EmparejarConElBanco le pone a cada plato de la carta su clave del banco.
//
// Se llama UNA vez, al leer la carta, y el resultado se guarda. Emparejar en
// cada foto seria repetir el mismo trabajo, y ademas dejaria que la foto de un
// plato cambiara sola el dia que alguien amplie el banco.
//
// Devuelve cuantos se emparejaron: es lo que dice si el banco esta cubriendo
// esta carta o se le esta escapando entera.
func (c *Carta) EmparejarConElBanco(banco []PlatoTipico) int {
	emparejados := 0
	for i := range c.Categorias {
		for j := range c.Categorias[i].Platos {
			p := &c.Categorias[i].Platos[j]
			if p.TipicoOrigen == TipicoDelDueno {
				continue
			}
			t, ok := EmparejarTipico(p.Nombre, banco)
			if !ok {
				continue
			}
			p.Tipico, p.TipicoOrigen = t.Clave, TipicoPorRegla
			emparejados++
		}
	}
	return emparejados
}
