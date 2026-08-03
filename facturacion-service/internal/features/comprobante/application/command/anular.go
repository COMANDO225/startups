package command

import (
	"context"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/ulid"
)

// Anular da de baja un comprobante ante SUNAT. El camino depende de como llego
// el comprobante a SUNAT, no del tipo de documento:
//
//	nunca informado   -> se marca anulado y no se informa nada
//	factura aceptada  -> Comunicacion de Baja (RA)
//	boleta aceptada   -> Resumen Diario (RC) con la linea en estado 3
type Anular struct {
	repo    domain.Repositorio
	resumen domain.ResumenRepositorio
	cola    domain.EncoladorResumen
	tx      transaction.Transactor
}

func NewAnular(
	repo domain.Repositorio,
	resumen domain.ResumenRepositorio,
	cola domain.EncoladorResumen,
	tx transaction.Transactor,
) *Anular {
	return &Anular{repo: repo, resumen: resumen, cola: cola, tx: tx}
}

type AnularCmd struct {
	TenantID      string
	ComprobanteID string
	Motivo        string
}

func (uc *Anular) Execute(ctx context.Context, cmd AnularCmd) (*domain.Comprobante, error) {
	c, err := uc.repo.PorID(ctx, cmd.TenantID, cmd.ComprobanteID)
	if err != nil {
		return nil, err
	}

	if err := puedeAnularse(c); err != nil {
		return nil, err
	}

	var resultado *domain.Comprobante

	err = uc.tx.RunInTx(ctx, func(ctx context.Context) error {
		// Nunca llego a SUNAT: no hay nada que comunicar, basta con no
		// incluirlo en ningun resumen.
		if !fueInformado(c) {
			if err := uc.repo.MarcarAnulado(ctx, c.ID(), cmd.Motivo); err != nil {
				return err
			}
			c.MarcarAnulado(cmd.Motivo)
			resultado = c
			return nil
		}

		tipo := domain.ResumenBaja
		if domain.ViajaPorResumen(c.TipoDoc(), c.Serie()) {
			tipo = domain.ResumenDiario
		}

		fecha := c.FechaEmision()

		correlativo, err := uc.resumen.SiguienteCorrelativoResumen(ctx, cmd.TenantID, tipo, fecha)
		if err != nil {
			return err
		}

		res := domain.NewResumen(domain.NuevoResumen{
			ID:          string(ulid.New()),
			TenantID:    cmd.TenantID,
			Tipo:        tipo,
			FechaRef:    fecha,
			Correlativo: correlativo,
		})

		if err := uc.resumen.CrearResumen(ctx, res); err != nil {
			return err
		}

		// El comprobante pasa a 'anulado' antes de enviarse: es lo que hace que
		// la linea del resumen viaje con estado 3 en vez de 1.
		if err := uc.repo.MarcarAnulado(ctx, c.ID(), cmd.Motivo); err != nil {
			return err
		}
		c.MarcarAnulado(cmd.Motivo)

		if err := uc.resumen.AsignarResumen(ctx, res.ID(), []string{c.ID()}); err != nil {
			return err
		}

		resultado = c
		return uc.cola.EncolarEnvioResumen(ctx, res.ID())
	})
	if err != nil {
		return nil, err
	}

	return resultado, nil
}

func puedeAnularse(c *domain.Comprobante) error {
	switch c.Estado() {
	case domain.EstadoAnulado:
		return domain.ErrYaAnulado()
	case domain.EstadoRechazado:
		// SUNAT nunca lo registro: no hay nada que dar de baja.
		return domain.ErrNoAnulable(string(c.Estado()))
	case domain.EstadoProcesando, domain.EstadoTicketPendiente:
		return domain.ErrAnularEnVuelo()
	}
	return nil
}

// fueInformado indica si SUNAT llego a registrar el comprobante. Una boleta en
// 'pendiente_resumen' todavia no viajo, aunque ya tenga su XML firmado.
func fueInformado(c *domain.Comprobante) bool {
	switch c.Estado() {
	case domain.EstadoAceptado, domain.EstadoObservado, domain.EstadoDuplicado:
		return true
	}
	return false
}
