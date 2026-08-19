package app

import (
	"fmt"
	"strings"

	"tacu-backend/internal/modules/carta/domain"
)

// Este prompt nacio de un fallo: "1/4 de pollo con papas y ensalada" devolvia
// medio pollo, la ensalada encima del mismo plato y un ave de supermercado.
// De ahi las tres reglas que sostienen todo:
//
//  1. la porcion se pide como piezas anatomicas, nunca como fraccion
//  2. los recipientes se reparten uno por uno, o el modelo lo amontona todo
//  3. la identidad se describe fisicamente: "a la brasa" no significa nada
//
// LO PROHIBIDO SE DICE EN POSITIVO: los codificadores colapsan "con X" y "sin X"
// al mismo punto. La unica lista de ausencias es la ultima linea, y solo cubre
// utileria secundaria — el control de porcion NUNCA va ahi.

// plantilla es LA UNICA. Sus ranuras las rellena una domain.Receta.
//
// Antes eran tres casi identicas. Lo que cambia entre una fuente y una ronda son
// las ranuras; camara, luz y encuadre son iguales y son lo que hace que 60 fotos
// parezcan del mismo sitio. LO QUE NO TIENE RANURA ES LO QUE EL DUENO NO PUEDE
// ROMPER.
const plantilla = `Product photograph for a food delivery app menu card. Plain catalog photo of one real restaurant serving.

MAIN SUBJECT — exactly this and nothing more: %s%s%s

THE VESSEL — this is what the picture is about: %s
%s%s%s
BACKGROUND AND SURFACE: %s

CAMERA AND LIGHT: shot from a 45-degree angle, slightly above the plate, 50mm lens, f/8, everything in focus front to back. Even soft diffused light from the upper left, neutral white balance, gentle contrast, no dramatic shading. The colours of the food are rich, deep and saturated, and the light picks out wet highlights on the sauces, the marinade and the freshly fried crust, so the food reads as juicy and just served.

FRAMING: square 1:1 crop, framed close on the main plate. The main plate is the hero: it sits in the centre, fills most of the frame and is the sharpest and brightest thing in the picture. Everything else — a side bowl, a sauce dish, a second plate, a stack of small plates — is smaller, sits further back, and may be partly cut off by the edge of the frame. The camera is close enough that the food, not the empty background, occupies the picture.

HOW IT HAS TO LOOK: appetising. A customer picks this dish because of this picture. The portion is generous and full, heaped and abundant, plated by hand and slightly uneven the way a real kitchen plates it — never a small, flat, half-empty serving. Every element the dish is made of is present, plentiful and clearly visible, none of them reduced to a token pinch at the edge. The food is at its best minute: freshly made, moist, glossy, the colours vivid.

The scene contains no cutlery, no hands, no napkins, no drinks, and nothing scattered over the surface around the plate.%s`

// plantillaVajilla es la vista previa del estilo: el recipiente VACIO que el
// dueno acaba de describir, para que lo vea antes de pagar la carta entera con
// el.
//
// Comparte camara, luz y encuadre con la plantilla de los platos a proposito:
// una vista previa que no se parece a lo que va a salir no sirve de vista
// previa.
//
// Lo que NO comparte es la comida, y por eso lo dice tres veces. A un generador
// entrenado con fotos de comida se le pide un plato vacio y le pone comida
// igual: es la misma clase de sesgo que devolvia el ave entera ante "1/4 de
// pollo".
const plantillaVajilla = `Product photograph of one single EMPTY piece of restaurant tableware, photographed on its own. This is a picture of the dish-ware itself and not of a meal: there is no food and no drink anywhere in the picture.

THE PIECE — exactly this and nothing else: %s

BACKGROUND AND SURFACE: %s

CAMERA AND LIGHT: shot from a 45-degree angle, slightly above the piece, 50mm lens, f/8, everything in focus front to back. Even soft diffused light from the upper left, neutral white balance, gentle contrast, no dramatic shading. The real colour, material and texture of the piece are clearly visible.

FRAMING: square 1:1 crop. The piece sits in the centre and the WHOLE of it is inside the frame, nothing cut off at the edges, close enough that it fills most of the picture.

The piece is empty and clean: no food, no drink, no sauce, no crumbs, no garnish, no cutlery, no napkin and no hands. There is no text and no logo anywhere in the picture.`

