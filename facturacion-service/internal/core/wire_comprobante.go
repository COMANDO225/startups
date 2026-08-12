package core

import (
	"facturacion-service/internal/features/comprobante/application/command"
	"facturacion-service/internal/features/comprobante/application/query"
	"facturacion-service/internal/features/comprobante/domain"
	engineclient "facturacion-service/internal/features/comprobante/infra/engine"
	comprobantehttp "facturacion-service/internal/features/comprobante/infra/http"
	"facturacion-service/internal/features/comprobante/infra/postgres"
	"facturacion-service/internal/features/comprobante/infra/webhook"
	"facturacion-service/internal/features/comprobante/infra/worker"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/crypto"

	"github.com/riverqueue/river"
)

type comprobanteDeps struct {
	controller  *comprobantehttp.Controller
	tenants     domain.TenantRepositorio
	monitor     comprobantehttp.MonitorSalud
	crearEmisor *command.CrearEmisor
}

func (a *App) wireComprobante(cipher *crypto.Cipher, workers *river.Workers) *comprobanteDeps {
	repo := postgres.NewRepo(a.DB.Pool)
	tenants := postgres.NewTenantRepo(a.DB.Pool, cipher)
	motor := engineclient.NewClient(a.Config.Engine.URL, a.Config.Engine.Timeout, a.Config.Engine.MaxPorEmisor)
	cola := worker.NewEncolador(a.River)
	tx := transaction.NewTransactor(a.DB.Pool)

	// En desarrollo los webhooks apuntan a localhost; en produccion eso seria
	// una puerta abierta a la red interna (SSRF).
	sender := webhook.NewSender(a.Config.Webhook.Timeout, !a.Config.IsProduction())

	emitir := command.NewEmitir(repo, cola, tx)
	anular := command.NewAnular(repo, repo, cola, tx)
	procesar := command.NewProcesar(repo, tenants, motor, cola)
	armar := command.NewArmarResumen(repo, cola, tx)
	enviarResumen := command.NewEnviarResumen(repo, tenants, motor)
	consultar := command.NewConsultarTicket(repo, tenants, motor)
	notificar := command.NewNotificar(repo, tenants, sender)

	river.AddWorker(workers, worker.NewEmitirWorker(procesar))
	river.AddWorker(workers, worker.NewRescatarWorker(repo, repo, cola, repo, a.Log))
	river.AddWorker(workers, worker.NewResumenDiarioWorker(repo, armar, a.Log))
	river.AddWorker(workers, worker.NewEnviarResumenWorker(enviarResumen, repo, cola))
	river.AddWorker(workers, worker.NewConsultarTicketWorker(consultar))
	river.AddWorker(workers, worker.NewNotificarWorker(notificar))

	return &comprobanteDeps{
		controller: comprobantehttp.NewController(
			emitir,
			anular,
			query.NewObtener(repo),
			query.NewListar(repo),
			query.NewRequierenAtencion(repo),
		),
		tenants:     tenants,
		monitor:     repo,
		crearEmisor: command.NewCrearEmisor(tenants, tx),
	}
}
