package http

import (
	"strings"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"

	"github.com/gofiber/fiber/v3"
)

const ctxKeyTenant = "tenant"

func Register(router fiber.Router, ctrl *Controller, tenants domain.TenantRepositorio) {
	g := router.Group("/comprobantes", autenticar(tenants))

	g.Post("/", ctrl.Emitir)
	g.Get("/", ctrl.Listar)
	g.Get("/atencion", ctrl.Atencion)
	g.Get("/:id", ctrl.Obtener)
	g.Get("/:id/impresa", ctrl.Impresa)
	g.Get("/:id/qr.png", ctrl.QRPNG)
	g.Post("/:id/anular", ctrl.Anular)
}

// autenticar resuelve el tenant desde la API key. Cada emisor tiene su propio
// certificado y su Clave SOL, asi que el tenant define con que credenciales se
// firma y se envia a SUNAT.
func autenticar(tenants domain.TenantRepositorio) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		if key == "" {
			return domainerr.Authentication("Falta la API key").WithCode("API_KEY_REQUERIDA")
		}

		tenant, err := tenants.PorAPIKey(c.Context(), key)
		if err != nil {
			return domainerr.Authentication("API key invalida").WithCode("API_KEY_INVALIDA")
		}

		c.Locals(ctxKeyTenant, tenant)
		return c.Next()
	}
}
