package app

import (
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// El pollo entero equivale a cuatro cuartos, o sea un plato entero de papas. Por
// eso rompe la regla de "pollo y papas comparten plato": el ave va sola en su
// plato y las papas en otro.
func TestElPolloEnteroLlevaSuPropioPlatoDePapas(t *testing.T) {
	p := PromptFoto(domain.Plato{Nombre: "1 pollo"}, []domain.Tipo{domain.Polleria}, domain.Receta{})

	if !strings.Contains(p, "SECOND PLATE") {
		t.Error("el pollo entero deberia pedir un segundo plato solo para las papas")
	}
	if !strings.Contains(p, "no fries on this plate") {
		t.Error("el plato del pollo entero deberia excluir las papas explicitamente")
	}
	if strings.Contains(p, "the chicken and the fries share this one plate") {
		t.Error("el pollo entero NO comparte plato con las papas")
	}
}

// Las porciones chicas si comparten plato, que es como se sirve en la polleria.
func TestLasPorcionesChicasCompartenPlatoConLasPapas(t *testing.T) {
	for _, n := range []string{"1/8 pollo", "1/4 pollo", "1/2 pollo"} {
		p := PromptFoto(domain.Plato{Nombre: n}, []domain.Tipo{domain.Polleria}, domain.Receta{})
		if !strings.Contains(p, "the chicken and the fries share this one plate") {
			t.Errorf("%s: deberia compartir plato con las papas", n)
		}
		if strings.Contains(p, "SECOND PLATE") {
			t.Errorf("%s: no deberia pedir un plato aparte de papas", n)
		}
	}
}

// La fraccion NUNCA puede llegar al prompt: es el bug original.
func TestElPromptNoContieneLaFraccion(t *testing.T) {
	for _, n := range []string{"1/8 pollo", "1/4 pollo", "1/2 pollo", "1 pollo"} {
		p := PromptFoto(domain.Plato{Nombre: n}, []domain.Tipo{domain.Polleria}, domain.Receta{})
		for _, mala := range []string{"1/4", "1/8", "quarter chicken", "half chicken"} {
			if strings.Contains(strings.ToLower(p), strings.ToLower(mala)) {
				t.Errorf("%s: el prompt contiene %q", n, mala)
			}
		}
	}
}

// EL FALLO DE LA FUENTE. "Fuente de Ceviche" a S/ 80 salia identica al ceviche
// individual de S/ 30 porque la plantilla generica pide, literalmente, un plato
// redondo con una sola racion.
func TestLaFuenteNoUsaPlatoIndividual(t *testing.T) {
	p := PromptFoto(domain.Plato{Nombre: "Fuente de Ceviche", Categoria: "Fuente Familiar"}, []domain.Tipo{domain.Cevicheria}, domain.Receta{})

	if !strings.Contains(p, "oval white ceramic serving platter") {
		t.Error("una fuente deberia pedir bandeja ovalada")
	}
	if strings.Contains(p, "one single serving of") {
		t.Error("una fuente NO es una racion individual: es lo que causaba el bug")
	}
	if strings.Contains(p, "round ceramic restaurant dinner plate") {
		t.Error("una fuente NO va en plato redondo de comensal")
	}
	// El ancla de escala. Sin objeto contra el que comparar, el modelo dibuja la
	// bandeja correcta del tamano equivocado — la otra mitad del fallo del pollo.
	if !strings.Contains(p, "small empty white individual plates") {
		t.Error("a la fuente le falta el ancla de escala")
	}
}

// El plato individual sigue igual: la fuente no puede contagiar al resto de la
// carta, que son la inmensa mayoria de los platos.
func TestElPlatoIndividualSigueEnPlatoRedondo(t *testing.T) {
	p := PromptFoto(domain.Plato{Nombre: "Ceviche de Pescado"}, []domain.Tipo{domain.Cevicheria}, domain.Receta{})

	if !strings.Contains(p, "one single serving of") {
		t.Error("un plato normal si es una racion individual")
	}
	if strings.Contains(p, "serving platter") {
		t.Error("un plato normal no va en bandeja de compartir")
	}
}

// EL AJUSTE DEL DUENO SE SUMA, NUNCA REEMPLAZA.
//
// Es el invariante que sostiene todo el diseno de tres capas: el dueno corrige
// la COMIDA y no puede tocar la receta fotografica. Si este test se cae, el
// cuadro de texto del lapiz se convirtio en una forma de romper el catalogo.
func TestElAjusteDelDuenoNoPisaLaRecetaFotografica(t *testing.T) {
	plato := domain.Plato{
		Nombre:     "Ceviche de Pescado",
		FotoAjuste: "el nuestro va con mas cancha y camote grueso",
	}
	p := PromptFoto(plato, []domain.Tipo{domain.Cevicheria}, domain.Receta{})

	if !strings.Contains(p, "mas cancha y camote grueso") {
		t.Fatal("el ajuste del dueno no llego al prompt")
	}

	// Lo que el dueno NO puede haber borrado al escribir su correccion.
	for _, receta := range []string{
		"45-degree angle",                 // camara
		"soft diffused light",             // luz
		"plain seamless white background", // fondo
		"square 1:1 crop",                 // encuadre
		"no cutlery, no hands",            // lo prohibido
	} {
		if !strings.Contains(p, receta) {
			t.Errorf("el ajuste del dueno se llevo por delante %q", receta)
		}
	}
}

// Sin ajuste no queda una cabecera huerfana colgando en el prompt.
func TestSinAjusteNoQuedaSeccionVacia(t *testing.T) {
	for _, n := range []string{"Ceviche de Pescado", "Fuente de Ceviche", "1/4 pollo"} {
		p := PromptFoto(domain.Plato{Nombre: n}, []domain.Tipo{domain.Polleria}, domain.Receta{})
		if strings.Contains(p, "HOW THIS RESTAURANT SERVES IT") {
			t.Errorf("%s: sin ajuste no deberia aparecer la seccion del dueno", n)
		}
	}
}

// El ajuste convive con el conocimiento de porciones: corregir la foto de un
// 1/4 de pollo no puede costarle la anatomia.
func TestElAjusteConviveConLaPorcionDePollo(t *testing.T) {
	p := PromptFoto(domain.Plato{
		Nombre:     "1/4 pollo",
		FotoAjuste: "nuestras papas son mas gruesas",
	}, []domain.Tipo{domain.Polleria}, domain.Receta{})

	if !strings.Contains(p, "papas son mas gruesas") {
		t.Error("el ajuste no llego")
	}
	if !strings.Contains(p, "drumstick and its thigh still joined") {
		t.Error("el ajuste se llevo por delante la anatomia del cuarto")
	}
}

// Un plato sin conocimiento propio cae a la plantilla generica, y ahi la
// descripcion de la carta es lo unico que lo distingue.
func TestPlatoSinConocimientoUsaSuDescripcion(t *testing.T) {
	p := PromptFoto(domain.Plato{
		Nombre:      "COMBO 1",
		Descripcion: "POLLO + PAPAS + ENSALADA + GASEOSA 1.5 L.",
	}, []domain.Tipo{domain.Polleria}, domain.Receta{})
	if !strings.Contains(p, "COMBO 1") {
		t.Error("deberia usar el nombre del plato")
	}
	if !strings.Contains(p, "POLLO + PAPAS + ENSALADA") {
		t.Error("deberia incluir lo que el combo trae: nadie compra 'COMBO 1' a ciegas")
	}
}

// --- las capas ---

// EL INVARIANTE DEL DISENO: nada de lo que el dueno edite puede borrar la receta
// fotografica. Si esto se cae, el editor de estilo rompe el catalogo.
func TestLaBaseDelDuenoNoPuedeBorrarLaRecetaFotografica(t *testing.T) {
	base := domain.Receta{
		Recipiente: "un plato de barro",
		Fondo:      "sobre una mesa de madera oscura, en un restaurante con luz de tarde",
	}
	p := PromptFoto(domain.Plato{Nombre: "Ceviche de Pescado"}, []domain.Tipo{domain.Cevicheria}, base)

	if !strings.Contains(p, "mesa de madera oscura") {
		t.Fatal("el fondo del dueno no llego al prompt")
	}
	for _, receta := range []string{
		"45-degree angle",
		"soft diffused light",
		"square 1:1 crop",
		"no cutlery, no hands",
	} {
		if !strings.Contains(p, receta) {
			t.Errorf("la base del dueno se llevo por delante %q", receta)
		}
	}
}

// El dueno cambia el plato por defecto, pero NO el de una fuente: eso es un
// hecho del plato, no una preferencia.
func TestElFormatoGanaElRecipienteSobreLaBase(t *testing.T) {
	base := domain.Receta{Recipiente: "un cuenco hondo de ceramica negra"}

	// Plato normal: manda el dueno.
	normal := PromptFoto(domain.Plato{Nombre: "Ceviche de Pescado"}, []domain.Tipo{domain.Cevicheria}, base)
	if !strings.Contains(normal, "cuenco hondo de ceramica negra") {
		t.Error("en un plato normal el recipiente del dueno deberia mandar")
	}

	// Fuente: manda el formato.
	fuente := PromptFoto(
		domain.Plato{Nombre: "Fuente de Ceviche", Categoria: "Fuente Familiar"},
		[]domain.Tipo{domain.Cevicheria}, base)
	if strings.Contains(fuente, "cuenco hondo de ceramica negra") {
		t.Error("una fuente NO puede ir en el recipiente por defecto del dueno")
	}
	if !strings.Contains(fuente, "oval white ceramic serving platter") {
		t.Error("la fuente perdio su bandeja")
	}

	// Y el fondo del dueno sobrevive en los dos casos.
	conFondo := PromptFoto(
		domain.Plato{Nombre: "Fuente de Ceviche", Categoria: "Fuente Familiar"},
		[]domain.Tipo{domain.Cevicheria}, domain.Receta{Fondo: "en la playa al atardecer"})
	if !strings.Contains(conFondo, "en la playa al atardecer") {
		t.Error("el formato se llevo por delante el fondo del dueno, que es solo suyo")
	}
}

// "Ronda" en una cevicheria es un plato de acoples; en una polleria, cervezas.
func TestElTipoDecideQueFormatosExisten(t *testing.T) {
	enCevicheria := PromptFoto(domain.Plato{Nombre: "Ronda Marina"}, []domain.Tipo{domain.Cevicheria}, domain.Receta{})
	if !strings.Contains(enCevicheria, "wedge-shaped compartments") {
		t.Error("en una cevicheria una ronda lleva acoples radiales")
	}

	enPolleria := PromptFoto(domain.Plato{Nombre: "Ronda Marina"}, []domain.Tipo{domain.Polleria}, domain.Receta{})
	if strings.Contains(enPolleria, "wedge-shaped compartments") {
		t.Error("en una polleria 'ronda' no es un plato con acoples")
	}
}

// "1/2" en una pizzeria es media pizza, no medio ave.
func TestLasFraccionesDePolloSoloAplicanEnPolleria(t *testing.T) {
	enPolleria := PromptFoto(domain.Plato{Nombre: "1/2 pollo"}, []domain.Tipo{domain.Polleria}, domain.Receta{})
	if !strings.Contains(enPolleria, "HALF A CHICKEN") {
		t.Error("en una polleria 1/2 pollo es medio ave")
	}

	enPizzeria := PromptFoto(domain.Plato{Nombre: "1/2 pollo"}, []domain.Tipo{domain.Pizzeria}, domain.Receta{})
	if strings.Contains(enPizzeria, "HALF A CHICKEN") {
		t.Error("en una pizzeria no se dibuja medio ave por leer 1/2")
	}
}

// Cada tipo emplata a su manera: si no, dos restaurantes distintos darian la
// misma foto para el mismo nombre.
func TestCadaTipoTraeSuAcompanamiento(t *testing.T) {
	cevicheria := PromptFoto(domain.Plato{Nombre: "Ceviche de Pescado"}, []domain.Tipo{domain.Cevicheria}, domain.Receta{})
	if !strings.Contains(cevicheria, "sweet potato") {
		t.Error("a una cevicheria le falta el camote")
	}
	if strings.Contains(cevicheria, "mayonnaise") {
		t.Error("una cevicheria no sirve las cuatro cremas de la polleria")
	}

	polleria := PromptFoto(domain.Plato{Nombre: "Salchipapa"}, []domain.Tipo{domain.Polleria}, domain.Receta{})
	if !strings.Contains(polleria, "mayonnaise") {
		t.Error("a una polleria le faltan las cremas, y aplican a todo lo que sirve, no solo al pollo")
	}

	// El generico no inventa guarniciones.
	generico := PromptFoto(domain.Plato{Nombre: "Algo raro"}, []domain.Tipo{domain.Generico}, domain.Receta{})
	if strings.Contains(generico, "WHAT SURROUNDS IT") {
		t.Error("sin saber de que cocina es, no se le inventan guarniciones")
	}
}

// El trio coloca cada componente en SU compartimento.
func TestElPromptDelTrioSeparaLosTresComponentes(t *testing.T) {
	p := PromptFoto(domain.Plato{
		Nombre:    "Ceviche + Arroz c/ Mariscos + Chicharrón Mixto",
		Categoria: "Tríos",
	}, []domain.Tipo{domain.Cevicheria}, domain.Receta{})

	for _, posicion := range []string{"LEFT COMPARTMENT", "MIDDLE COMPARTMENT", "RIGHT COMPARTMENT"} {
		if !strings.Contains(p, posicion) {
			t.Errorf("falta %s", posicion)
		}
	}
	if strings.Contains(p, "round ceramic restaurant dinner plate") {
		t.Error("un trio NO va en plato redondo: ese era el bug")
	}
}

// La prohibicion de letreros es fija para la comida y se levanta SOLO para el
// envase. Si esto se rompe, o salen carteles inventados sobre los platos, o las
// gaseosas vuelven a salir sin marca.
func TestSoloElProductoEnvasadoPuedeLlevarSuEtiqueta(t *testing.T) {
	plato := domain.Plato{Nombre: "Gaseosa 1L."}

	comida := PromptFoto(plato, []domain.Tipo{domain.Cevicheria}, domain.Receta{})
	if !strings.Contains(comida, "no text and no logo") {
		t.Error("la comida tiene que seguir prohibiendo letreros")
	}

	envasado := PromptFoto(plato, []domain.Tipo{domain.Cevicheria},
		domain.Receta{Marca: "keeps its own printed label"})
	if strings.Contains(envasado, "There is no text and no logo") {
		t.Error("al envasado no se le puede prohibir su propia etiqueta")
	}
	if !strings.Contains(envasado, "keeps its own printed label") {
		t.Error("la marca no llego al prompt")
	}
	// Y sigue sin poder aparecer NINGUN otro letrero.
	if !strings.Contains(envasado, "No other text and no other logo") {
		t.Error("solo la etiqueta del producto, nada mas")
	}
}

// La variante MATIZA lo que el banco sabe; no lo inventa. Sin la guarda, un
// "Combo Mixto" de una polleria se llevaria una descripcion de mariscos
// surtidos solo por decir "mixto" en el nombre.
func TestLaVarianteSoloMatizaLoQueElBancoYaConoce(t *testing.T) {
	desconocido := PromptFoto(domain.Plato{Nombre: "Combo Mixto"},
		[]domain.Tipo{domain.Cevicheria}, domain.Receta{})
	if strings.Contains(desconocido, "assortment of several different seafoods") {
		t.Error("se invento el plato a partir de una palabra del nombre")
	}

	// Con el banco detras —Identidad es lo que trae el banco— si.
	conocido := PromptFoto(domain.Plato{Nombre: "Chicharrón Mixto"},
		[]domain.Tipo{domain.Cevicheria},
		domain.Receta{Identidad: "golden fried fish chunks"})
	if !strings.Contains(conocido, "assortment of several different seafoods") {
		t.Error("la variante no llego al prompt")
	}
	if !strings.Contains(conocido, "golden fried fish chunks") {
		t.Error("la variante se llevo por delante lo que el banco sabia")
	}
}
