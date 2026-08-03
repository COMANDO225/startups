package response

import "time"

// Status representa el estado de la respuesta.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// Response es la estructura estándar de respuesta exitosa de la API.
// Las respuestas de error se manejan en server/error_handler.go con su propio struct.
type Response[T any] struct {
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
	Status    Status    `json:"status"`
	Data      T         `json:"data,omitempty"`
	Metadata  *Metadata `json:"metadata,omitempty"`
}

// Metadata contiene información de paginación y acciones.
type Metadata struct {
	// paginación
	Page        int  `json:"page,omitempty"`
	Limit       int  `json:"limit,omitempty"`
	TotalItems  int  `json:"total_items,omitempty"`
	TotalPages  int  `json:"total_pages,omitempty"`
	HasNext     bool `json:"has_next,omitempty"`
	HasPrevious bool `json:"has_previous,omitempty"`

	// acción para el frontend
	Action *Action `json:"action,omitempty"`
}

// Action representa una acción/vista del frontend a activar.
type Action struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// MessageResponse representa una respuesta con solo un mensaje.
type MessageResponse struct {
	Message string `json:"message"`
}

// Pagination almacena información de paginación para construir metadatos.
type Pagination struct {
	Page       int
	Limit      int
	TotalItems int
}

// ToMetadata convierte Pagination a Metadata.
func (p Pagination) ToMetadata() *Metadata {
	totalPages := (p.TotalItems + p.Limit - 1) / p.Limit
	if totalPages < 1 {
		totalPages = 1
	}
	return &Metadata{
		Page:        p.Page,
		Limit:       p.Limit,
		TotalItems:  p.TotalItems,
		TotalPages:  totalPages,
		HasNext:     p.Page < totalPages,
		HasPrevious: p.Page > 1,
	}
}
