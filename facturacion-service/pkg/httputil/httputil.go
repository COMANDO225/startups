package httputil

import (
	"net"
	"strconv"
	"strings"

	domainerr "facturacion-service/internal/shared/domain/errors"
	"facturacion-service/pkg/ulid"
	"github.com/gofiber/fiber/v3"
)

// ParsePagination extrae page, limit y offset de query strings con validación.
// maxLimit clampea el limit máximo permitido.
func ParsePagination(pageStr, limitStr string, maxLimit int) (page, limit, offset int) {
	page = ParseQueryInt(pageStr, 1)
	limit = ParseQueryInt(limitStr, 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset = (page - 1) * limit
	return
}

// ParseQueryInt parsea un query string a int con fallback.
func ParseQueryInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

// ParseFormFloat parsea un string de form a float32 con fallback.
// Clampea el resultado entre min y max.
func ParseFormFloat(s string, fallback, min, max float32) float32 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return fallback
	}
	f := float32(v)
	if f < min {
		return min
	}
	if f > max {
		return max
	}
	return f
}

// DerefBool desreferencia *bool con valor default si nil.
func DerefBool(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

// RequireIDParam extrae y valida un path param como ULID.
// Retorna el string original si es válido, o error de validación si no.
func RequireIDParam(c fiber.Ctx, name string) (string, error) {
	raw := c.Params(name)
	if _, err := ulid.Parse(raw); err != nil {
		return "", domainerr.Validation("ID inválido: " + name).WithCode("INVALID_ID")
	}
	return raw, nil
}

// StringPtr retorna un puntero a string, nil si está vacío.
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// IPToString convierte net.IP a string, retorna vacío si nil.
func IPToString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

// IPToStringPtr convierte net.IP a *string, retorna nil si IP es nil.
func IPToStringPtr(ip net.IP) *string {
	if ip == nil {
		return nil
	}
	s := ip.String()
	return &s
}

// ParseIP parsea una cadena de dirección IP, manejando corchetes IPv6.
func ParseIP(s string) net.IP {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	return net.ParseIP(s)
}
