package command

import (
	"context"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
)

type Procesar struct {
	repo    domain.Repositorio
	tenants domain.TenantRepositorio
	motor   domain.Motor
	avisos  domain.Avisador
}

func NewProcesar(repo domain.Repositorio, tenants domain.TenantRepositorio, motor domain.Motor, avisos domain.Avisador) *Procesar {
	return &Procesar{repo: repo, tenants: tenants, motor: motor, avisos: avisos}
}

// Execute es idempotente: solo un worker toma el comprobante y los demas se
// retiran sin reenviarlo a SUNAT.
func (uc *Procesar) Execute(ctx context.Context, comprobanteID string) error {
	c, err := uc.repo.Tomar(ctx, comprobanteID)
	if err != nil {
		if domainerr.IsKind(err, domainerr.KindConflict) {
			return uc.cerrarOReprogramar(ctx, comprobanteID)
		}
		return err
	}

	tenant, err := uc.tenants.PorID(ctx, c.TenantID())
	if err != nil {
		return err
	}

	// Las boletas no se envian individualmente: se firman y esperan al resumen
	// diario. Enviarlas sueltas seria un rechazo seguro de SUNAT.
	if domain.ViajaPorResumen(c.TipoDoc(), c.Serie()) {
		return uc.firmarParaResumen(ctx, c, tenant)
	}

	res, err := uc.motor.Emitir(ctx, tenant, c.Payload(), c.TipoDoc(), c.Serie(), c.Correlativo(), c.FechaEmision())
	if err != nil {
		c.MarcarError(err.Error())
		if updErr := uc.repo.Actualizar(ctx, c); updErr != nil {
			return updErr
		}
		return err
	}

	// El motor solo devuelve ticket al enviar un resumen; un comprobante
	// individual nunca deberia traerlo. Si llega, es una anomalia del motor y
	// NADIE consulta ese ticket: no hay worker para comprobantes en
	// 'ticket_pendiente' ni el barrido cubre ese estado. Marcarlo como error lo
	// deja retomable y visible en /atencion, en vez de dejarlo en un estado sin
	// salida.
	if res.Ticket != "" {
		c.MarcarError("el motor devolvio ticket para un comprobante individual: " + res.Ticket)
	} else {
		c.Resolver(res.Codigo, res.Mensaje, res.XML, res.CDR)
	}

	if err := uc.repo.Actualizar(ctx, c); err != nil {
		return err
	}

	// SUNAT no llego a pronunciarse sobre el documento. El comprobante quedo
	// retomable; devolver error hace que River reintente con backoff en vez de
	// cerrar el job y dejarlo esperando al barrido.
	if c.Estado() == domain.EstadoError {
		return domain.ErrMotor(res.Codigo + ": " + res.Mensaje)
	}

	return uc.avisar(ctx, c)
}

// El aviso se encola aparte para que un webhook caido no marque como fallida
// una emision que SUNAT ya acepto.
func (uc *Procesar) avisar(ctx context.Context, c *domain.Comprobante) error {
	if !c.Estado().EsFinal() {
		return nil
	}
	return uc.avisos.EncolarNotificacion(ctx, c.TenantID(), c.ID())
}

func (uc *Procesar) firmarParaResumen(ctx context.Context, c *domain.Comprobante, tenant *domain.Tenant) error {
	res, err := uc.motor.Firmar(ctx, tenant, c.Payload(), c.TipoDoc(), c.Serie(), c.Correlativo(), c.FechaEmision())
	if err != nil {
		c.MarcarError(err.Error())
		if updErr := uc.repo.Actualizar(ctx, c); updErr != nil {
			return updErr
		}
		return err
	}

	c.MarcarPendienteResumen(res.XML)
	return uc.repo.Actualizar(ctx, c)
}

// No se pudo tomar el comprobante. Si ya llego a un estado final, el trabajo
// esta hecho y el job se cierra. Si no, sigue en manos de otro worker que puede
// morir, asi que el job debe volver: es lo unico que mantiene viva la condicion
// de rescate por timeout.
func (uc *Procesar) cerrarOReprogramar(ctx context.Context, comprobanteID string) error {
	estado, err := uc.repo.EstadoActual(ctx, comprobanteID)
	if err != nil {
		return err
	}

	if estado.EsFinal() {
		return nil
	}

	return domain.ErrEnVuelo()
}
