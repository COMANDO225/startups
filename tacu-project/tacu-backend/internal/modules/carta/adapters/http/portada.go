package http

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// GestorDePortada maneja la foto del local: la que encabeza el catalogo.
type GestorDePortada interface {
	Poner(ctx context.Context, importacionID id.ID, bytes []byte) (string, error)
	Quitar(ctx context.Context, importacionID id.ID) error
}

func (h *Handler) montarPortada(r fiber.Router) {
	r.Post("/importaciones/:id/portada", h.ponerPortada)
	r.Delete("/importaciones/:id/portada", h.quitarPortada)
}

// ponerPortada guarda la foto del local del dueno.
//
// Cuelga de la importacion en la RUTA porque es el token de la importacion lo
// que autoriza, igual que todo lo demas; pero se guarda en el restaurante, que
// es de quien es.
func (h *Handler) ponerPortada(c fiber.Ctx) error {
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

	clave, err := h.portada.Poner(c.Context(), impID, bytes)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(aPortadaDTO(clave, h.url))
}

func (h *Handler) quitarPortada(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}
	if err := h.portada.Quitar(c.Context(), impID); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
