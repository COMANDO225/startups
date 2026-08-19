package http

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// EditorDePlato corrige a mano lo que la IA no podia saber.
type EditorDePlato interface {
	// Etiquetas le pone nombre a los precios de un plato con varias opciones.
	// Devuelve el plato ya reverificado y el recuento nuevo de la carta.
	Etiquetas(ctx context.Context, platoID id.ID, etiquetas []string) (domain.Plato, domain.Marcas, error)

	// Quitar borra un plato de la carta. Es la confirmacion del dueno sobre un
	// plato que la lectura marco como ausente, o simplemente uno que no quiere.
	Quitar(ctx context.Context, platoID id.ID) error

	// Recuperar deshace el marcado de ausente: la lectura se equivoco y el
	// plato sigue en la carta.
	Recuperar(ctx context.Context, platoID id.ID) error
}

// PlatoEditadoDTO es lo que hace falta para apagar el aviso en pantalla sin
// recargar la carta entera.
//
// Devuelve TAMBIEN las marcas de toda la importacion, y esa es la parte que
// importa: la barra de publicar lee ese recuento. Sin el en la respuesta, el
// frontend apagaria el rojo de la tarjeta y la barra seguiria diciendo "quedan 3
// por revisar" hasta la siguiente recarga.
type PlatoEditadoDTO struct {
	Plato  PlatoDTO  `json:"plato"`
	Marcas MarcasDTO `json:"marcas"`
}

// editarPlato aplica las correcciones del dueno sobre un plato.
//
// Hoy solo las etiquetas de los precios, que es lo unico que el dueno puede
// arreglar tecleando: es lo que desbloquea publicar cuando la carta traia dos
// montos en una fila sin decir de que era cada uno. Nombre, descripcion y
// precios entran por aqui el dia que se ofrezcan; el endpoint ya es el sitio.
func (h *Handler) editarPlato(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}

	var cuerpo struct {
		// Etiquetas van por POSICION, igual que se pintan. Es lo mismo que hace
		// la pantalla y evita inventarle un id a cada precio, que hoy no tiene.
		Etiquetas []string `json:"etiquetas"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	plato, marcas, err := h.editar.Etiquetas(c.Context(), platoID, cuerpo.Etiquetas)
	if err != nil {
		return traducirError(c, err)
	}

	return c.JSON(PlatoEditadoDTO{
		Plato:  aPlatoDTO(plato, h.url),
		Marcas: MarcasDTO{Revisar: marcas.Revisar, Confirmar: marcas.Confirmar},
	})
}

// quitarPlato borra un plato. Se llega aqui por decision del dueno, nunca por
// una lectura: un plato puede tener una foto de $0.0336 detras y la lectura
// pudo equivocarse al no traerlo.
func (h *Handler) quitarPlato(c fiber.Ctx) error {
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

	if err := h.editar.Quitar(c.Context(), platoID); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// recuperarPlato deshace el "ausente": la lectura no lo trajo pero sigue en la
// carta. Pasa cuando una hoja sale movida y esa parte no se leyo.
func (h *Handler) recuperarPlato(c fiber.Ctx) error {
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

	if err := h.editar.Recuperar(c.Context(), platoID); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
