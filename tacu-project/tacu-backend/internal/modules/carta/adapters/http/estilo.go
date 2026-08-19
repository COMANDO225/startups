package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// Estilista lee y guarda como quiere el dueno que se vean sus fotos.
type Estilista interface {
	// Base devuelve el estilo YA PLEGADO para una categoria. Con categoria vacia
	// devuelve la general. Los tipos vienen ordenados: el primero es el
	// principal.
	Base(ctx context.Context, importacionID id.ID, categoria string) ([]domain.Tipo, domain.Receta, error)

	GuardarBase(ctx context.Context, importacionID id.ID, categoria string, base domain.Receta) error
	GuardarTipos(ctx context.Context, importacionID id.ID, tipos []domain.Tipo) error
}

// Vistista dibuja la vista previa del estilo: el recipiente vacio.
type Vistista interface {
	Clave(ctx context.Context, importacionID id.ID, categoria string) (string, error)
	Generar(ctx context.Context, importacionID id.ID, categoria string) (string, error)
}

type TipoDTO struct {
	Clave       string `json:"clave"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
}

type EstiloDTO struct {
	Modelo BaseDTO `json:"base"`

	// Categoria vacia = la base general del restaurante.
	Categoria string `json:"categoria"`
}

type BaseDTO struct {
	Recipiente  string   `json:"recipiente"`
	Fondo       string   `json:"fondo"`
	Referencias []string `json:"referencias"`

	// Vista es la foto del recipiente vacio. Vacia = nunca se genero una para
	// esta categoria, y la pantalla enseña la de por defecto, que es la misma
	// para todos y no cuesta nada.
	Vista string `json:"vista"`
}

func (h *Handler) montarEstilo(r fiber.Router) {
	r.Get("/importaciones/:id/estilo", h.obtenerEstilo)
	r.Put("/importaciones/:id/estilo", h.guardarEstilo)
	r.Post("/importaciones/:id/estilo/vista", h.generarVistaDeEstilo)
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
	_, base, err := h.estilo.Base(c.Context(), impID, categoria)
	if err != nil {
		return traducirError(c, err)
	}
	vista, err := h.vista.Clave(c.Context(), impID, categoria)
	if err != nil {
		return traducirError(c, err)
	}

	return c.JSON(EstiloDTO{
		Categoria: categoria,
		Modelo: BaseDTO{
			Recipiente:  base.Recipiente,
			Fondo:       base.Fondo,
			Referencias: urls(base.Referencias, h.url),
			Vista:       urlOVacia(vista, h.url),
		},
	})
}

// generarVistaDeEstilo dibuja el recipiente vacio que el dueno acaba de
// describir. CUESTA DINERO: una foto, cargada al presupuesto de esta carta.
//
// Va en su propio POST y no dentro del PUT del estilo porque guardar es gratis
// y se hace a cada correccion; generar se pide.
func (h *Handler) generarVistaDeEstilo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	clave, err := h.vista.Generar(c.Context(), impID, c.Query("categoria"))
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(fiber.Map{"vista": h.url(clave)})
}

// urlOVacia deja pasar el vacio en vez de armar la URL de la clave "": el
// frontend distingue "no hay vista" de "hay una", y una URL que apunta a nada
// se veria como una imagen rota.
func urlOVacia(clave string, url URLDeClave) string {
	if clave == "" {
		return ""
	}
	return url(clave)
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
		Recipiente string `json:"recipiente"`
		Fondo      string `json:"fondo"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	// Las referencias NO entran por aqui: se suben y se borran una a una con su
	// propio endpoint, y mezclarlas en este PUT las borraria cada vez que el
	// dueno solo quisiera cambiar el fondo.
	_, base, err := h.estilo.Base(c.Context(), impID, c.Query("categoria"))
	if err != nil {
		return traducirError(c, err)
	}
	base.Recipiente = cuerpo.Recipiente
	base.Fondo = cuerpo.Fondo

	if err := h.estilo.GuardarBase(c.Context(), impID, c.Query("categoria"), base); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
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
	if err := h.estilo.GuardarTipos(c.Context(), impID, tipos); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func urls(claves []string, url URLDeClave) []string {
	fuera := make([]string, 0, len(claves))
	for _, c := range claves {
		fuera = append(fuera, url(c))
	}
	return fuera
}
