package http

import (
	"context"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// GestorDeReferencias maneja las fotos de ejemplo DE UN PLATO: "asi se ve mi
// ceviche". Las del restaurante —su vajilla, su fondo— ya no pasan por aqui: son
// las dos ranuras del estilo, y cada una tiene rol propio en el prompt.
type GestorDeReferencias interface {
	AgregarAPlato(ctx context.Context, platoID id.ID, bytes []byte, mime string) ([]string, error)
	QuitarDePlato(ctx context.Context, platoID id.ID, clave string) ([]string, error)
}

// ReferenciaDTO lleva la clave junto a la URL: la URL para pintarla, la clave
// para borrar esa y no otra. Juntas y no en dos listas paralelas, que se
// desalinean en cuanto una de las dos se filtra o se reordena.
type ReferenciaDTO struct {
	Clave string `json:"clave"`
	URL   string `json:"url"`
}

type ReferenciasDTO struct {
	Referencias []ReferenciaDTO `json:"referencias"`
}

func referenciasDTO(claves []string, url URLDeClave) []ReferenciaDTO {
	fuera := make([]ReferenciaDTO, 0, len(claves))
	for _, c := range claves {
		fuera = append(fuera, ReferenciaDTO{Clave: c, URL: url(c)})
	}
	return fuera
}

func (h *Handler) montarReferencias(r fiber.Router) {
	r.Post("/platos/:id/referencias", h.subirReferenciaDePlato)
	r.Delete("/platos/:id/referencias", h.quitarReferenciaDePlato)
}

func (h *Handler) subirReferenciaDePlato(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	bytes, mime, err := h.leerImagenSubida(c)
	if err != nil {
		return err
	}

	claves, err := h.referencias.AgregarAPlato(c.Context(), platoID, bytes, mime)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(ReferenciasDTO{Referencias: referenciasDTO(claves, h.url)})
}

func (h *Handler) quitarReferenciaDePlato(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	claves, err := h.referencias.QuitarDePlato(c.Context(), platoID, c.Query("clave"))
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(ReferenciasDTO{Referencias: referenciasDTO(claves, h.url)})
}

// leerImagenSubida es lo comun a subir una foto y subir una referencia.
//
// El tipo se SNIFFEA: la cabecera del formulario la pone el cliente y puede
// decir lo que quiera. Es lo que impide que un .exe renombrado a .jpg entre.
func (h *Handler) leerImagenSubida(c fiber.Ctx) ([]byte, string, error) {
	cabecera, err := c.FormFile("foto")
	if err != nil {
		return nil, "", problema(c, http.StatusBadRequest, "falta el archivo 'foto'", err.Error())
	}
	if cabecera.Size > h.maxSubidaBytes {
		return nil, "", problema(c, http.StatusBadRequest, "la imagen pesa demasiado", "")
	}

	f, err := cabecera.Open()
	if err != nil {
		return nil, "", problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}
	bytes, err := io.ReadAll(io.LimitReader(f, h.maxSubidaBytes))
	_ = f.Close()
	if err != nil {
		return nil, "", problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}

	mime := http.DetectContentType(bytes)
	if !esImagenSoportada(mime) {
		return nil, "", problema(c, http.StatusBadRequest, "el archivo no es una imagen soportada", mime)
	}
	return bytes, mime, nil
}
