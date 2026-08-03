package domainerr

import (
	"errors"
	"fmt"
)

// --- Kind: categoría de error → HTTP status ---

// Kind categoriza errores de dominio. Cada Kind mapea directamente
// a un HTTP status code y lleva su código y mensaje default.
type Kind uint8

const (
	KindUnknown        Kind = iota // 500 — error no clasificado
	KindValidation                 // 400 — datos de entrada inválidos
	KindNotFound                   // 404 — recurso no encontrado
	KindConflict                   // 409 — recurso ya existe o conflicto de estado
	KindAuthentication             // 401 — credenciales o token inválido/expirado
	KindAuthorization              // 403 — acceso denegado
	KindRateLimit                  // 429 — demasiadas solicitudes
	KindInternal                   // 500 — error interno
)

// kindInfo centraliza TODO lo que define un Kind en un solo lugar.
// Agregar un Kind nuevo = 1 entrada aquí + 1 constructor.
type kindInfo struct {
	httpStatus  int
	defaultCode string
	message     string
}

// Registro único — array indexado por Kind para O(1) sin hash overhead.
var kinds = [...]kindInfo{
	KindUnknown:        {500, "UNKNOWN_ERROR", "Error interno del servidor"},
	KindValidation:     {400, "VALIDATION_ERROR", "Los datos enviados son inválidos"},
	KindNotFound:       {404, "NOT_FOUND", "Recurso no encontrado"},
	KindConflict:       {409, "CONFLICT", "El recurso ya existe o hay un conflicto"},
	KindAuthentication: {401, "AUTHENTICATION_ERROR", "Credenciales inválidas"},
	KindAuthorization:  {403, "AUTHORIZATION_ERROR", "Acceso denegado"},
	KindRateLimit:      {429, "RATE_LIMITED", "Demasiadas solicitudes"},
	KindInternal:       {500, "INTERNAL_ERROR", "Error interno del servidor"},
}

// HTTPStatus retorna el código HTTP asociado a este Kind.
func (k Kind) HTTPStatus() int { return kinds[k].httpStatus }

// DefaultMessage retorna el mensaje genérico del Kind (fallback para 5xx o mensajes vacíos).
func (k Kind) DefaultMessage() string { return kinds[k].message }

// --- Error type ---

// Detail representa un error específico de campo (validación).
type Detail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// Error es el tipo de error unificado del dominio.
//
// Funciona como las excepciones de Java: el Kind determina la categoría (HTTP status),
// el message es el mensaje para el cliente (4xx) o de debug (5xx — el handler lo reemplaza),
// y el code identifica el error específico para el frontend.
//
// Uso simple (el código se deriva del Kind):
//
//	domainerr.NotFound("usuario no encontrado")
//
// Con código custom cuando el frontend lo necesita:
//
//	domainerr.Conflict("el email ya está registrado").WithCode("EMAIL_ALREADY_EXISTS")
type Error struct {
	kind       Kind
	code       string
	message    string
	suggestion string
	details    []Detail
	meta       map[string]string
	cause      error
}

// --- constructores por Kind ---

// Validation crea un error de datos inválidos (400).
func Validation(message string) *Error {
	return newFromKind(KindValidation, message)
}

// NotFound crea un error de recurso no encontrado (404).
func NotFound(message string) *Error {
	return newFromKind(KindNotFound, message)
}

// Conflict crea un error de conflicto/recurso existente (409).
func Conflict(message string) *Error {
	return newFromKind(KindConflict, message)
}

// Authentication crea un error de credenciales/token inválido (401).
func Authentication(message string) *Error {
	return newFromKind(KindAuthentication, message)
}

// Authorization crea un error de acceso denegado (403).
func Authorization(message string) *Error {
	return newFromKind(KindAuthorization, message)
}

// RateLimit crea un error de demasiadas solicitudes (429).
func RateLimit(message string) *Error {
	return newFromKind(KindRateLimit, message)
}

// Internal crea un error interno (500).
func Internal(message string) *Error {
	return newFromKind(KindInternal, message)
}

func newFromKind(k Kind, message string) *Error {
	return &Error{kind: k, code: kinds[k].defaultCode, message: message}
}

// New crea un error con kind, code y mensaje explícitos.
// Escape hatch para casos que necesitan control total.
func New(kind Kind, code, message string) *Error {
	return &Error{kind: kind, code: code, message: message}
}

// --- error interface ---

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %s", e.code, e.message, e.cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is permite matching por Kind o Code con errors.Is().
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	if t.kind != KindUnknown && e.kind == t.kind {
		return true
	}
	if t.code != "" && e.code == t.code {
		return true
	}
	return false
}

// --- fluent builders ---

// Wrap envuelve un error causa, creando una copia para preservar el original.
func (e *Error) Wrap(cause error) *Error {
	return &Error{
		kind:       e.kind,
		code:       e.code,
		message:    e.message,
		suggestion: e.suggestion,
		details:    e.details,
		meta:       e.meta,
		cause:      cause,
	}
}

// WithCode establece un código de error custom (para el frontend).
func (e *Error) WithCode(code string) *Error {
	e.code = code
	return e
}

// WithSuggestion establece la sugerencia accionable para el cliente.
func (e *Error) WithSuggestion(s string) *Error {
	e.suggestion = s
	return e
}

// WithDetails agrega detalles de validación por campo.
func (e *Error) WithDetails(details ...Detail) *Error {
	e.details = append(e.details, details...)
	return e
}

// WithMeta agrega metadata arbitraria clave-valor.
func (e *Error) WithMeta(key, value string) *Error {
	if e.meta == nil {
		e.meta = make(map[string]string, 2)
	}
	e.meta[key] = value
	return e
}

// --- getters ---

func (e *Error) Kind() Kind              { return e.kind }
func (e *Error) Code() string            { return e.code }
func (e *Error) Message() string         { return e.message }
func (e *Error) Suggestion() string      { return e.suggestion }
func (e *Error) Details() []Detail       { return e.details }
func (e *Error) Meta() map[string]string { return e.meta }
func (e *Error) Cause() error            { return e.cause }

// --- helpers ---

// AsError extrae un *Error de cualquier error usando errors.As.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// IsKind verifica si un error corresponde a un Kind específico.
func IsKind(err error, kind Kind) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.kind == kind
	}
	return false
}
