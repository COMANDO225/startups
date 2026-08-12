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

	app.Get("/metrics", comprobantehttp.Metricas(comprobante.monitor, a.Config.Metrics.Token))

	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": a.Config.App.Name,
			"version": a.Config.App.Version,
		})
	})

	v1 := app.Group("/v1")

	// Fuera del grupo autenticado por API key: el alta usa su propio token de
	// administracion, porque crea justamente las API keys.
	v1.Post("/emisores", comprobantehttp.CrearEmisor(comprobante.crearEmisor, a.Config.Admin.Token))

	comprobantehttp.Register(v1, comprobante.controller, comprobante.tenants)
}
