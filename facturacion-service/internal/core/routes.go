package core

import (
	comprobantehttp "facturacion-service/internal/features/comprobante/infra/http"
	"facturacion-service/internal/shared/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func (a *App) registerMiddleware() {
	app := a.Server.App

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(middleware.RequestContext())
}

func (a *App) registerRoutes(comprobante *comprobanteDeps) {
	app := a.Server.App

	app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
	app.Get(healthcheck.ReadinessEndpoint, healthcheck.New(healthcheck.Config{
		Probe: func(c fiber.Ctx) bool {
			return a.DB.Health(c.Context()) == nil
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": a.Config.App.Name,
			"version": a.Config.App.Version,
		})
	})

	v1 := app.Group("/v1")
	comprobantehttp.Register(v1, comprobante.controller, comprobante.tenants)
}
