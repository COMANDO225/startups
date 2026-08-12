package http

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"facturacion-service/internal/features/comprobante/domain"

	"github.com/gofiber/fiber/v3"
)

// MonitorSalud es lo unico que las metricas necesitan del repositorio. Va aparte
// de domain.Repositorio para no obligar a cada implementacion a cargar con esto.
type MonitorSalud interface {
	Salud(ctx context.Context) (domain.SaludPipeline, error)
}

// Metricas expone el estado del pipeline en formato de exposicion de Prometheus.
//
// ponytail: el formato son tres lineas de texto por metrica, asi que se escribe
// a mano en vez de sumar client_golang. Si algun dia hacen falta las metricas
// del runtime de Go (GC, goroutines), ahi si conviene la libreria.
func Metricas(monitor MonitorSalud, token string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !tokenValido(c, token) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		salud, err := monitor.Salud(c.Context())
		if err != nil {
			return err
		}

		var b strings.Builder

		b.WriteString("# HELP facturacion_comprobantes Comprobantes por estado.\n")
		b.WriteString("# TYPE facturacion_comprobantes gauge\n")
		// Se emiten todos los estados conocidos, incluso en cero: un estado que
		// aparece y desaparece del scrape produce huecos en las graficas.
		for _, e := range domain.TodosLosEstados() {
			fmt.Fprintf(&b, "facturacion_comprobantes{estado=%q} %d\n", e, salud.PorEstado[e])
		}

		b.WriteString("\n# HELP facturacion_requieren_atencion Comprobantes que ningun reintento arregla. Deberia ser cero.\n")
		b.WriteString("# TYPE facturacion_requieren_atencion gauge\n")
		fmt.Fprintf(&b, "facturacion_requieren_atencion %d\n", salud.RequierenAtencion)

		b.WriteString("\n# HELP facturacion_mas_viejo_sin_resolver_segundos Antiguedad del comprobante mas viejo que aun no llega a estado final.\n")
		b.WriteString("# TYPE facturacion_mas_viejo_sin_resolver_segundos gauge\n")
		fmt.Fprintf(&b, "facturacion_mas_viejo_sin_resolver_segundos %.0f\n", salud.AntiguedadMasViejo.Seconds())

		c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
		return c.SendString(b.String())
	}
}

// Sin token configurado el endpoint queda abierto, que sirve en desarrollo. En
// produccion conviene ponerlo: el volumen de comprobantes es informacion del
// negocio y no tiene por que ser publica.
func tokenValido(c fiber.Ctx, token string) bool {
	if token == "" {
		return true
	}
	recibido := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(recibido), []byte(token)) == 1
}
