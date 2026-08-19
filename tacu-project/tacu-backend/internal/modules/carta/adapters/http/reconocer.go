package http

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
)

// Reconocedor vuelve a pasar una carta ya guardada por el banco de platos.
type Reconocedor interface {
	Reconocer(ctx context.Context, importacionID id.ID) (emparejados, aprendidos int, err error)
}

// ReconocidoDTO es lo que hace falta para saber si sirvio de algo.
type ReconocidoDTO struct {
	// Emparejados son los platos que ahora saben que son.
	Emparejados int `json:"emparejados"`

	// Aprendidos son los platos que el banco no conocia y ahora si. Ese numero
	// no es de esta carta: queda para todos los restaurantes que vengan.
	Aprendidos int `json:"aprendidos"`

	Total int `json:"total"`
}

func (h *Handler) montarReconocer(r fiber.Router) {
	r.Post("/importaciones/:id/reconocer", h.reconocer)
}

// reconocer arrastra hacia atras lo que el banco aprendio despues.
//
// Existe porque el banco crece: una carta leida ayer no sabe lo que el banco
// aprendio hoy, y sin esto se quedaria con sus fotos viejas para siempre.
//
// NO reescribe la carta: solo la clave de cada plato. Ningun id cambia, asi que
// las fotos ya generadas —que estan pagadas— siguen colgando de su plato.
//
// Es POST y no GET porque escribe, y porque puede costar una llamada al modelo
// cuando quedan huecos. Se paga sola una vez: en la siguiente ya no hay huecos
// que preguntar.
func (h *Handler) reconocer(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	emparejados, aprendidos, err := h.reconocedor.Reconocer(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}

	imp, err := h.repo.Obtener(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}

	h.log.Info("carta reconocida contra el banco",
		"importacion", impID, "emparejados", emparejados, "aprendidos", aprendidos)

	return c.JSON(ReconocidoDTO{
		Emparejados: emparejados,
		Aprendidos:  aprendidos,
		Total:       len(imp.Carta.Platos()),
	})
}
