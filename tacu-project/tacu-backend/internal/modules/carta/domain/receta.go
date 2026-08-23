package domain

import (
	"strings"

	"tacu-backend/internal/kernel/id"
)

// Receta son las ranuras variables de una foto. Las rellenan cuatro capas, de la
// mas general a la mas especifica: tipo de restaurante, base del dueno, formato
// del plato y ajuste del plato.
//
// Camara, luz, encuadre y lo prohibido NO tienen ranura: viven fijos en la
// plantilla para que nada de lo editable los alcance.
type Receta struct {
	// Sujeto es CUANTO hay y en que viene: "una racion", "una fuente familiar
	// de", "una pierna y un muslo". Lo escribe el formato o la porcion.
	Sujeto string

	// Identidad es QUE ES esa comida: como se ve de verdad y que no es. Lo pone
	// el banco de platos y tiene ranura propia porque si compartiera la de
	// Sujeto, el formato la borraria al plegarse encima — y una fuente de
	// ceviche necesita las dos cosas: que es una fuente Y que es un ceviche.
	//
	// Es lo mismo que ya hacia el pollo a la brasa juntando las piezas con su
	// identidad, pero para todos los platos y sin repetirlo en cada sitio.
	Identidad string

	Recipiente string
	Reparto    string // que va donde DENTRO del recipiente

	// Escala se ancla contra un objeto visible, nunca en centimetros: un
	// generador no sabe cuanto es un centimetro.
	Escala string

	// Marca, cuando no esta vacia, dice que esto es un producto envasado y su
	// etiqueta SI va en la foto. Es la unica excepcion al "no text and no logo"
	// de la plantilla, y por eso es una ranura y no un texto suelto: asi solo la
	// llena el banco, nunca el dueno.
	Marca string

	// Curso NO se escribe en el prompt: viaja para que quien matiza sepa a que
	// familia le esta hablando. Sin el, "Surtido" en la seccion de jugos recibia
	// la descripcion de un surtido de MARISCOS, y salia un vaso con calamares.
	Curso string

	Acompanamiento string // lo pone el tipo de restaurante
	Fondo          string // lo pone el dueno
	Ajuste         string

	// Referencias son CLAVES del almacen, nunca URLs ni bytes. Son las fotos de
	// ejemplo DEL PLATO: "asi se ve mi ceviche". Hablan de la comida.
	Referencias []string

	// FotoVajilla y FotoFondo son las fotos que el dueno subio de SU vajilla y
	// de SU fondo. Tienen campo propio y no van en Referencias porque el prompt
	// las nombra POR POSICION —"la primera imagen es el plato exacto"— y una
	// bolsa sin roles no se puede nombrar: el modelo recibia dos imagenes
	// mudas y tenia que adivinar si copiaba el plato, la mesa o la comida.
	FotoVajilla string
	FotoFondo   string
}

// Sobre pliega esta receta encima de otra, campo por campo: lo relleno gana, lo
// vacio hereda.
//
// Las referencias se acumulan en vez de pisarse: la foto de ejemplo del
// restaurante y la del plato dicen cosas distintas y las dos ayudan.
func (r Receta) Sobre(base Receta) Receta {
	fuera := base

	for _, c := range []struct{ nuevo, viejo *string }{
		{&r.Sujeto, &fuera.Sujeto},
		{&r.Identidad, &fuera.Identidad},
		{&r.Recipiente, &fuera.Recipiente},
		{&r.Reparto, &fuera.Reparto},
		{&r.Escala, &fuera.Escala},
		{&r.Marca, &fuera.Marca},
		{&r.Curso, &fuera.Curso},
		{&r.Acompanamiento, &fuera.Acompanamiento},
		{&r.Fondo, &fuera.Fondo},
		{&r.FotoVajilla, &fuera.FotoVajilla},
		{&r.FotoFondo, &fuera.FotoFondo},
		{&r.Ajuste, &fuera.Ajuste},
	} {
		if strings.TrimSpace(*c.nuevo) != "" {
			*c.viejo = *c.nuevo
		}
	}

	fuera.Referencias = append(append([]string{}, r.Referencias...), base.Referencias...)
	return fuera
}

// EncargoDeFoto es todo lo que hace falta para generar la foto de un plato. Base
// ya viene plegada: el worker no sabe que la jerarquia existe.
type EncargoDeFoto struct {
	Plato Plato

	// RestauranteID abre la clave del objeto: el almacen guarda por tenant. Va
	// en el encargo y no se consulta aparte porque quien lo arma ya lo tiene
	// delante —lo necesita para los tipos— y una consulta de mas por foto son 74
	// por carta.
	RestauranteID id.ID

	// Tipos son los del negocio, en orden: el primero es el principal. Cual de
	// ellos manda en ESTE plato lo decide TipoDePlato al armar la receta.
	Tipos []Tipo

	Base Receta

	// Corregir cambia lo que se le pide al modelo: en vez de una foto nueva
	// desde la receta entera, se le manda LA FOTO ACTUAL y solo el cambio.
	//
	// Hace falta porque el ajuste del dueno no siempre es sobre la comida. Ante
	// "el plato esta inclinado" la receta no puede ganar: su parrafo fijo de
	// camara dice "45-degree angle" siete parrafos mas abajo y lo contradice.
	// Editando, el unico texto es el cambio, asi que no hay con que competir.
	Corregir bool
}

// Ensamblar pliega las capas. EL ORDEN ES LA REGLA: el formato va despues de la
// base para que una fuente siga yendo en bandeja aunque el dueno tenga
// configurado plato redondo — eso es un hecho del plato, no una preferencia.
//
// El fondo del dueno sobrevive siempre porque ni el tipo ni el formato lo tocan.
func Ensamblar(tipo, base, formato, ajuste Receta) Receta {
	r := tipo
	r = base.Sobre(r)

	// El formato manda sobre el recipiente, y se lleva la foto con el: si esto
	// va en fuente, la foto del plato individual del dueno CONTRADICE al texto,
	// y de las dos cosas el modelo copia la imagen. Una foto sin su texto es
	// peor que ninguna.
	if strings.TrimSpace(formato.Recipiente) != "" {
		r.FotoVajilla = ""
	}
	r = formato.Sobre(r)
	return ajuste.Sobre(r)
}
