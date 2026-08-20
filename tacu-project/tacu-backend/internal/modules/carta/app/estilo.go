package app

import (
	"context"
	"errors"
	"fmt"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// ErrSinPresupuesto se devuelve cuando la carta ya gasto lo suyo. Es un 402 en
// el borde y no un 500: no hay nada roto, no queda dinero.
var ErrSinPresupuesto = errors.New("esta carta ya gasto su presupuesto")

// ErrRanuraDesconocida protege el unico enum que entra por la URL.
var ErrRanuraDesconocida = errors.New("esa ranura no existe")

var ErrTextoLargo = errors.New("la descripcion es demasiado larga")

// El mismo tope que maxAjuste y por la misma razon: el texto del dueno se suma a
// una plantilla que ya trae recipiente, escala, camara, luz y encuadre.
const maxTextoDeRanura = 500

// Las dos ranuras, y no hay mas. Viajan por la URL, asi que se validan al
// entrar: sin esto, ?ranura=cualquiera escribiria en la vajilla por ser el
// primer caso del switch.
const (
	RanuraVajilla = "vajilla"
	RanuraFondo   = "fondo"
)

func RanuraValida(cual string) bool {
	return cual == RanuraVajilla || cual == RanuraFondo
}

// RepoEstilo es lo que hace falta para el estilo del dueno.
type RepoEstilo interface {
	// Base devuelve el estilo YA PLEGADO: lo de la categoria sobre lo general.
	// Es lo que se dibuja y lo que se ensena.
	Base(ctx context.Context, importacionID id.ID, categoria string) ([]domain.Tipo, domain.Estilo, error)

	// EstiloPropio devuelve SOLO lo que esta categoria tiene escrito. Es sobre
	// lo que se escribe: guardar lo plegado congelaria dentro de la categoria
	// todo lo heredado, y dejaria de seguir a la general para siempre.
	EstiloPropio(ctx context.Context, importacionID id.ID, categoria string) (domain.Estilo, error)

	GuardarBase(ctx context.Context, importacionID id.ID, categoria string, e domain.Estilo) error

	// RestauranteDeImportacion abre la clave del objeto: el almacen guarda por
	// tenant.
	RestauranteDeImportacion(ctx context.Context, importacionID id.ID) (id.ID, error)

	ReservarPresupuesto(ctx context.Context, importacionID id.ID, costo dinero.MicrosUSD) (bool, error)
}

// Pintor dibuja una imagen a partir de un prompt pelado.
//
// Lo declara aqui quien lo consume: al generador de fotos de platos se le pasa
// un EncargoDeFoto entero, y una vista previa no tiene plato, ni tipos, ni
// presupuesto por plato. Lo unico que comparten es que llaman al mismo modelo.
type Pintor interface {
	Pintar(ctx context.Context, prompt string) (bytes []byte, mime string, err error)
}

// ConImportacion carga el gasto de la llamada a la carta que la pidio. Sin esto
// la vista previa se generaria sin aparecer en el libro de nadie.
type ConImportacion func(ctx context.Context, importacionID id.ID) context.Context

// EstiloUC es lo que el dueno decide de sus fotos: dos ranuras, y tres maneras
// de llenar cada una.
//
// Las tres viven juntas porque son la MISMA decision tomada por vias distintas
// —no escribir nada, describirla, o subir una foto— y solo una gana. Repartidas
// en dos casos de uso, subir una foto no podia limpiar el dibujo que la
// contradecia, y el dueno se quedaba con una vista previa que ya no era lo que
// se iba a generar.
type EstiloUC struct {
	repo     RepoEstilo
	almacen  Almacen
	pintor   Pintor
	atribuir ConImportacion
	costo    dinero.MicrosUSD
}

func NuevoEstilo(repo RepoEstilo, almacen Almacen, pintor Pintor,
	atribuir ConImportacion, costo dinero.MicrosUSD) *EstiloUC {
	return &EstiloUC{repo: repo, almacen: almacen, pintor: pintor,
		atribuir: atribuir, costo: costo}
}

// Leer devuelve el estilo plegado que hay que ensenar.
func (uc *EstiloUC) Leer(ctx context.Context, impID id.ID, categoria string) (domain.Estilo, error) {
	_, e, err := uc.repo.Base(ctx, impID, categoria)
	return e, err
}

// GuardarTextos guarda lo que el dueno escribio en las dos ranuras.
//
// Cambiar el texto TIRA su dibujo: el dibujo era la vista previa del texto
// anterior, y dejarlo puesto le enseniaria al dueno la vajilla de antes
// diciendole que es la que acaba de escribir. La foto no se toca: es la otra
// via, y quitarla es un acto aparte.
func (uc *EstiloUC) GuardarTextos(ctx context.Context, impID id.ID, categoria, vajilla, fondo string) error {
	for _, t := range []string{vajilla, fondo} {
		if len(t) > maxTextoDeRanura {
			return fmt.Errorf("%w: %d caracteres, el maximo es %d",
				ErrTextoLargo, len(t), maxTextoDeRanura)
		}
	}

	propio, err := uc.repo.EstiloPropio(ctx, impID, categoria)
	if err != nil {
		return err
	}

	if vajilla != propio.Vajilla.Texto {
		propio.Vajilla.Texto, propio.Vajilla.Vista = vajilla, ""
	}
	if fondo != propio.Fondo.Texto {
		propio.Fondo.Texto, propio.Fondo.Vista = fondo, ""
	}
	return uc.repo.GuardarBase(ctx, impID, categoria, propio)
}

// SubirFoto guarda la foto que el dueno tomo de SU vajilla o de SU fondo.
//
// Es la via barata y la mas exacta: no llama al modelo, no cuesta nada y no hay
// descripcion que pueda acercarse a la foto del plato de verdad. Por eso, al
// subirla, se lleva por delante el dibujo de esa ranura: pasa a ser ella la
// vista previa.
func (uc *EstiloUC) SubirFoto(ctx context.Context, impID id.ID, categoria, cual string,
	bytes []byte, mime string) (domain.Estilo, error) {
	if !RanuraValida(cual) {
		return domain.Estilo{}, fmt.Errorf("%w: %q", ErrRanuraDesconocida, cual)
	}

	propio, err := uc.repo.EstiloPropio(ctx, impID, categoria)
	if err != nil {
		return domain.Estilo{}, err
	}

	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return domain.Estilo{}, err
	}
	clave, err := GuardarFoto(ctx, uc.almacen, ClaveDeEstilo(restaurante, impID), bytes)
	if err != nil {
		return domain.Estilo{}, fmt.Errorf("guardando la foto del estilo: %w", err)
	}

	r := uc.ranura(&propio, cual)
	r.Foto, r.Vista = clave, ""

	if err := uc.repo.GuardarBase(ctx, impID, categoria, propio); err != nil {
		return domain.Estilo{}, err
	}
	return uc.Leer(ctx, impID, categoria)
}

