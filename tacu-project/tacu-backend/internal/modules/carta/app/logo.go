package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
)

var (
	// ErrSinLetrero: no se puede redibujar lo que no se ha subido.
	ErrSinLetrero = errors.New("todavia no subiste la foto de tu letrero")

	// ErrImagenAjena tiene mensaje propio y no se apoya en ErrImagenInvalida:
	// "el archivo no es una imagen soportada" no es lo que paso, y un error que
	// describe otra cosa manda a buscar el fallo donde no esta.
	ErrImagenAjena = errors.New("esa imagen no es de este restaurante")
)

// PromptDeLogo redibuja el logo que hay en una foto de un letrero.
//
// REDIBUJAR, NO INVENTAR, y ese es todo el encargo: lo que hay en ese cartel es
// la marca de otro negocio, y un logo "parecido" es una marca adulterada que el
// dueno publica sin darse cuenta.
//
// MEDIDO el 10-sep-2026 con cmd/logocheck sobre el letrero de Galponcito, una
// pizarra plastificada con reflejos: low ($0.006) saca el texto limpio pero
// deforma la mascota —el pico se le vuelve de pato—; medium ($0.053) reconstruye
// el pollo con su sombrero, su panuelo y sus guantes. Se paga el 9x porque el
// dibujo es la parte que hace reconocible un logo.
//
// Ni una ni otra salen identicas al original: esto REINTERPRETA. Por eso el
// resultado se ensena al lado del letrero y el dueno decide, en vez de
// sustituirle la marca en silencio.
const PromptDeLogo = `Redraw the logo that appears in this photograph as a clean, flat digital logo file.

This is a photograph of a real restaurant's sign, taken with a phone: it has glare, reflections, plastic wrap, uneven lighting and it is slightly skewed. Your job is to RECOVER the logo that is underneath all of that, not to design a new one.

KEEP EXACTLY, this is the whole point: the same words spelled the same way, the same letterforms, the same colours, the same mascot or emblem and the same arrangement of the parts. Someone who knows this restaurant has to recognise it instantly as their own logo.

FIX ONLY the damage of the photograph: remove the glare, the reflections and the wrinkles of the plastic, straighten it, sharpen the edges of the letters and the shapes, and rebuild the colours as flat solid areas.

The result is the logo ALONE, centred, on a plain white background, with nothing else around it: no sign, no wall, no menu, no plate, no food, no frame and no photograph of the place.

Do not add any word that is not already in the sign. Do not translate anything. Do not invent a tagline.`

// RepoLogo es lo que hace falta para la marca del negocio.
type RepoLogo interface {
	RestauranteDeImportacion(ctx context.Context, importacionID id.ID) (id.ID, error)
	MarcaDeRestaurante(ctx context.Context, restauranteID id.ID) (logo, letrero string, err error)
	GuardarLogo(ctx context.Context, restauranteID id.ID, clave string) (string, error)
	GuardarLetrero(ctx context.Context, restauranteID id.ID, clave string) (string, error)
	ReservarPresupuesto(ctx context.Context, importacionID id.ID, costo dinero.MicrosUSD) (bool, error)
}

// Redibujante saca un logo limpio de la foto de un letrero.
//
// Toma la foto como REFERENCIA, no como inspiracion. Sin ella el generador no
// redibuja nada: rellena el hueco con un logo famoso que recuerda. Comprobado —
// pidiendole el logo de una polleria peruana sin mandarle la foto devolvio "LOS
// POLLOS HERMANOS" y "TORCHY'S TACOS", marca registrada ajena incluida.
type Redibujante interface {
	Redibujar(ctx context.Context, prompt string, letrero []byte) (bytes []byte, mime string, err error)
}

// AlmacenDelLogo es lo de siempre MAS leer, y se declara aqui en vez de
// ensanchar el compartido: redibujar parte de una foto que ya esta guardada, y
// ese "leer" no lo necesita nadie mas. Quien consume declara lo que pide.
type AlmacenDelLogo interface {
	AlmacenDeImagenes
	Leer(ctx context.Context, clave string) ([]byte, string, error)
}

// Logo es la marca del negocio: el archivo que se publica y el letrero del que
// puede salir.
type Logo struct {
	repo    RepoLogo
	almacen AlmacenDelLogo
	pintor  Redibujante
	costo   dinero.MicrosUSD
}

func NuevoLogo(repo RepoLogo, almacen AlmacenDelLogo, pintor Redibujante,
	costo dinero.MicrosUSD) *Logo {
	return &Logo{repo: repo, almacen: almacen, pintor: pintor, costo: costo}
}

