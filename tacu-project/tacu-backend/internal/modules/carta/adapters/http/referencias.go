package http

import (
	"context"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// GestorDeReferencias maneja las fotos de ejemplo que guian la generacion.
type GestorDeReferencias interface {
	AgregarAPlato(ctx context.Context, platoID id.ID, bytes []byte, mime string) ([]string, error)
	QuitarDePlato(ctx context.Context, platoID id.ID, clave string) ([]string, error)
	AgregarABase(ctx context.Context, impID id.ID, categoria string, bytes []byte, mime string) ([]string, error)
	QuitarDeBase(ctx context.Context, impID id.ID, categoria, clave string) ([]string, error)
}

type ReferenciasDTO struct {
	// URLs para pintarlas; Claves para poder borrar una concreta.
	URLs   []string `json:"urls"`
	Claves []string `json:"claves"`
}

func (h *Handler) montarReferencias(r fiber.Router) {
	r.Post("/platos/:id/referencias", h.subirReferenciaDePlato)
	r.Delete("/platos/:id/referencias", h.quitarReferenciaDePlato)
	r.Post("/importaciones/:id/estilo/referencias", h.subirReferenciaDeBase)
	r.Delete("/importaciones/:id/estilo/referencias", h.quitarReferenciaDeBase)
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
	return c.JSON(ReferenciasDTO{URLs: urls(claves, h.url), Claves: claves})
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
	return c.JSON(ReferenciasDTO{URLs: urls(claves, h.url), Claves: claves})
}

func (h *Handler) subirReferenciaDeBase(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	bytes, mime, err := h.leerImagenSubida(c)
	if err != nil {
		return err
	}

	claves, err := h.referencias.AgregarABase(c.Context(), impID, c.Query("categoria"), bytes, mime)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(ReferenciasDTO{URLs: urls(claves, h.url), Claves: claves})
}

func (h *Handler) quitarReferenciaDeBase(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	claves, err := h.referencias.QuitarDeBase(c.Context(), impID, c.Query("categoria"), c.Query("clave"))
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(ReferenciasDTO{URLs: urls(claves, h.url), Claves: claves})
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