// vajillaPorDefecto es el plato de siempre: lo que se dibuja mientras el dueno
// no ha escrito nada. Tiene que decir lo mismo que el placeholder de la
// pantalla, o la vista previa enseniaria una cosa y las fotos saldrian con otra.
const vajillaPorDefecto = "a plain white round ceramic restaurant dinner plate, the ordinary one a " +
	"Peruvian restaurant serves a main course on."

// PromptDeVajilla dibuja el recipiente de la base, vacio.
//
// Solo mira recipiente y fondo: son las dos cosas que el dueno configura. El
// texto suyo viaja en espanol sin traducir, como el ajuste de un plato.
func PromptDeVajilla(base domain.Receta) string {
	pieza := strings.TrimSpace(base.Recipiente)
	if pieza == "" {
		pieza = vajillaPorDefecto
	}
	fondo := strings.TrimSpace(base.Fondo)
	if fondo == "" {
		fondo = fondoPorDefecto
	}
	return fmt.Sprintf(plantillaVajilla, pieza, fondo)
}

// El fondo blanco de catalogo. Es lo unico del ambiente que el dueno puede
// cambiar desde su base.
const fondoPorDefecto = "plain seamless white background, empty and uniform, with a soft contact " +
	"shadow under the plate. The surface is bare."

// recetaDePlato ensambla las capas de UN plato. El orden vive en
// domain.Ensamblar.
func recetaDePlato(p domain.Plato, tipos []domain.Tipo, base domain.Receta) domain.Receta {
	// Cual de los tipos del negocio manda en ESTE plato: en una cevicheria que
	// tambien es polleria, el ceviche lleva camote y choclo y el pollo sus
	// cuatro salsas.
	tipo := domain.TipoDePlato(tipos, p.Nombre, p.Categoria)

	ajuste := domain.Receta{Ajuste: p.FotoAjuste, Referencias: p.FotoReferencias}

	r := domain.Ensamblar(tipo.Receta(), base, formatoDe(p, tipo), ajuste)
	if strings.TrimSpace(r.Fondo) == "" {
		r.Fondo = fondoPorDefecto
	}
	r.Identidad = conLaVariante(r.Identidad, p.Nombre)
	return r
}

// conLaVariante matiza lo que el banco sabe con lo que dice ESTE nombre
// impreso. Va despues de Ensamblar y no como una capa mas porque no viene de
// una tabla ni del dueno: se lee del nombre del plato, que es de donde salen ya
// el formato y la porcion.
//
// Y solo cuando hay identidad: sin banco detras, "Combo Mixto" se llevaria una
// descripcion de mariscos surtidos sin que nadie sepa si eso es un combo de
// pollo. La variante MATIZA un plato conocido, no lo inventa.
func conLaVariante(identidad, nombre string) string {
	if strings.TrimSpace(identidad) == "" {
		return identidad
	}
	v := domain.VarianteDePlato(nombre)
	if v == "" {
		return identidad
	}
	return identidad + "\n\n" + v
}

// formatoDe elige la capa de conocimiento. La porcion va primero: las dos reglas
// pueden dispararse sobre el mismo nombre.
func formatoDe(p domain.Plato, tipo domain.Tipo) domain.Receta {
	if porcion, ok := domain.PorcionDePollo(tipo, p.Nombre); ok {
		return recetaDePorcion(porcion)
	}
	if f, ok := domain.FormatoDePlato(tipo, p.Nombre, p.Categoria); ok {
		r := f.Receta
		r.Sujeto = sujetoDeFormato(f.Clave, p)
		return r
	}
	// Sin conocimiento propio, el recipiente lo pone el tipo o el dueno.
	return domain.Receta{Sujeto: "one single serving of " + sujetoSimple(p)}
}

