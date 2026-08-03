package worker

import (
	"context"

	"facturacion-service/internal/features/comprobante/application/command"

	"github.com/riverqueue/river"
)

type NotificarArgs struct {
	TenantID      string `json:"tenant_id"`
	ComprobanteID string `json:"comprobante_id"`
}

func (NotificarArgs) Kind() string { return "comprobante.notificar" }

func (a NotificarArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		// El reintento del webhook es de River: si el cliente esta caido, se
		// vuelve a intentar con backoff hasta MaxAttempts.
		MaxAttempts: 10,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: estadosActivos()},
	}
}

type NotificarWorker struct {
	river.WorkerDefaults[NotificarArgs]
	notificar *command.Notificar
}

func NewNotificarWorker(notificar *command.Notificar) *NotificarWorker {
	return &NotificarWorker{notificar: notificar}
}

func (w *NotificarWorker) Work(ctx context.Context, job *river.Job[NotificarArgs]) error {
	return w.notificar.Execute(ctx, job.Args.TenantID, job.Args.ComprobanteID)
}
