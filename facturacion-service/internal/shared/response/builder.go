package response

import (
	"time"

	sharedctx "facturacion-service/internal/shared/context"
	"github.com/gofiber/fiber/v3"
)

// Builder construye respuestas exitosas de API con API fluida.
type Builder[T any] struct {
	ctx      fiber.Ctx
	data     T
	metadata *Metadata
	status   int
}

// New crea un nuevo constructor de respuestas.
func New[T any](c fiber.Ctx) *Builder[T] {
	return &Builder[T]{
		ctx:    c,
		status: fiber.StatusOK,
	}
}

// Data establece los datos de la respuesta.
func (b *Builder[T]) Data(data T) *Builder[T] {
	b.data = data
	return b
}

// WithPagination agrega metadatos de paginación.
func (b *Builder[T]) WithPagination(page, limit, total int) *Builder[T] {
	p := Pagination{Page: page, Limit: limit, TotalItems: total}
	b.metadata = p.ToMetadata()
	return b
}

// WithAction agrega una acción para el frontend.
func (b *Builder[T]) WithAction(action *Action) *Builder[T] {
	if b.metadata == nil {
		b.metadata = &Metadata{}
	}
	b.metadata.Action = action
	return b
}

// WithMetadata establece metadatos personalizados.
func (b *Builder[T]) WithMetadata(metadata *Metadata) *Builder[T] {
	b.metadata = metadata
	return b
}

// Status establece el código de estado HTTP.
func (b *Builder[T]) Status(status int) *Builder[T] {
	b.status = status
	return b
}

// Success envía una respuesta exitosa con 200 OK.
func (b *Builder[T]) Success() error {
	return b.send(fiber.StatusOK)
}

// Created envía una respuesta exitosa con 201 Created.
func (b *Builder[T]) Created() error {
	return b.send(fiber.StatusCreated)
}

// NoContent envía una respuesta 204 sin contenido.
func (b *Builder[T]) NoContent() error {
	return b.ctx.SendStatus(fiber.StatusNoContent)
}

func (b *Builder[T]) send(status int) error {
	return b.ctx.Status(status).JSON(Response[T]{
		RequestID: sharedctx.GetRequestID(b.ctx),
		Timestamp: time.Now().UTC(),
		Status:    StatusSuccess,
		Data:      b.data,
		Metadata:  b.metadata,
	})
}
