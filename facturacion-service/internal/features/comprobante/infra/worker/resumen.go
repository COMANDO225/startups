package worker

import (
	"context"
	"time"

	"facturacion-service/internal/features/comprobante/application/command"
	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"

	"github.com/riverqueue/river"
)

const (
	// SUNAT tarda minutos en resolver un resumen; consultar mas seguido solo
	// gasta llamadas.
	reintentoTicket = 3 * time.Minute

	// Los resumenes se arman al cierre del dia, pero el barrido corre seguido
	// para recoger lo que quedo de dias anteriores.
	intervaloResumen  = 15 * time.Minute
	maxGruposPorRonda = 50
)

// --- Armar el resumen diario ---

type ResumenDiarioArgs struct{}

func (ResumenDiarioArgs) Kind() string { return "comprobante.resumen_diario" }

type ResumenDiarioWorker struct {
	river.WorkerDefaults[ResumenDiarioArgs]
	repo  domain.ResumenRepositorio
	armar *command.ArmarResumen
	log   Logger
}

func NewResumenDiarioWorker(repo domain.ResumenRepositorio, armar *command.ArmarResumen, log Logger) *ResumenDiarioWorker {
	return &ResumenDiarioWorker{repo: repo, armar: armar, log: log}
}

func (w *ResumenDiarioWorker) Work(ctx context.Context, _ *river.Job[ResumenDiarioArgs]) error {
	grupos, err := w.repo.GruposPendientes(ctx, maxGruposPorRonda)
	if err != nil {
		return err
	}

	for _, g := range grupos {
		res, err := w.armar.Execute(ctx, g.TenantID, g.Fecha)
		if err != nil {
			// Sin boletas no es un fallo: otro worker se adelanto.
			if domErr, ok := domainerr.AsError(err); ok && domErr.Code() == "SIN_BOLETAS" {
				continue
			}
			w.log.Errorw("no se pudo armar el resumen diario",
				"tenant_id", g.TenantID, "fecha", g.Fecha.Format("2006-01-02"), "error", err)
			continue
		}

		// El envio lo hace EnviarResumenWorker: aqui solo se crea y se encola,
		// para que un fallo de red no deje boletas sin resumen.
		w.log.Infow("resumen diario creado",
			"resumen", res.Identificador(), "tenant_id", g.TenantID)
	}

	return nil
}

// --- Enviar el resumen ---

type EnviarResumenArgs struct {
	ResumenID string `json:"resumen_id"`
}

func (EnviarResumenArgs) Kind() string { return "comprobante.enviar_resumen" }

func (a EnviarResumenArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: estadosActivos()}}
}

type EnviarResumenWorker struct {
	river.WorkerDefaults[EnviarResumenArgs]
	enviar *command.EnviarResumen
	cola   EncoladorTickets
	repo   domain.ResumenRepositorio
}

type EncoladorTickets interface {
	EncolarConsultaTicket(ctx context.Context, resumenID string) error
}

func NewEnviarResumenWorker(enviar *command.EnviarResumen, repo domain.ResumenRepositorio, cola EncoladorTickets) *EnviarResumenWorker {
	return &EnviarResumenWorker{enviar: enviar, repo: repo, cola: cola}
}

func (w *EnviarResumenWorker) Work(ctx context.Context, job *river.Job[EnviarResumenArgs]) error {
	err := w.enviar.Execute(ctx, job.Args.ResumenID)

	if domErr, ok := domainerr.AsError(err); ok && domErr.Code() == "COMPROBANTE_EN_VUELO" {
		return river.JobSnooze(reintentoEnVuelo)
	}
	if err != nil {
		return err
	}

	// Si SUNAT devolvio ticket, el ciclo sigue con la consulta.
	res, err := w.repo.ResumenPorID(ctx, job.Args.ResumenID)
	if err != nil {
		return err
	}
	if res.Ticket() == "" {
		return nil
	}

	return w.cola.EncolarConsultaTicket(ctx, res.ID())
}

// --- Consultar el ticket ---

type ConsultarTicketArgs struct {
	ResumenID string `json:"resumen_id"`
}

func (ConsultarTicketArgs) Kind() string { return "comprobante.consultar_ticket" }

func (a ConsultarTicketArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: estadosActivos()}}
}

type ConsultarTicketWorker struct {
	river.WorkerDefaults[ConsultarTicketArgs]
	consultar *command.ConsultarTicket
}

func NewConsultarTicketWorker(consultar *command.ConsultarTicket) *ConsultarTicketWorker {
	return &ConsultarTicketWorker{consultar: consultar}
}

func (w *ConsultarTicketWorker) Work(ctx context.Context, job *river.Job[ConsultarTicketArgs]) error {
	err := w.consultar.Execute(ctx, job.Args.ResumenID)

	// El job se reprograma a si mismo en vez de fallar: SUNAT todavia no
	// termino de procesar el resumen.
	if domErr, ok := domainerr.AsError(err); ok && domErr.Code() == "TICKET_NO_LISTO" {
		return river.JobSnooze(reintentoTicket)
	}

	return err
}
