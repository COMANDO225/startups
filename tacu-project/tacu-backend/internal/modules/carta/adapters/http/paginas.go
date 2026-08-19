package http

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// GestorDePaginas son las hojas de la carta de papel: anadir la que falto,
// quitar la que no era y cambiarles el orden.
type GestorDePaginas interface {
	Agregar(ctx context.Context, importacionID id.ID, bytes []byte, mime string) ([]string, error)
	Quitar(ctx context.Context, importacionID id.ID, clave string) ([]string, error)

	// QuitarConPlatos quita la hoja Y borra sus platos. Devuelve cuantos.
	QuitarConPlatos(ctx context.Context, importacionID id.ID, clave string) (int, error)
	Reordenar(ctx context.Context, importacionID id.ID, claves []string) ([]string, error)
}

// PaginaDTO viaja con clave Y url: la url es para pintarla y la clave es lo que
// identifica la pagina al quitarla o reordenarla.
type PaginaDTO struct {
	Clave string `json:"clave"`
	URL   string `json:"url"`
}

type PaginasDTO struct {
	Paginas []PaginaDTO `json:"paginas"`
}

func (h *Handler) montarPaginas(r fiber.Router) {
	r.Post("/importaciones/:id/paginas", h.agregarPagina)
	r.Delete("/importaciones/:id/paginas", h.quitarPagina)
	r.Put("/importaciones/:id/paginas", h.reordenarPaginas)
}

// agregarPagina guarda una hoja mas y la manda a leer.
//
// Responde 202 y no 200: cuando esta respuesta llega, la pagina esta guardada
// pero sus platos todavia no existen. El contador paginas_leyendo de la
// importacion es lo que dice cuando terminaron de sumarse.
func (h *Handler) agregarPagina(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	cabecera, err := c.FormFile("pagina")
	if err != nil {
		return problema(c, http.StatusBadRequest, "falta el archivo 'pagina'", err.Error())
	}
	if cabecera.Size > h.maxSubidaBytes {
		return problema(c, http.StatusBadRequest, "la pagina pesa demasiado", "")
	}

	f, err := cabecera.Open()
	if err != nil {
		return problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}
	bytes, err := io.ReadAll(io.LimitReader(f, h.maxSubidaBytes))
	_ = f.Close()
	if err != nil {
		return problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}

	// El tipo se SNIFFEA: la cabecera del formulario la pone el cliente y puede
	// decir lo que quiera.
	mime := http.DetectContentType(bytes)
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}

	claves, err := h.paginas.Agregar(c.Context(), impID, bytes, mime)
	if err != nil {
		return traducirError(c, err)
	}
	return c.Status(http.StatusAccepted).JSON(h.paginasDTO(claves))
}

func (h *Handler) quitarPagina(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	clave := c.Query("clave")

	// platos=borrar es la eleccion del dueno en el aviso, y NO tiene vuelta
	// atras: los platos de esa hoja se van con sus fotos. Cualquier otro valor
	// —incluido el que falta— mantiene los platos, que es lo que no destruye
	// nada. Un default destructivo aqui seria borrarle la carta a quien llame
	// mal al endpoint.
	if c.Query("platos") == "borrar" {
		borrados, err := h.paginas.QuitarConPlatos(c.Context(), impID, clave)
		if err != nil {
			return traducirError(c, err)
		}
		imp, err := h.repo.Obtener(c.Context(), impID)
		if err != nil {
			return traducirError(c, err)
		}
		return c.JSON(fiber.Map{
			"paginas":         h.paginasDTO(imp.Imagenes).Paginas,
			"platos_borrados": borrados,
		})
	}

	claves, err := h.paginas.Quitar(c.Context(), impID, clave)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(h.paginasDTO(claves))
}

func (h *Handler) reordenarPaginas(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	var cuerpo struct {
		Claves []string `json:"claves"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	claves, err := h.paginas.Reordenar(c.Context(), impID, cuerpo.Claves)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(h.paginasDTO(claves))
}

func (h *Handler) paginasDTO(claves []string) PaginasDTO {
	dto := PaginasDTO{Paginas: make([]PaginaDTO, 0, len(claves))}
	for _, c := range claves {
		dto.Paginas = append(dto.Paginas, PaginaDTO{Clave: c, URL: h.url(c)})
	}
	return dto
}
