package http

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// Publicador deja la carta visible y la sirve por su slug.
type Publicador interface {
	Ejecutar(ctx context.Context, importacionID id.ID) (string, error)
	Carta(ctx context.Context, slug string) (domain.Importacion, error)
}

type PublicadaDTO struct {
	Slug string `json:"slug"`

	// Ruta y no URL absoluta: el dominio lo sabe quien sirve la pagina, no la
	// API. Guardar aqui "https://tacu.pe/..." ataria la base al dominio de hoy.
	Ruta string `json:"ruta"`
}

func (h *Handler) montarPublicar(r fiber.Router) {
	r.Post("/importaciones/:id/publicar", h.publicar)

	// SIN autorizar: es la carta que ve el cliente del restaurante.
	r.Get("/r/:slug", h.cartaPublica)
}

func (h *Handler) publicar(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	slug, err := h.publicador.Ejecutar(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(PublicadaDTO{Slug: slug, Ruta: "/r/" + slug})
}

func (h *Handler) cartaPublica(c fiber.Ctx) error {
	imp, err := h.publicador.Carta(c.Context(), c.Params("slug"))
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(aImportacionDTO(imp, h.url, h.porFotoUSD))
}
