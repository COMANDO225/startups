package domain

import "strings"

// VarianteDePlato saca del nombre impreso lo que el banco NO sabe.
//
// El banco describe "ceviche"; la carta dice "Ceviche Mixto". Sin esto la
// variante se tira y salen las dos fotos iguales. Medido sobre las 23 fotos de
// laboratorio: "Chicharron Mixto" salio con pescado y nada mas, cuando mixto es
// justamente que haya calamar, pescado y langostino a la vez, y "Coca Cola 1L"
// salio en la botellita de vidrio personal.
//
// Son dos ejes independientes y por eso son dos tablas: uno dice QUE lleva
// ("mixto", "a lo macho") y el otro CUANTO viene ("1L", "jarra", "1/2 doc").
// Pueden dispararse los dos sobre el mismo nombre —"Jarra de Chicha Morada"
// solo el segundo, "Ceviche Mixto" solo el primero— y dentro de cada eje gana
// el primero que encaje.
//
// Solo se aplica cuando el banco YA sabe que es el plato: pegarle "varios
// mariscos a la vez" a un "Combo Mixto" que nadie reconocio seria inventar el
// plato entero en vez de matizarlo. De eso se encarga quien llama.
func VarianteDePlato(nombre, curso string) string {
	aguja := "-" + Slug(nombre) + "-"

	var partes []string
	if v := primeraQueEncaje(aguja, curso, deContenido); v != "" {
		partes = append(partes, v)
	}
	if v := primeraQueEncaje(aguja, curso, deTamano); v != "" {
		partes = append(partes, v)
	}
	return strings.Join(partes, "\n\n")
}

func primeraQueEncaje(aguja, curso string, tabla []variante) string {
	for _, v := range tabla {
		if !v.valeEn(curso) {
			continue
		}
		for _, p := range v.patrones {
			if strings.Contains(aguja, "-"+p+"-") {
				return v.dice
			}
		}
	}
	return ""
}

type variante struct {
	patrones []string
	dice     string

	// cursos acota donde tiene sentido. Vacio = en todos.
	//
	// Hace falta porque "mixto" y "surtido" no quieren decir lo mismo en un
	// plato que en un vaso: en una cevicheria son varios mariscos, en la pizarra
	// de jugos son varias frutas. Sin esto, el "Surtido" de la seccion de jugos
	// se llevaba la descripcion de los mariscos y salia un vaso con calamares
	// dentro.
	cursos []string
}

func (v variante) valeEn(curso string) bool {
	if len(v.cursos) == 0 {
		return true
	}
	for _, c := range v.cursos {
		if c == curso {
			return true
		}
	}
	return false
}