// Poner deja un logo como el que se publica.
//
// Es el camino GRATIS y el que gana siempre: si el dueno tiene su archivo, no
// hay nada que redibujar ni que aprobar. Tambien es como se acepta una propuesta
// —la clave ya existe en el almacen y solo hay que apuntarla en la fila—.
func (uc *Logo) Poner(ctx context.Context, impID id.ID, bytes []byte) (string, error) {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return "", err
	}
	clave, err := GuardarFoto(ctx, uc.almacen, ClaveDeLogo(restaurante), bytes)
	if err != nil {
		return "", err
	}
	return clave, uc.apuntar(ctx, restaurante, clave)
}

// Aceptar se queda con una propuesta que ya esta en el almacen.
func (uc *Logo) Aceptar(ctx context.Context, impID id.ID, clave string) error {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return err
	}
	// La clave tiene que ser de ESTE restaurante: sin comprobarlo, cualquiera
	// con un token podria colgarle a su carta el logo de otro.
	if !EsDeEsteRestaurante(clave, restaurante) {
		return ErrImagenAjena
	}
	return uc.apuntar(ctx, restaurante, clave)
}

// SubirLetrero guarda la foto del cartel. Gratis: redibujar es otra decision y
// es la que cuesta.
func (uc *Logo) SubirLetrero(ctx context.Context, impID id.ID, bytes []byte) (string, error) {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return "", err
	}
	clave, err := GuardarFoto(ctx, uc.almacen, ClaveDeLetrero(restaurante), bytes)
	if err != nil {
		return "", err
	}
	anterior, err := uc.repo.GuardarLetrero(ctx, restaurante, clave)
	if err != nil {
		return "", err
	}
	if anterior != "" && anterior != clave {
		_ = BorrarFoto(ctx, uc.almacen, anterior)
	}
	return clave, nil
}

// Redibujar saca un logo del letrero guardado y devuelve la PROPUESTA.
//
// No la apunta en la fila a proposito: el logo de un negocio no se sustituye sin
// que su dueno lo mire. Se guarda en el almacen —hace falta una URL para
// ensenarla— y solo se queda si el la acepta.
func (uc *Logo) Redibujar(ctx context.Context, impID id.ID) (string, error) {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return "", err
	}
	_, letrero, err := uc.repo.MarcaDeRestaurante(ctx, restaurante)
	if err != nil {
		return "", err
	}
	if letrero == "" {
		return "", ErrSinLetrero
	}

	// Se reserva ANTES de llamar, igual que las fotos y la vista previa del
	// estilo: comprobar despues deja que dos pestanas abiertas se pasen las dos
	// del presupuesto.
	hay, err := uc.repo.ReservarPresupuesto(ctx, impID, uc.costo)
	if err != nil {
		return "", fmt.Errorf("reservando presupuesto: %w", err)
	}
	if !hay {
		return "", ErrSinPresupuesto
	}

	datos, _, err := uc.almacen.Leer(ctx, letrero)
	if err != nil {
		return "", fmt.Errorf("leyendo el letrero: %w", err)
	}

	dibujo, _, err := uc.pintor.Redibujar(ctx, PromptDeLogo, datos)
	if err != nil {
		return "", fmt.Errorf("redibujando el logo: %w", err)
	}

	// Clave nueva en cada intento, nunca la misma sobrescrita: el navegador
	// tiene cacheada la anterior y el dueno veria la de antes creyendo que
	// redibujar no hizo nada.
	return GuardarFoto(ctx, uc.almacen, ClaveDeLogo(restaurante), dibujo)
}

// Quitar deja el catalogo sin logo, que es como salia antes.
func (uc *Logo) Quitar(ctx context.Context, impID id.ID) error {
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return err
	}
	anterior, err := uc.repo.GuardarLogo(ctx, restaurante, "")
	if err != nil {
		return err
	}
	return BorrarFoto(ctx, uc.almacen, anterior)
}

// apuntar deja la clave en la fila y borra el logo que habia.
func (uc *Logo) apuntar(ctx context.Context, restaurante id.ID, clave string) error {
	anterior, err := uc.repo.GuardarLogo(ctx, restaurante, clave)
	if err != nil {
		return err
	}
	// Igual que en la portada: si el borrado del viejo falla, el logo nuevo ya
	// esta puesto y lo que queda es un objeto de mas, no una carta rota.
	if anterior != "" && anterior != clave {
		_ = BorrarFoto(ctx, uc.almacen, anterior)
	}
	return nil
}
