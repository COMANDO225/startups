package worker

import (
	"context"
	"time"

	"facturacion-service/internal/features/comprobante/application/command"
	domainerr "facturacion-service/internal/shared/domain/errors"
	"facturacion-service/internal/shared/transaction"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// reintentoEnVuelo debe ser menor que domain.TimeoutProcesando para que el job
// siga vivo cuando se abra la ventana de rescate.
const reintentoEnVuelo = 2 * time.Minute

// estadosActivos: River exige que la lista incluya todos los estados no
// finalizados para deduplicar correctamente.
func estadosActivos() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStatePending,
		rivertype.JobStateScheduled,
		rivertype.JobStateAvailable,
		rivertype.JobStateRunning,
		rivertype.JobStateRetryable,
	}
}

type EmitirArgs struct {
	ComprobanteID string `json:"comprobante_id"`
}

func (EmitirArgs) Kind() string { return "comprobante.emitir" }

// InsertOpts evita que el barredor acumule jobs duplicados: mientras haya uno
// pendiente para el mismo comprobante, reencolarlo es un no-op.
func (a EmitirArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: estadosActivos(),
		},
	}
}

type EmitirWorker struct {
	river.WorkerDefaults[EmitirArgs]
	procesar *command.Procesar
}

func NewEmitirWorker(procesar *command.Procesar) *EmitirWorker {
	return &EmitirWorker{procesar: procesar}
}

func (w *EmitirWorker) Work(ctx context.Context, job *river.Job[EmitirArgs]) error {
	err := w.procesar.Execute(ctx, job.Args.ComprobanteID)

	// El comprobante lo tiene otro worker. Cerrar el job aqui lo dejaria
	// huerfano si ese worker muere, asi que se reprograma.
	if domErr, ok := domainerr.AsError(err); ok && domErr.Code() == "COMPROBANTE_EN_VUELO" {
		return river.JobSnooze(reintentoEnVuelo)
	}

	return err
}

// Encolador inserta el job dentro de la transaccion activa. Si el commit falla,
// el job desaparece con el resto de la transaccion: sin dual-write.
type Encolador struct {
	client *river.Client[pgx.Tx]
}

func NewEncolador(client *river.Client[pgx.Tx]) *Encolador {
	return &Encolador{client: client}
}

func (e *Encolador) EncolarEnvioResumen(ctx context.Context, resumenID string) error {
	args := EnviarResumenArgs{ResumenID: resumenID}

	// Igual que la emision: dentro de la transaccion si la hay, para que el job
	// no exista si el commit falla.
	if tx, ok := transaction.TxFromContext(ctx); ok {
		_, err := e.client.InsertTx(ctx, tx, args, nil)
		return err
	}

	_, err := e.client.Insert(ctx, args, nil)
	return err
}

func (e *Encolador) EncolarConsultaTicket(ctx context.Context, resumenID string) error {
	_, err := e.client.Insert(ctx, ConsultarTicketArgs{ResumenID: resumenID}, nil)
	return err
}

func (e *Encolador) EncolarNotificacion(ctx context.Context, tenantID, comprobanteID string) error {
	_, err := e.client.Insert(ctx, NotificarArgs{TenantID: tenantID, ComprobanteID: comprobanteID}, nil)
	return err
}

func (e *Encolador) EncolarEmision(ctx context.Context, comprobanteID string) error {
	args := EmitirArgs{ComprobanteID: comprobanteID}

	tx, ok := transaction.TxFromContext(ctx)
	if !ok {
		_, err := e.client.Insert(ctx, args, nil)
		return err
	}

	_, err := e.client.InsertTx(ctx, tx, args, nil)
	return err
}
