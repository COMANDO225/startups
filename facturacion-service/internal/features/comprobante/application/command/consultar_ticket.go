package command

import (
	"context"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
)

// ConsultarTicket cierra el ciclo del resumen diario: SUNAT respondio con un
// ticket y hay que preguntarle si el contenido fue aceptado.
type ConsultarTicket struct {
	resumen domain.ResumenRepositorio
	tenants domain.TenantRepositorio
	motor   domain.Motor
}

func NewConsultarTicket(resumen domain.ResumenRepositorio, tenants domain.TenantRepositorio, motor domain.Motor) *ConsultarTicket {
	return &ConsultarTicket{resumen: resumen, tenants: tenants, motor: motor}
}

// ErrTicketNoListo pide al worker que vuelva mas tarde. SUNAT puede tardar
// minutos en procesar un resumen.
func ErrTicketNoListo() *domainerr.Error {
	return domainerr.Conflict("El ticket todavia no tiene respuesta").WithCode("TICKET_NO_LISTO")
}

func (uc *ConsultarTicket) Execute(ctx context.Context, resumenID string) error {
	res, err := uc.resumen.ResumenPorID(ctx, resumenID)
	if err != nil {
		return err
	}

	if res.Estado().EsFinal() {
		return nil
	}

	if res.Ticket() == "" {
		return domain.ErrResumenSinTicket()
	}

	tenant, err := uc.tenants.PorID(ctx, res.TenantID())
	if err != nil {
		return err
	}

	resultado, err := uc.motor.ConsultarTicket(ctx, tenant, res.Ticket())
	if err != nil {
		return err
	}

	// El motor devuelve estado "pendiente" cuando SUNAT aun no termino: no es
	// un fallo, hay que reintentar mas tarde sin tocar el resumen.
	if resultado.Estado == "pendiente" {
		return ErrTicketNoListo()
	}

	res.Resolver(resultado.Codigo, resultado.Mensaje, resultado.CDR)

	if err := uc.resumen.ActualizarResumen(ctx, res); err != nil {
		return err
	}

	// El CDR del resumen resuelve de una vez todas las boletas que iban dentro.
	return uc.resumen.ResolverComprobantesDeResumen(
		ctx, res.ID(), res.Estado(), res.CodigoSunat(), res.MensajeSunat(),
	)
}
