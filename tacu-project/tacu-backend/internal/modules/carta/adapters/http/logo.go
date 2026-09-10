package http

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// GestorDeLogo maneja la marca del negocio: el logo que se publica y el letrero
// del que puede salir.
type GestorDeLogo interface {
	Poner(ctx context.Context, importacionID id.ID, bytes []byte) (string, error)
	Aceptar(ctx context.Context, importacionID id.ID, clave string) error
	SubirLetrero(ctx context.Context, importacionID id.ID, bytes []byte) (string, error)
	Redibujar(ctx context.Context, importacionID id.ID) (string, error)
	Quitar(ctx context.Context, importacionID id.ID) error
}

// LogoDTO viaja con la CLAVE ademas de las urls: aceptar una propuesta manda de
// vuelta cual, y la url no sirve para eso —lleva firma y tamano pegados—.
type LogoDTO struct {
	Clave      string `json:"clave,omitempty"`
	URL        string `json:"url,omitempty"`
	URLMedia   string `json:"url_media,omitempty"`
	URLPequena string `json:"url_pequena,omitempty"`
}

func aLogoDTO(clave string, url URLDeClave) LogoDTO {
	if clave == "" {
		return LogoDTO{}
	}
	p := aPortadaDTO(clave, url)
	return LogoDTO{Clave: clave, URL: p.URL, URLMedia: p.URLMedia, URLPequena: p.URLPequena}
}

func (h *Handler) montarLogo(r fiber.Router) {
	r.Post("/importaciones/:id/logo", h.ponerLogo)
	r.Put("/importaciones/:id/logo", h.aceptarLogo)
	r.Delete("/importaciones/:id/logo", h.quitarLogo)
	r.Post("/importaciones/:id/letrero", h.subirLetrero)
	r.Post("/importaciones/:id/logo/redibujar", h.redibujarLogo)
}

// ponerLogo: el dueno sube SU archivo. Gratis, y gana sobre cualquier redibujo.
func (h *Handler) ponerLogo(c fiber.Ctx) error {
	return h.conImagen(c, func(impID id.ID, bytes []byte) error {
		clave, err := h.logo.Poner(c.Context(), impID, bytes)
		if err != nil {
			return err
		}
		return c.JSON(aLogoDTO(clave, h.url))
	})
}

// subirLetrero guarda la foto del cartel. NO redibuja: eso cuesta y es otra
// decision del dueno, no un efecto de subir una foto.
func (h *Handler) subirLetrero(c fiber.Ctx) error {
	return h.conImagen(c, func(impID id.ID, bytes []byte) error {
		clave, err := h.logo.SubirLetrero(c.Context(), impID, bytes)
		if err != nil {
			return err
		}
		return c.JSON(aLogoDTO(clave, h.url))
	})
}

// redibujarLogo devuelve una PROPUESTA: no reemplaza el logo del dueno.
//
// El logo es la marca de su negocio. Sustituirla porque un modelo dibujo algo
// parecido seria adulterarla sin que el lo mire, asi que esto solo propone y es
// aceptarLogo quien se la queda.
func (h *Handler) redibujarLogo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}
	clave, err := h.logo.Redibujar(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(aLogoDTO(clave, h.url))
}

func (h *Handler) aceptarLogo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}
	var cuerpo struct {
		Clave string `json:"clave"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}
	if err := h.logo.Aceptar(c.Context(), impID, cuerpo.Clave); err != nil {
		return traducirError(c, err)
	}
	return c.JSON(aLogoDTO(cuerpo.Clave, h.url))
}

func (h *Handler) quitarLogo(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}
	if err := h.logo.Quitar(c.Context(), impID); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// conImagen es lo comun a los dos endpoints que reciben un archivo: identificar,
// autorizar y leer la imagen. Sin esto son treinta lineas repetidas donde lo
// unico que cambia es la llamada del medio.
func (h *Handler) conImagen(c fiber.Ctx, hacer func(id.ID, []byte) error) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}
	bytes, _, err := h.leerImagenSubida(c)
	if err != nil {
		return err
	}
	if err := hacer(impID, bytes); err != nil {
		return traducirError(c, err)
	}
	return nil
}