// La identidad del ave va dentro del SUJETO: describe que ES la cosa
// fotografiada, igual que las piezas anatomicas.
func recetaDePorcion(porcion domain.Porcion) domain.Receta {
	reparto := porcion.Reparto
	if reparto == "" {
		reparto = repartoNormal
	}
	return domain.Receta{
		Sujeto:     porcion.Piezas + "\n\n" + identidadDelPolloALaBrasa,
		Recipiente: recipientePolleria,
		Reparto:    reparto,
		Escala:     porcion.Escala,
	}
}

// PromptFoto arma el prompt. El nombre comercial no llega tal cual cuando hay
// conocimiento para traducirlo: "1/4 pollo" no existe para el modelo, "una
// pierna y un muslo unidos" si.
func PromptFoto(p domain.Plato, tipos []domain.Tipo, base domain.Receta) string {
	r := recetaDePlato(p, tipos, base)

	return fmt.Sprintf(plantilla,
		sujetoConIdentidad(r),
		detalleDe(p),
		ajusteDelDueno(r.Ajuste),
		r.Recipiente,
		seccion("HOW THE FOOD SITS IN IT — ", r.Reparto),
		seccion("WHAT SURROUNDS IT — ", r.Acompanamiento),
		seccion("SCALE — ", r.Escala),
		r.Fondo,
		sinLetreros(r.Marca),
	)
}

// sinLetreros cierra la plantilla prohibiendo texto, salvo cuando la foto es de
// un producto envasado.
//
// La prohibicion existe para que el modelo no invente carteles y menus encima de
// la comida, y se queda para todo lo que cocina la casa. Pero una gaseosa la
// compra el restaurante cerrada y la revende: taparle la etiqueta seria
// fotografiar una botella generica que no es lo que el cliente va a recibir.
func sinLetreros(marca string) string {
	if strings.TrimSpace(marca) == "" {
		return " There is no text and no logo anywhere in the picture."
	}
	return " " + marca +
		" No other text and no other logo appears anywhere in the picture."
}

// sujetoConIdentidad junta CUANTO hay con QUE ES: "una fuente familiar de
// Ceviche Mixto" y, debajo, como se ve un ceviche de verdad.
//
// Van en la misma ranura porque describen la misma cosa, pero en campos
// distintos de la receta: si compartieran campo, el formato pisaria al banco al
// plegarse encima y una fuente de ceviche perderia que es un ceviche.
func sujetoConIdentidad(r domain.Receta) string {
	sujeto := strings.TrimSpace(r.Sujeto)
	identidad := strings.TrimSpace(r.Identidad)
	switch {
	case identidad == "":
		return sujeto
	case sujeto == "":
		return identidad
	}
	return sujeto + "\n\n" + identidad
}

// "always the same standard size" es lo que mantiene comparables el 1/8 y el
// 1/2, de donde sale la sensacion de porcion.
const recipientePolleria = "a plain white round ceramic restaurant dinner plate, always the same " +
	"standard size in every photo of this series."

// Pollo y papas comparten plato; la ensalada va en su bol. Escrito en una sola
// frase, el modelo lo amontonaba todo junto.
const repartoNormal = `MAIN PLATE — the chicken and the fries share this one plate. The chicken pieces sit in the middle and a generous heap of thick-cut golden french fries is piled directly beside them, on the same plate, touching. There is no salad and no sauce on this plate; the rest of the plate is bare white ceramic.

SECOND CONTAINER — separate, behind and to one side, partly inside the frame: a small plain white bowl holding fresh salad, shredded lettuce, tomato slices, red onion and a little grated carrot. The salad lives only in that bowl.`