// El eje del contenido: lo que cambia DENTRO del plato.
//
// "mixto" va primero porque es el que mas aparece y el que mas se notaba roto.
var deContenido = []variante{
	// Los dos de la pizarra de jugos van PRIMERO y acotados a bebida: gana el que
	// encaje antes, asi que un "Surtido" entre los jugos se lo lleva este y no el
	// de mariscos.
	//
	// Y son DOS, no uno: en una jugueria peruana el surtido y el especial son
	// bebidas distintas y se distinguen a simple vista. El surtido es rosa porque
	// lleva betarraga rallada; el especial es crema porque lleva leche, huevo y
	// algarrobina. Meterlos juntos era el mismo error que juntar el tiradito con
	// el tiradito a la bandera.
	{
		patrones: []string{"especial"},
		cursos:   []string{CursoBebida},
		dice: "THIS IS THE 'JUGO ESPECIAL' OF A PERUVIAN JUICE BAR, AND IT IS A " +
			"MILKSHAKE, NOT A FRUIT JUICE: papaya and banana blended with milk, a " +
			"whole raw egg and dark algarrobina syrup. It is therefore CREAM " +
			"COLOURED — a pale beige, milky and warm, never pink, never red and " +
			"never bright yellow. It is noticeably THICK, heavy and completely " +
			"opaque, so dense that it barely moves, with a thick foam collar on top " +
			"and sometimes a dark thread of algarrobina drawn over the foam.",
	},

	{
		patrones: []string{"surtido", "surtida", "mixto", "mixta"},
		cursos:   []string{CursoBebida},
		dice: "THIS ONE IS A BLEND OF SEVERAL DIFFERENT FRUITS, AND THAT IS THE " +
			"WHOLE POINT OF IT: it is not one single fruit. Peruvian juice bars " +
			"blend papaya, pineapple, banana and strawberry together with a little " +
			"grated beetroot, and it is the beetroot that gives it its colour: the " +
			"drink is an opaque pinkish rose-red, a warm magenta-salmon, and never " +
			"yellow, never orange and never beige. It is thick and completely " +
			"opaque, filling the glass to near the rim, with a fine pale froth on " +
			"top. Because it is a blend of many fruits there is NO single fruit " +
			"resting beside the glass: no wedge, no slice, no half fruit, nothing " +
			"next to it.",
	},

	{
		patrones: []string{"mixto", "mixta", "mixtos", "mixtas", "surtido", "surtida"},
		dice: "THIS ONE IS MIXED, AND THAT IS THE WHOLE POINT OF IT: it is not one single " +
			"ingredient but an assortment of several different seafoods in the same " +
			"serving, all mixed together and each one still recognisable — chunks of " +
			"white fish, whole rings of squid, plump prawns, mussels and pieces of " +
			"octopus. Because they are different creatures they come in clearly " +
			"different shapes and sizes: rings, long strips, thick squares and whole " +
			"curled prawns, all in the same heap. A serving that shows only one kind " +
			"of seafood is wrong.",
	},

	{
		patrones: []string{"conchas-negras", "concha-negra"},
		dice: "This one is made with black clams: the flesh is dark grey-purple, almost " +
			"black, and it stains the marinade a smoky grey. Their shells are thick, " +
			"ridged and black.",
	},

	{
		patrones: []string{"langostino", "langostinos"},
		dice: "This one is made with prawns: whole plump pink-orange prawns, curled, with " +
			"their tails on, big enough to be recognised one by one.",
	},

	{
		patrones: []string{"camaron", "camarones"},
		dice: "This one is made with river prawns: whole, bright orange-red once cooked, " +
			"with their heads and claws on.",
	},

	{
		patrones: []string{"calamar", "calamares"},
		dice: "This one is made with squid: white rings of even thickness, plus a few of " +
			"the little tentacle crowns.",
	},

	{
		patrones: []string{"pulpo"},
		dice: "This one is made with octopus: thick round slices with a purple-red frilled " +
			"rim and white flesh in the middle.",
	},

	{
		patrones: []string{"a-lo-macho"},
		dice: "This one is served a lo macho: the whole thing is smothered in a thick, " +
			"glossy orange-red seafood sauce, and mussels, squid rings and whole " +
			"prawns sit in the sauce on top of it.",
	},

	{
		patrones: []string{"a-la-chorrillana"},
		dice: "This one is served a la chorrillana: covered with a stew of sliced onion and " +
			"tomato cooked in red chilli, poured over it while the sauce is still wet.",
	},

	{
		patrones: []string{"a-la-crema", "a-la-huancaina"},
		dice: "This one is covered in a thick, smooth, pale creamy sauce that coats it " +
			"completely and pools a little at the base.",
	},

	{
		patrones: []string{"apanado", "apanada", "milanesa"},
		dice: "This one is breaded: a flat piece coated in golden crumbs, fried until the " +
			"crust is even and crunchy all over.",
	},

	// Va el ULTIMO: es el que menos dice, y cualquiera de los de arriba que
	// encaje en el mismo nombre lo describe mejor. "Pescado a lo Macho" tiene
	// que salir bajo su salsa, no como "solo pescado".
	//
	// Y hace falta: el banco solo tiene una entrada de chaufa y es la de
	// mariscos, asi que "Chaufa de Pescado" salia con langostinos y calamar.
	{
		patrones: []string{"pescado", "pescados", "filete"},
		dice: "This one is made with fish and only with fish: chunks or pieces of white " +
			"fish, and no prawns, no squid, no mussels and no octopus anywhere in it.",
	},
}

// El eje del tamano: cuanto viene y en que envase, que es lo que decide si sale
// la botella personal o la de litro.
var deTamano = []variante{
	{
		patrones: []string{"3l", "3-l", "3-litros"},
		dice: "SIZE — this is the biggest family bottle, three litres: a very large tall " +
			"plastic bottle with a screw cap.",
	},

	{
		patrones: []string{"15l", "1-5l", "1-5-l", "litro-y-medio"},
		dice: "SIZE — this is the large one-and-a-half-litre plastic bottle with a screw " +
			"cap, the family size.",
	},

	{
		patrones: []string{"1l", "1-l", "1lt", "1-litro", "litro", "un-litro"},
		dice: "SIZE — this is the one-litre bottle: a big tall plastic bottle with a screw " +
			"cap, roughly as tall as a forearm, the family size that is shared at the " +
			"table. It is NOT the small individual glass bottle.",
	},

	{
		patrones: []string{"medio-litro", "500ml", "600ml", "1-2-litro"},
		dice:     "SIZE — this is the half-litre bottle, the medium single-serve one.",
	},

	{
		patrones: []string{"personal", "individual"},
		dice: "SIZE — this is the small individual single-serve bottle, the one person " +
			"drinks by themselves.",
	},

	{
		patrones: []string{"jarra"},
		dice: "SIZE — this is served by the jug: one large clear glass jug filled to the " +
			"top, big enough for the whole table, not a single glass.",
	},

	{
		patrones: []string{"vaso"},
		dice:     "SIZE — this is one single tall glass, filled close to the rim.",
	},

	{
		patrones: []string{"1-2-doc", "media-docena", "6-unid"},
		dice:     "SIZE — half a dozen: exactly six pieces on the plate, countable one by one.",
	},

	{
		patrones: []string{"docena", "12-unid"},
		dice:     "SIZE — a full dozen: twelve pieces on the plate, countable one by one.",
	},
}
