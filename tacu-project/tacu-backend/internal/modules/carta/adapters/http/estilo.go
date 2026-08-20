package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
)

// Estilista es lo que el dueno decide de sus fotos: dos ranuras —en que sirve y
// sobre que— y tres maneras de llenar cada una.
//
// Todo lo que escribe devuelve el estilo entero ya plegado: la pantalla ensena
// las dos ranuras a la vez, y con un 204 tendria que volver a pedirlo para
// pintar lo que acaba de hacer.
type Estilista interface {
	Leer(ctx context.Context, impID id.ID, categoria string) (domain.Estilo, error)
	GuardarTextos(ctx context.Context, impID id.ID, categoria, vajilla, fondo string) error
	SubirFoto(ctx context.Context, impID id.ID, categoria, ranura string, bytes []byte, mime string) (domain.Estilo, error)
	Vaciar(ctx context.Context, impID id.ID, categoria, ranura string) (domain.Estilo, error)
	Dibujar(ctx context.Context, impID id.ID, categoria, ranura string) (domain.Estilo, error)
}

// Negocio son los tipos del restaurante. Aparte del estilo a proposito: el tipo
// de negocio no es una preferencia de foto, es lo que el sitio ES, y decide
// tambien la guarnicion y el orden de las categorias.
type Negocio interface {
	Base(ctx context.Context, importacionID id.ID, categoria string) ([]domain.Tipo, domain.Estilo, error)
	GuardarTipos(ctx context.Context, importacionID id.ID, tipos []domain.Tipo) error
}

type TipoDTO struct {
	Clave       string `json:"clave"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
}

// RanuraDTO es lo que la pantalla necesita para pintar UNA ranura, y nada mas.
//
// Imagen ya viene resuelta —la foto suya si la hay, si no el dibujo, si no
// vacia— porque la precedencia es una regla del dominio y reimplementarla en
// TypeScript seria tener dos versiones de ella esperando a divergir. Vacia
// significa "la de por defecto", que la sirve el frontend con la app.
type RanuraDTO struct {
	Texto  string `json:"texto"`
	Imagen string `json:"imagen"`
	Propia bool   `json:"propia"`
	Tocada bool   `json:"tocada"`
}

type EstiloDTO struct {
	// Categoria vacia = la base general del restaurante.
	Categoria string `json:"categoria"`

	Vajilla RanuraDTO `json:"vajilla"`
	Fondo   RanuraDTO `json:"fondo"`
}

func (h *Handler) montarEstilo(r fiber.Router) {
	r.Get("/importaciones/:id/estilo", h.obtenerEstilo)
	r.Put("/importaciones/:id/estilo", h.guardarEstilo)
	r.Post("/importaciones/:id/estilo/:ranura/dibujo", h.dibujarRanura)
	r.Post("/importaciones/:id/estilo/:ranura/foto", h.subirFotoDeRanura)
	r.Delete("/importaciones/:id/estilo/:ranura", h.vaciarRanura)
	r.Get("/importaciones/:id/tipos", h.obtenerTipos)
	r.Put("/importaciones/:id/tipos", h.guardarTipos)
}

func (h *Handler) obtenerEstilo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	categoria := c.Query("categoria")
	e, err := h.estilo.Leer(c.Context(), impID, categoria)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(h.estiloDTO(categoria, e))
}

func (h *Handler) guardarEstilo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	var cuerpo struct {
		Vajilla string `json:"vajilla"`
		Fondo   string `json:"fondo"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	// Las fotos NO entran por aqui: se suben y se quitan con su propia ruta.
	// Mezclarlas en este PUT las borraria cada vez que el dueno solo quisiera
	// corregir una palabra del texto.
	categoria := c.Query("categoria")
	if err := h.estilo.GuardarTextos(c.Context(), impID, categoria,
		strings.TrimSpace(cuerpo.Vajilla), strings.TrimSpace(cuerpo.Fondo)); err != nil {
		return traducirError(c, err)
	}

	e, err := h.estilo.Leer(c.Context(), impID, categoria)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(h.estiloDTO(categoria, e))
}

