package command

import (
	"context"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/ulid"
)

const maxBoletasPorResumen = 500

// ArmarResumen junta las boletas pendientes de un emisor y una fecha, crea el
// Resumen Diario y encola su envio.
//
// Crear y enviar estan separados a proposito: el envio es una llamada de red que
// puede fallar, y si formara parte de esta transaccion un fallo dejaria las
// boletas sin resumen. Asi el resumen queda persistido y el envio se reintenta.
type ArmarResumen struct {
	resumen domain.ResumenRepositorio
	cola    domain.EncoladorResumen
	tx      transaction.Transactor
}

func NewArmarResumen(resumen domain.ResumenRepositorio, cola domain.EncoladorResumen, tx transaction.Transactor) *ArmarResumen {
	return &ArmarResumen{resumen: resumen, cola: cola, tx: tx}
}

func (uc *ArmarResumen) Execute(ctx context.Context, tenantID string, fecha time.Time) (*domain.Resumen, error) {
	var res *domain.Resumen

	err := uc.tx.RunInTx(ctx, func(ctx context.Context) error {
		boletas, err := uc.resumen.BoletasParaResumen(ctx, tenantID, fecha, maxBoletasPorResumen)
		if err != nil {
			return err
		}
		if len(boletas) == 0 {
			return domain.ErrSinBoletas()
		}

		correlativo, err := uc.resumen.SiguienteCorrelativoResumen(ctx, tenantID, domain.ResumenDiario, fecha)
		if err != nil {
			return err
		}

		res = domain.NewResumen(domain.NuevoResumen{
			ID:          string(ulid.New()),
			TenantID:    tenantID,
			Tipo:        domain.ResumenDiario,
			FechaRef:    fecha,
			Correlativo: correlativo,
		})

		if err := uc.resumen.CrearResumen(ctx, res); err != nil {
			return err
		}

		ids := make([]string, len(boletas))
		for i, b := range boletas {
			ids[i] = b.ID()
		}

		if err := uc.resumen.AsignarResumen(ctx, res.ID(), ids); err != nil {
			return err
		}

		// Encolado dentro de la transaccion: si el commit falla, el job no existe.
		return uc.cola.EncolarEnvioResumen(ctx, res.ID())
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}
