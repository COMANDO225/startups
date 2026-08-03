package context

import (
	"net"
	"strings"
	"time"

	domainerr "facturacion-service/internal/shared/domain/errors"
	"github.com/gofiber/fiber/v3"
)

// ClientType identifies which frontend platform is calling the API.
type ClientType string

const (
	ClientTypeB2B     ClientType = "b2b"
	ClientTypeB2C     ClientType = "b2c"
	ClientTypeUnknown ClientType = ""
)

const (
	KeyRequestContext = "request_context"
	KeyRequestID      = "request_id"
	KeyTimestamp      = "timestamp"
	KeyUserID         = "user_id"
	KeyOrgID          = "org_id"
	KeySessionID      = "session_id"
	KeyClientType     = "client_type"
)

// RequestContext almacena datos con alcance de solicitud.
type RequestContext struct {
	RequestID  string
	IP         net.IP
	UserAgent  string
	Timestamp  time.Time
	ClientType ClientType
}

// GetRequestContext extrae el RequestContext del contexto de Fiber.
func GetRequestContext(c fiber.Ctx) *RequestContext {
	if ctx, ok := c.Locals(KeyRequestContext).(*RequestContext); ok {
		return ctx
	}
	return &RequestContext{
		Timestamp: time.Now().UTC(),
	}
}

// GetRequestID extrae el ID de solicitud del contexto de Fiber.
func GetRequestID(c fiber.Ctx) string {
	if id, ok := c.Locals(KeyRequestID).(string); ok {
		return id
	}
	return ""
}

// GetUserID extrae el ID de usuario del contexto de Fiber.
func GetUserID(c fiber.Ctx) string {
	if id, ok := c.Locals(KeyUserID).(string); ok {
		return id
	}
	return ""
}

// GetOrgID returns the organization ID from the Fiber context.
func GetOrgID(c fiber.Ctx) string {
	id, _ := c.Locals(KeyOrgID).(string)
	return id
}

// RequireUserID extrae el userID del contexto.
// Retorna *domainerr.Error si no está autenticado.
func RequireUserID(c fiber.Ctx) (string, error) {
	id := GetUserID(c)
	if id == "" {
		return "", domainerr.Authentication("Usuario no autenticado").WithCode("UNAUTHORIZED")
	}
	return id, nil
}

// RequireSessionID extrae el sessionID del contexto.
// Retorna *domainerr.Error si no hay sesión activa.
func RequireSessionID(c fiber.Ctx) (string, error) {
	id, ok := c.Locals(KeySessionID).(string)
	if !ok || id == "" {
		return "", domainerr.Authentication("Sesión no encontrada").WithCode("UNAUTHORIZED")
	}
	return id, nil
}

// ExtractIP extrae la IP real del cliente desde los headers de la solicitud.
// Maneja varios escenarios de proxy (Cloudflare, nginx, etc.).
func ExtractIP(c fiber.Ctx) net.IP {
	headers := []string{
		"CF-Connecting-IP", // Cloudflare
		"True-Client-IP",   // Cloudflare Enterprise
		"X-Real-IP",        // nginx
		"X-Forwarded-For",  // proxy estándar
	}

	for _, header := range headers {
		if ip := c.Get(header); ip != "" {
			// X-Forwarded-For puede tener múltiples IPs (cliente, proxy1, proxy2)
			if header == "X-Forwarded-For" {
				parts := strings.Split(ip, ",")
				ip = strings.TrimSpace(parts[0])
			}
			if parsed := net.ParseIP(ip); parsed != nil {
				return parsed
			}
		}
	}

	// respaldo a IP directa de la conexión
	ipStr := c.IP()
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	return net.ParseIP(ipStr)
}

// ExtractUserAgent extrae el header User-Agent.
func ExtractUserAgent(c fiber.Ctx) string {
	return c.Get("User-Agent")
}

// GetClientType extracts the client type from the Fiber context.
func GetClientType(c fiber.Ctx) ClientType {
	if ct, ok := c.Locals(KeyClientType).(ClientType); ok {
		return ct
	}
	return ClientTypeUnknown
}

// ExtractClientType reads and validates the X-Client-Type header.
func ExtractClientType(c fiber.Ctx) ClientType {
	switch ClientType(strings.ToLower(c.Get("X-Client-Type"))) {
	case ClientTypeB2B:
		return ClientTypeB2B
	case ClientTypeB2C:
		return ClientTypeB2C
	default:
		return ClientTypeUnknown
	}
}