// Vaciar devuelve una ranura al de siempre: sin texto, sin foto y sin dibujo.
//
// Vacia las tres a la vez porque "volver al de siempre" es una sola cosa para el
// dueno. Quitar solo la foto le dejaria un texto que el escribio hace semanas y
// que no esperaba que volviera a mandar.
func (uc *EstiloUC) Vaciar(ctx context.Context, impID id.ID, categoria, cual string) (domain.Estilo, error) {
	if !RanuraValida(cual) {
		return domain.Estilo{}, fmt.Errorf("%w: %q", ErrRanuraDesconocida, cual)
	}

	propio, err := uc.repo.EstiloPropio(ctx, impID, categoria)
	if err != nil {
		return domain.Estilo{}, err
	}
	*uc.ranura(&propio, cual) = domain.Ranura{}

	if err := uc.repo.GuardarBase(ctx, impID, categoria, propio); err != nil {
		return domain.Estilo{}, err
	}
	return uc.Leer(ctx, impID, categoria)
}

// Dibujar pinta la ranura que el dueno describio. CUESTA DINERO: una foto,
// cargada al presupuesto de esta carta.
//
// Se pide a mano y no se dispara al guardar: guardar es gratis y se hace a cada
// tecla que el dueno corrige, y cobrarle $0.0336 por cada correccion seria
// cobrarle por escribir.
//
// Dibuja lo PLEGADO y guarda en lo PROPIO: una categoria que no redefinio el
// texto tiene que dibujar el de la general, que es el que de verdad va a salir
// en sus fotos.
func (uc *EstiloUC) Dibujar(ctx context.Context, impID id.ID, categoria, cual string) (domain.Estilo, error) {
	if !RanuraValida(cual) {
		return domain.Estilo{}, fmt.Errorf("%w: %q", ErrRanuraDesconocida, cual)
	}

	plegado, err := uc.Leer(ctx, impID, categoria)
	if err != nil {
		return domain.Estilo{}, err
	}
	propio, err := uc.repo.EstiloPropio(ctx, impID, categoria)
	if err != nil {
		return domain.Estilo{}, err
	}

	// Se reserva ANTES de llamar, igual que el worker de fotos y por lo mismo:
	// comprobar despues deja que dos pestanas abiertas se pasen las dos del
	// presupuesto.
	hay, err := uc.repo.ReservarPresupuesto(ctx, impID, uc.costo)
	if err != nil {
		return domain.Estilo{}, fmt.Errorf("reservando presupuesto: %w", err)
	}
	if !hay {
		return domain.Estilo{}, ErrSinPresupuesto
	}

	prompt := PromptDeVajilla(plegado)
	if cual == RanuraFondo {
		prompt = PromptDeFondo(plegado)
	}

	// El mime que devuelve el modelo se descarta: GuardarFoto normaliza a WebP y
	// la extension la decide el paquete imagen, no el generador.
	bytes, _, err := uc.pintor.Pintar(uc.atribuir(ctx, impID), prompt)
	if err != nil {
		return domain.Estilo{}, fmt.Errorf("dibujando la ranura %s: %w", cual, err)
	}

	// Clave nueva en cada dibujo, nunca la misma sobrescrita: el navegador tiene
	// cacheada la anterior y el dueno veria la de antes creyendo que su cambio
	// no hizo nada.
	restaurante, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return domain.Estilo{}, err
	}
	clave, err := GuardarFoto(ctx, uc.almacen, ClaveDeEstilo(restaurante, impID), bytes)
	if err != nil {
		return domain.Estilo{}, fmt.Errorf("guardando el dibujo del estilo: %w", err)
	}

	// El texto plegado baja a lo propio junto con su dibujo. Van juntos o el
	// dibujo quedaria colgando de un texto que esta fila no tiene, y el dia que
	// la general cambiara, la categoria enseniaria un dibujo de otra cosa.
	r := uc.ranura(&propio, cual)
	r.Texto, r.Vista, r.Foto = uc.texto(plegado, cual), clave, ""

	if err := uc.repo.GuardarBase(ctx, impID, categoria, propio); err != nil {
		return domain.Estilo{}, err
	}
	return uc.Leer(ctx, impID, categoria)
}

// ranura devuelve un puntero a la que toque. Es lo que hace que las dos se
// traten igual en vez de duplicar cada metodo.
func (uc *EstiloUC) ranura(e *domain.Estilo, cual string) *domain.Ranura {
	if cual == RanuraFondo {
		return &e.Fondo
	}
	return &e.Vajilla
}

func (uc *EstiloUC) texto(e domain.Estilo, cual string) string {
	if cual == RanuraFondo {
		return e.Fondo.Texto
	}
	return e.Vajilla.Texto
}
