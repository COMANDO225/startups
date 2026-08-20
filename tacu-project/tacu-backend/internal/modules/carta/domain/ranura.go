package domain

import "strings"

// Ranura es UNA de las dos cosas que el dueno decide de sus fotos: en que se
// sirve y sobre que. Las dos se llenan igual —lo de siempre, una descripcion, o
// una foto suya— y por eso son el mismo tipo usado dos veces.
//
// Existe porque antes eran tres entradas sueltas y desparejas: dos textos con
// ranura propia en el prompt y una bolsa de "fotos de ejemplo" SIN ROL, que
// llegaba al modelo como imagenes sin etiqueta delante del texto. El modelo
// tenia que adivinar si esa foto era el plato, el fondo o la comida —y encima la
// misma bolsa cargaba las fotos de ejemplo del plato, que dicen lo contrario.
type Ranura struct {
	// Texto es lo que escribio el dueno. Vacio = el de siempre.
	Texto string

	// Foto es la clave de SU foto. Gana sobre el texto: una foto del plato dice
	// mas que cualquier descripcion, y ademas no cuesta una generacion.
	Foto string

	// Vista es el dibujo del texto, y nada mas que eso: la vista previa de lo
	// que el dueno escribio. Cuando hay Foto no hace falta, porque la foto YA es
	// la vista.
	Vista string
}

// Tocada dice si el dueno cambio algo aqui. Lo usa la pantalla para ofrecer
// "volver al de siempre" solo cuando hay algo a lo que volver.
func (r Ranura) Tocada() bool {
	return strings.TrimSpace(r.Texto) != "" || r.Foto != ""
}

// VistaActual es la imagen que representa la ranura ahora mismo.
//
// Vacia significa "la de por defecto", que la sirve el frontend con la app y no
// cuesta nada: es la misma para todos los restaurantes, asi que generarla una
// vez por cada uno seria cobrar por lo que ya sabemos.
func (r Ranura) VistaActual() string {
	if r.Foto != "" {
		return r.Foto
	}
	return r.Vista
}

// Sobre pliega esta ranura encima de otra: lo relleno gana, lo vacio hereda. Es
// la misma regla que Receta.Sobre y por lo mismo, que una categoria pueda
// cambiar solo la foto sin perder el texto de la general.
//
// El dibujo viaja PEGADO a su texto: pertenece al texto que lo produjo, asi que
// una categoria que no redefine el texto tampoco se queda con un dibujo suyo
// colgando de la descripcion de otro.
func (r Ranura) Sobre(base Ranura) Ranura {
	fuera := base
	if strings.TrimSpace(r.Texto) != "" {
		fuera.Texto = r.Texto
		fuera.Vista = r.Vista
	}
	if r.Foto != "" {
		fuera.Foto = r.Foto
	}
	return fuera
}

// Estilo es lo que el dueno configura de sus fotos, y son exactamente dos cosas.
//
// No es una Receta: la receta son las ranuras del prompt, que rellenan cuatro
// capas —el tipo de negocio, el banco de platos, el formato y el dueno—. Esto es
// solo la capa del dueno, y se convierte en Receta al plegarse.
type Estilo struct {
	Vajilla Ranura
	Fondo   Ranura
}

// Sobre pliega un estilo de categoria encima del general, ranura por ranura.
func (e Estilo) Sobre(base Estilo) Estilo {
	return Estilo{
		Vajilla: e.Vajilla.Sobre(base.Vajilla),
		Fondo:   e.Fondo.Sobre(base.Fondo),
	}
}

// Receta convierte el estilo del dueno en la capa que entra en Ensamblar.
//
// El texto va a las ranuras de siempre y la foto a la suya, con rol. Que la foto
// tenga campo propio y no se mezcle con Referencias es TODO el arreglo: el
// prompt las nombra por posicion, y para nombrarlas hay que saber cual es cual.
func (e Estilo) Receta() Receta {
	return Receta{
		Recipiente:  e.Vajilla.Texto,
		Fondo:       e.Fondo.Texto,
		FotoVajilla: e.Vajilla.Foto,
		FotoFondo:   e.Fondo.Foto,
	}
}