// dibujarRanura pinta lo que el dueno escribio. CUESTA DINERO: una foto, cargada
// al presupuesto de esta carta.
//
// Va en su propia ruta y no dentro del PUT porque guardar es gratis y se hace a
// cada correccion; dibujar se pide.
func (h *Handler) dibujarRanura(c fiber.Ctx) error {
	return h.conRanura(c, func(impID id.ID, categoria, ranura string) (domain.Estilo, error) {
		return h.estilo.Dibujar(c.Context(), impID, categoria, ranura)
	})
}

// subirFotoDeRanura guarda la foto que el dueno tomo de su plato o de su mesa.
// Es gratis y es instantanea: no pasa por el modelo.
func (h *Handler) subirFotoDeRanura(c fiber.Ctx) error {
	bytes, mime, err := h.leerImagenSubida(c)
	if err != nil {
		return err
	}
	return h.conRanura(c, func(impID id.ID, categoria, ranura string) (domain.Estilo, error) {
		return h.estilo.SubirFoto(c.Context(), impID, categoria, ranura, bytes, mime)
	})
}

func (h *Handler) vaciarRanura(c fiber.Ctx) error {
	return h.conRanura(c, func(impID id.ID, categoria, ranura string) (domain.Estilo, error) {
		return h.estilo.Vaciar(c.Context(), impID, categoria, ranura)
	})
}

// conRanura es el preambulo comun de las tres: id, permiso, ranura valida, y la
// misma respuesta.
//
// La ranura se valida AQUI ademas de en el caso de uso: viene de la URL, y una
// ranura que no existe es un 400 del cliente, no un 500 nuestro.
func (h *Handler) conRanura(c fiber.Ctx,
	hacer func(impID id.ID, categoria, ranura string) (domain.Estilo, error)) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	ranura := c.Params("ranura")
	if !app.RanuraValida(ranura) {
		return problema(c, http.StatusBadRequest, "esa ranura no existe", ranura)
	}

	categoria := c.Query("categoria")
	e, err := hacer(impID, categoria, ranura)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(h.estiloDTO(categoria, e))
}

func (h *Handler) estiloDTO(categoria string, e domain.Estilo) EstiloDTO {
	return EstiloDTO{
		Categoria: categoria,
		Vajilla:   h.ranuraDTO(e.Vajilla),
		Fondo:     h.ranuraDTO(e.Fondo),
	}
}

func (h *Handler) ranuraDTO(r domain.Ranura) RanuraDTO {
	return RanuraDTO{
		Texto:  r.Texto,
		Imagen: urlOVacia(r.VistaActual(), h.url),
		Propia: r.Foto != "",
		Tocada: r.Tocada(),
	}
}

// urlOVacia deja pasar el vacio en vez de armar la URL de la clave "": el
// frontend distingue "no hay imagen" —y ensena la de por defecto, que viaja con
// la app y no cuesta nada— de "hay una", y una URL que apunta a nada se veria
// como una imagen rota.
func urlOVacia(clave string, url URLDeClave) string {
	if clave == "" {
		return ""
	}
	return url(clave)
}

// guardarTipos guarda los tipos de negocio. Son varios porque un negocio
// peruano suele ser varios: la cevicheria que tambien vende pollo a la brasa.
func (h *Handler) guardarTipos(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	var cuerpo struct {
		Tipos []string `json:"tipos"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	// TiposValidos descarta lo que no existe y lo repetido. Se compara el
	// tamano para no aceptar en silencio una lista con basura dentro.
	tipos := domain.TiposValidos(cuerpo.Tipos)
	if len(tipos) != len(cuerpo.Tipos) {
		return problema(c, http.StatusBadRequest,
			"hay un tipo de negocio que no existe o esta repetido", strings.Join(cuerpo.Tipos, ", "))
	}
	if err := h.negocio.GuardarTipos(c.Context(), impID, tipos); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