// Descrito fisicamente y por comparacion: "a la brasa" no significa nada para el
// modelo y devolvia un rostizado de supermercado.
const identidadDelPolloALaBrasa = `WHAT THE CHICKEN LOOKS LIKE: Peruvian charcoal-roasted chicken. The bird is plump, fat and heavy-bodied, with thick meat under the skin, clearly meatier than a lean supermarket chicken. The skin is deep reddish brown, close to mahogany, clearly darker than pale golden roast chicken. The colour is even and appetising across the whole bird, warm reddish brown everywhere, with only a few small toasted spots on the wing tips and the edges. A fine dark spice rub is visible on the surface. The skin is cooked and burnished, never blackened, never burnt, never sooty. The skin is glossy and wet-looking with its own rendered juices, catching the light in bright highlights along the breast and the thighs, so the bird reads as juicy and freshly out of the oven, still dripping a little. Not dry, not matte. Always bone-in and skin-on, never sliced, never carved into fillets, never shredded.`

// Referencias devuelve las claves ya plegadas: primero las del plato.
func Referencias(p domain.Plato, tipos []domain.Tipo, base domain.Receta) []string {
	return recetaDePlato(p, tipos, base).Referencias
}

// seccion envuelve una ranura opcional sin dejar cabeceras huerfanas.
//
// Vale para reparto y escala tambien: una gaseosa no tiene reparto, y dejar
// "HOW THE FOOD SITS IN IT —" seguido de nada es ruido que el modelo lee.
func seccion(titulo, cuerpo string) string {
	if strings.TrimSpace(cuerpo) == "" {
		return ""
	}
	return "\n" + titulo + cuerpo + "\n"
}

// Para un combo es la lista de lo que trae: sin ella nadie sabe que es "COMBO 1".
func detalleDe(p domain.Plato) string {
	if d := strings.TrimSpace(p.Descripcion); d != "" {
		return ", which consists of: " + d
	}
	return ""
}

func sujetoSimple(p domain.Plato) string { return strings.TrimSpace(p.Nombre) }

// El sujeto ya dice en que llega: "Ronda Marina" por si solo no describe nada.
func sujetoDeFormato(clave string, p domain.Plato) string {
	n := sujetoSimple(p)
	switch clave {
	case "fuente":
		return "one large family-size sharing platter of " + n
	case "trio":
		return "one three-compartment sharing platter holding three different dishes: " + n
	case "ronda":
		return "one large round sectioned sharing platter holding an assortment of different " +
			"Peruvian seafood dishes, called " + n
	}
	return n
}

// Va pegado al sujeto porque lo que el dueno escribe es sobre la COMIDA, no
// sobre la camara. Se suma, nunca reemplaza.
//
// Viaja en espanol sin traducir: los dos proveedores son multilingues.
func ajusteDelDueno(texto string) string {
	t := strings.TrimSpace(texto)
	if t == "" {
		return ""
	}
	return "\n\nHOW THIS RESTAURANT SERVES IT — the owner of the restaurant describes " +
		"their own version of this dish, and where this description disagrees with the " +
		"generic one above, this one is what the photo shows: " + t
}

// PromptCorreccion es lo que se manda cuando se EDITA la foto actual en vez de
// hacer una nueva.
//
// Es corto a proposito, y esa es toda la idea: junto a la imagen, el unico
// texto es el cambio. Mandar la receta entera aqui la pondria a competir con la
// instruccion —fue exactamente el fallo de "el plato esta inclinado", que perdia
// contra el "45-degree angle" del parrafo de camara— y ademas el modelo ya tiene
// delante el plato, el recipiente y el fondo: describirselos otra vez solo le da
// permiso para cambiarlos.
func PromptCorreccion(ajuste string) string {
	return "Keep this exact photograph and change only one thing: " +
		strings.TrimSpace(ajuste) +
		". Everything else stays identical: the same dish, the same plate, the same food, " +
		"the same background, the same lighting and the same framing. Do not re-plate, " +
		"do not add or remove any food, do not change the vessel."
}
