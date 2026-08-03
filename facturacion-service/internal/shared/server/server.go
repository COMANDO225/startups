package server

import (
	"errors"
	"fmt"
	"time"

	domainerr "facturacion-service/internal/shared/domain/errors"
	"facturacion-service/internal/shared/logger"
	"facturacion-service/pkg/ulid"
	"github.com/gofiber/fiber/v3"

	sharedctx "facturacion-service/internal/shared/context"
)

type Server struct {
	App *fiber.App
}

type Config struct {
	AppName      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	BodyLimit    int
	TrustProxy   bool
	ProxyHeader  string
}

// New crea el servidor Fiber. StructValidator hace que cada c.Bind() valide
// automaticamente, por eso no existe un middleware de bind.
func New(cfg Config, log *logger.Logger, v fiber.StructValidator) *Server {
	app := fiber.New(fiber.Config{
		AppName:            cfg.AppName,
		ReadTimeout:        cfg.ReadTimeout,
		WriteTimeout:       cfg.WriteTimeout,
		IdleTimeout:        cfg.IdleTimeout,
		BodyLimit:          cfg.BodyLimit,
		EnableIPValidation: true,
		TrustProxy:         cfg.TrustProxy,
		ProxyHeader:        cfg.ProxyHeader,
		ErrorHandler:       newGlobalErrorHandler(log),
		StructValidator:    v,
	})

	app.RegisterCustomConstraint(ulid.RouteConstraint{})

	return &Server{App: app}
}

func (s *Server) Listen(port int) error {
	return s.App.Listen(fmt.Sprintf(":%d", port), fiber.ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         false,
	})
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	return s.App.Shutdown()
}

// --- error handler ---

type errorPayload struct {
	Code       string             `json:"code"`
	Message    string             `json:"message"`
	Details    []domainerr.Detail `json:"details,omitempty"`
	Path       string             `json:"path,omitempty"`
	Suggestion string             `json:"suggestion,omitempty"`
	Meta       map[string]string  `json:"meta,omitempty"`
}

type errorResponse struct {
	RequestID string        `json:"request_id"`
	Timestamp time.Time     `json:"timestamp"`
	Status    string        `json:"status"`
	Error     *errorPayload `json:"error,omitempty"`
}

// mapeo de errores de Fiber (404 de ruta, 405, etc.) a códigos de dominio.
var fiberErrorMappings = map[int]struct{ Code, Message string }{
	fiber.StatusNotFound:              {"NOT_FOUND", "Ruta no encontrada"},
	fiber.StatusMethodNotAllowed:      {"METHOD_NOT_ALLOWED", "Método no permitido"},
	fiber.StatusRequestEntityTooLarge: {"PAYLOAD_TOO_LARGE", "El cuerpo de la solicitud es demasiado grande"},
	fiber.StatusRequestTimeout:        {"REQUEST_TIMEOUT", "La solicitud excedió el tiempo límite"},
}

func newGlobalErrorHandler(log *logger.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		var domErr *domainerr.Error
		if errors.As(err, &domErr) {
			return handleDomainError(c, log, domErr)
		}

		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return handleFiberError(c, log, fiberErr)
		}

		log.Errorw("error no manejado",
			"error", err.Error(),
			"request_id", sharedctx.GetRequestID(c),
			"method", c.Method(),
			"path", c.Path(),
		)
		return sendError(c, 500, "INTERNAL_ERROR", "Error interno del servidor", nil, "", nil)
	}
}

func handleDomainError(c fiber.Ctx, log *logger.Logger, domErr *domainerr.Error) error {
	kind := domErr.Kind()
	status := kind.HTTPStatus()
	code := domErr.Code()
	suggestion := domErr.Suggestion()
	message := domErr.Message()

	if status >= 500 {
		log.Errorw("error interno de dominio",
			"error", domErr.Error(),
			"code", code,
			"request_id", sharedctx.GetRequestID(c),
			"method", c.Method(),
			"path", c.Path(),
		)
		message = kind.DefaultMessage()
		suggestion = ""
	}
	if message == "" {
		message = kind.DefaultMessage()
	}

	return sendError(c, status, code, message, domErr.Details(), suggestion, domErr.Meta())
}

func handleFiberError(c fiber.Ctx, log *logger.Logger, fiberErr *fiber.Error) error {
	code := "INTERNAL_ERROR"
	message := fiberErr.Message

	if mapping, ok := fiberErrorMappings[fiberErr.Code]; ok {
		code = mapping.Code
		message = mapping.Message
	}

	if fiberErr.Code >= 500 {
		log.Errorw("error de fiber",
			"error", fiberErr.Message,
			"status", fiberErr.Code,
			"request_id", sharedctx.GetRequestID(c),
			"method", c.Method(),
			"path", c.Path(),
		)
	}

	return sendError(c, fiberErr.Code, code, message, nil, "", nil)
}

func sendError(c fiber.Ctx, status int, code, message string, details []domainerr.Detail, suggestion string, meta map[string]string) error {
	payload := &errorPayload{
		Code:       code,
		Message:    message,
		Path:       c.Path(),
		Suggestion: suggestion,
	}
	if len(details) > 0 {
		payload.Details = details
	}
	if len(meta) > 0 {
		payload.Meta = meta
	}

	return c.Status(status).JSON(errorResponse{
		RequestID: sharedctx.GetRequestID(c),
		Timestamp: time.Now().UTC(),
		Status:    "error",
		Error:     payload,
	})
}
