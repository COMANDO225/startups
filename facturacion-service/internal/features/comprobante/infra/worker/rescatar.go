package worker

import (
	"context"
	"time"

	"facturacion-service/internal/features/comprobante/domain"

	"github.com/riverqueue/river"
)

const (
	// antiguedadHuerfano: margen sobre TimeoutProcesando para no reencolar algo
	// que un worker todavia tiene en la mano.
	antiguedadHuerfano = 10 * time.Minute

	// El webhook lo reintenta River hasta 10 veces con backoff; recien despues
	// de eso tiene sentido que el barredor lo levante.
	antiguedadWebhook = 30 * time.Minute

	limiteRescate = 200
)

type RescatarArgs struct{}

func (RescatarArgs) Kind() string { return "comprobante.rescatar" }

// RescatarWorker es la red de seguridad del pipeline.
//
// River garantiza que un job encolado no se pierde, pero no que siempre vuelva:
// si se cierra por error o se descarta tras agotar reintentos, lo que quedo a
// medias no tiene quien lo empuje. Este barrido cubre las tres cosas que pueden
// quedar colgadas: comprobantes, resumenes y webhooks.
type RescatarWorker struct {
	river.WorkerDefaults[RescatarArgs]
	repo    domain.Repositorio
	resumen domain.ResumenRepositorio
	cola    ColaRescate
	log     Logger
}

// ColaRescate junta todo lo que el barredor necesita reencolar.
type ColaRescate interface {
	EncolarEmision(ctx context.Context, comprobanteID string) error
	EncolarEnvioResumen(ctx context.Context, resumenID string) error
	EncolarConsultaTicket(ctx context.Context, resumenID string) error
	EncolarNotificacion(ctx context.Context, tenantID, comprobanteID string) error
}

type Logger interface {
	Infow(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
}

func NewRescatarWorker(repo domain.Repositorio, resumen domain.ResumenRepositorio, cola ColaRescate, log Logger) *RescatarWorker {
	return &RescatarWorker{repo: repo, resumen: resumen, cola: cola, log: log}
}

func (w *RescatarWorker) Work(ctx context.Context, _ *river.Job[RescatarArgs]) error {
	comprobantes := w.rescatarComprobantes(ctx)
	resumenes := w.rescatarResumenes(ctx)
	webhooks := w.rescatarWebhooks(ctx)

	// Que esto no sea cero es señal de que algo falla aguas arriba.
	if comprobantes+resumenes+webhooks > 0 {
		w.log.Infow("barrido de huerfanos",
			"comprobantes", comprobantes, "resumenes", resumenes, "webhooks", webhooks)
	}

	return nil
}

func (w *RescatarWorker) rescatarComprobantes(ctx context.Context) int {
	huerfanos, err := w.repo.Huerfanos(ctx, antiguedadHuerfano, limiteRescate)
	if err != nil {
		w.log.Errorw("no se pudieron buscar comprobantes huerfanos", "error", err)
		return 0
	}

	n := 0
	for _, c := range huerfanos {
		if err := w.cola.EncolarEmision(ctx, c.ID()); err != nil {
			w.log.Errorw("no se pudo reencolar comprobante huerfano",
				"comprobante_id", c.ID(), "numero", c.Numero(), "error", err)
			continue
		}
		w.log.Infow("comprobante huerfano reencolado",
			"comprobante_id", c.ID(), "numero", c.Numero(),
			"estado", string(c.Estado()), "intentos", c.Intentos())
		n++
	}
	return n
}

// Un resumen colgado arrastra a todas sus boletas: ninguna llega a SUNAT hasta
// que el resumen avance.
func (w *RescatarWorker) rescatarResumenes(ctx context.Context) int {
	huerfanos, err := w.resumen.ResumenesEnCurso(ctx, antiguedadHuerfano, limiteRescate)
	if err != nil {
		w.log.Errorw("no se pudieron buscar resumenes huerfanos", "error", err)
		return 0
	}

	n := 0
	for _, r := range huerfanos {
		var encolar error

		// Con ticket ya se envio: lo que falta es consultarlo.
		if r.Estado() == domain.EstadoTicketPendiente && r.Ticket() != "" {
			encolar = w.cola.EncolarConsultaTicket(ctx, r.ID())
		} else {
			encolar = w.cola.EncolarEnvioResumen(ctx, r.ID())
		}

		if encolar != nil {
			w.log.Errorw("no se pudo reencolar resumen huerfano",
				"resumen_id", r.ID(), "resumen", r.Identificador(), "error", encolar)
			continue
		}

		w.log.Infow("resumen huerfano reencolado",
			"resumen_id", r.ID(), "resumen", r.Identificador(),
			"estado", string(r.Estado()), "intentos", r.Intentos())
		n++
	}
	return n
}

// Sin esto, un webhook que agota sus reintentos deja al cliente sin enterarse
// nunca de que su comprobante quedo resuelto.
func (w *RescatarWorker) rescatarWebhooks(ctx context.Context) int {
	pendientes, err := w.repo.SinNotificar(ctx, antiguedadWebhook, limiteRescate)
	if err != nil {
		w.log.Errorw("no se pudieron buscar webhooks pendientes", "error", err)
		return 0
	}

	n := 0
	for _, c := range pendientes {
		if err := w.cola.EncolarNotificacion(ctx, c.TenantID(), c.ID()); err != nil {
			w.log.Errorw("no se pudo reencolar notificacion",
				"comprobante_id", c.ID(), "numero", c.Numero(), "error", err)
			continue
		}
		n++
	}
	return n
}
