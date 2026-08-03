package command

import (
	"context"
	"encoding/json"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
)

// EnviarResumen envia a SUNAT un resumen ya creado. Esta separado de
// ArmarResumen para que el envio sea reintentable por si solo: si falla la red,
// el resumen y sus boletas ya estan asociados y basta con reenviar.
type EnviarResumen struct {
	resumen domain.ResumenRepositorio
	tenants domain.TenantRepositorio
	motor   domain.Motor
}

func NewEnviarResumen(resumen domain.ResumenRepositorio, tenants domain.TenantRepositorio, motor domain.Motor) *EnviarResumen {
	return &EnviarResumen{resumen: resumen, tenants: tenants, motor: motor}
}

func (uc *EnviarResumen) Execute(ctx context.Context, resumenID string) error {
	res, err := uc.resumen.TomarResumen(ctx, resumenID)
	if err != nil {
		if domainerr.IsKind(err, domainerr.KindConflict) {
			return uc.cerrarOReprogramar(ctx, resumenID)
		}
		return err
	}

	tenant, err := uc.tenants.PorID(ctx, res.TenantID())
	if err != nil {
		return err
	}

	comprobantes, err := uc.resumen.ComprobantesDeResumen(ctx, res.ID())
	if err != nil {
		return err
	}
	if len(comprobantes) == 0 {
		return domain.ErrSinBoletas()
	}

	resultado, err := uc.enviarSegunTipo(ctx, tenant, res, comprobantes)
	if err != nil {
		res.MarcarError(err.Error())
		if updErr := uc.resumen.ActualizarResumen(ctx, res); updErr != nil {
			return updErr
		}
		return err
	}

	if resultado.Ticket != "" {
		res.MarcarTicket(resultado.Ticket, resultado.XML)
	} else {
		res.Resolver(resultado.Codigo, resultado.Mensaje, resultado.CDR)
	}

	return uc.resumen.ActualizarResumen(ctx, res)
}

func (uc *EnviarResumen) enviarSegunTipo(
	ctx context.Context,
	tenant *domain.Tenant,
	res *domain.Resumen,
	comprobantes []*domain.Comprobante,
) (*domain.ResultadoEmision, error) {
	if res.Tipo() == domain.ResumenBaja {
		detalles := make([]domain.DetalleBaja, len(comprobantes))
		for i, c := range comprobantes {
			detalles[i] = domain.DetalleBaja{
				TipoDoc:     string(c.TipoDoc()),
				Serie:       c.Serie(),
				Correlativo: c.Correlativo(),
				Motivo:      c.MotivoBaja(),
			}
		}
		return uc.motor.EnviarBaja(ctx, tenant, res, detalles)
	}

	detalles := make([]domain.DetalleResumen, len(comprobantes))
	for i, c := range comprobantes {
		d, err := detalleDe(c)
		if err != nil {
			return nil, err
		}
		detalles[i] = d
	}
	return uc.motor.EnviarResumen(ctx, tenant, res, detalles)
}

func (uc *EnviarResumen) cerrarOReprogramar(ctx context.Context, resumenID string) error {
	res, err := uc.resumen.ResumenPorID(ctx, resumenID)
	if err != nil {
		return err
	}

	// Ya tiene ticket o llego a un estado final: el envio esta hecho.
	if res.Estado().EsFinal() || res.Estado() == domain.EstadoTicketPendiente {
		return nil
	}

	return domain.ErrEnVuelo()
}

// detalleDe extrae del payload lo que el RC necesita de cada boleta.
func detalleDe(b *domain.Comprobante) (domain.DetalleResumen, error) {
	var doc struct {
		Receptor struct {
			TipoDoc string `json:"tipo_doc"`
			NumDoc  string `json:"num_doc"`
		} `json:"receptor"`
		Totales struct {
			OperGravadas json.Number `json:"oper_gravadas"`
			IGV          json.Number `json:"igv"`
		} `json:"totales"`
	}

	if err := json.Unmarshal(b.Payload(), &doc); err != nil {
		return domain.DetalleResumen{}, domain.ErrPayloadInvalido(err.Error())
	}

	estado := domain.DetalleAdicionar
	if b.Estado() == domain.EstadoAnulado {
		estado = domain.DetalleAnular
	}

	return domain.DetalleResumen{
		TipoDoc:       string(b.TipoDoc()),
		SerieNumero:   b.Numero(),
		Estado:        estado,
		ClienteTipo:   doc.Receptor.TipoDoc,
		ClienteNumero: doc.Receptor.NumDoc,
		Total:         b.ImporteTotal(),
		OperGravadas:  doc.Totales.OperGravadas.String(),
		IGV:           doc.Totales.IGV.String(),
	}, nil
}
