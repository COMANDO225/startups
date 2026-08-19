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
func VarianteDePlato(nombre string) string {
	aguja := "-" + Slug(nombre) + "-"

	var partes []string
	if v := primeraQueEncaje(aguja, deContenido); v != "" {
		partes = append(partes, v)
	}
	if v := primeraQueEncaje(aguja, deTamano); v != "" {
		partes = append(partes, v)
	}
	return strings.Join(partes, "\n\n")
}

func primeraQueEncaje(aguja string, tabla []variante) string {
	for _, v := range tabla {
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
}

// El eje del contenido: lo que cambia DENTRO del plato.
//
// "mixto" va primero porque es el que mas aparece y el que mas se notaba roto.
var deContenido = []variante{
	{[]string{"mixto", "mixta", "mixtos", "mixtas", "surtido", "surtida"},
		"THIS ONE IS MIXED, AND THAT IS THE WHOLE POINT OF IT: it is not one single " +
			"ingredient but an assortment of several different seafoods in the same " +
			"serving, all mixed together and each one still recognisable — chunks of " +
			"white fish, whole rings of squid, plump prawns, mussels and pieces of " +
			"octopus. Because they are different creatures they come in clearly " +
			"different shapes and sizes: rings, long strips, thick squares and whole " +
			"curled prawns, all in the same heap. A serving that shows only one kind " +
			"of seafood is wrong."},

	{[]string{"conchas-negras", "concha-negra"},
		"This one is made with black clams: the flesh is dark grey-purple, almost " +
			"black, and it stains the marinade a smoky grey. Their shells are thick, " +
			"ridged and black."},

	{[]string{"langostino", "langostinos"},
		"This one is made with prawns: whole plump pink-orange prawns, curled, with " +
			"their tails on, big enough to be recognised one by one."},

	{[]string{"camaron", "camarones"},
		"This one is made with river prawns: whole, bright orange-red once cooked, " +
			"with their heads and claws on."},

	{[]string{"calamar", "calamares"},
		"This one is made with squid: white rings of even thickness, plus a few of " +
			"the little tentacle crowns."},

	{[]string{"pulpo"},
		"This one is made with octopus: thick round slices with a purple-red frilled " +
			"rim and white flesh in the middle."},

	{[]string{"a-lo-macho"},
		"This one is served a lo macho: the whole thing is smothered in a thick, " +
			"glossy orange-red seafood sauce, and mussels, squid rings and whole " +
			"prawns sit in the sauce on top of it."},

	{[]string{"a-la-chorrillana"},
		"This one is served a la chorrillana: covered with a stew of sliced onion and " +
			"tomato cooked in red chilli, poured over it while the sauce is still wet."},

	{[]string{"a-la-crema", "a-la-huancaina"},
		"This one is covered in a thick, smooth, pale creamy sauce that coats it " +
			"completely and pools a little at the base."},

	{[]string{"apanado", "apanada", "milanesa"},
		"This one is breaded: a flat piece coated in golden crumbs, fried until the " +
			"crust is even and crunchy all over."},

	// Va el ULTIMO: es el que menos dice, y cualquiera de los de arriba que
	// encaje en el mismo nombre lo describe mejor. "Pescado a lo Macho" tiene
	// que salir bajo su salsa, no como "solo pescado".
	//
	// Y hace falta: el banco solo tiene una entrada de chaufa y es la de
	// mariscos, asi que "Chaufa de Pescado" salia con langostinos y calamar.
	{[]string{"pescado", "pescados", "filete"},
		"This one is made with fish and only with fish: chunks or pieces of white " +
			"fish, and no prawns, no squid, no mussels and no octopus anywhere in it."},
}

// El eje del tamano: cuanto viene y en que envase, que es lo que decide si sale
// la botella personal o la de litro.
var deTamano = []variante{
	{[]string{"3l", "3-l", "3-litros"},
		"SIZE — this is the biggest family bottle, three litres: a very large tall " +
			"plastic bottle with a screw cap."},

	{[]string{"15l", "1-5l", "1-5-l", "litro-y-medio"},
		"SIZE — this is the large one-and-a-half-litre plastic bottle with a screw " +
			"cap, the family size."},

	{[]string{"1l", "1-l", "1lt", "1-litro", "litro", "un-litro"},
		"SIZE — this is the one-litre bottle: a big tall plastic bottle with a screw " +
			"cap, roughly as tall as a forearm, the family size that is shared at the " +
			"table. It is NOT the small individual glass bottle."},

	{[]string{"medio-litro", "500ml", "600ml", "1-2-litro"},
		"SIZE — this is the half-litre bottle, the medium single-serve one."},

	{[]string{"personal", "individual"},
		"SIZE — this is the small individual single-serve bottle, the one person " +
			"drinks by themselves."},

	{[]string{"jarra"},
		"SIZE — this is served by the jug: one large clear glass jug filled to the " +
			"top, big enough for the whole table, not a single glass."},

	{[]string{"vaso"},
		"SIZE — this is one single tall glass, filled close to the rim."},

	{[]string{"1-2-doc", "media-docena", "6-unid"},
		"SIZE — half a dozen: exactly six pieces on the plate, countable one by one."},

	{[]string{"docena", "12-unid"},
		"SIZE — a full dozen: twelve pieces on the plate, countable one by one."},
}
