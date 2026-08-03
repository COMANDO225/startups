package domain

import (
	"fmt"
	"time"
)

// Resumen es el sobre con el que las boletas llegan a SUNAT. A diferencia de una
// factura, SUNAT no responde con un CDR sino con un ticket que hay que consultar
// despues.
type Resumen struct {
	id           string
	tenantID     string
	tipo         TipoResumen
	fechaRef     time.Time
	correlativo  int64
	estado       Estado
	ticket       string
	xml          []byte
	cdr          []byte
	codigoSunat  string
	mensajeSunat string
	intentos     int
	tomadoAt     *time.Time
	createdAt    time.Time
	updatedAt    time.Time
}

type TipoResumen string

const (
	// ResumenDiario informa boletas emitidas.
	ResumenDiario TipoResumen = "RC"
	// ResumenBaja anula comprobantes ya informados.
	ResumenBaja TipoResumen = "RA"
)

type NuevoResumen struct {
	ID          string
	TenantID    string
	Tipo        TipoResumen
	FechaRef    time.Time
	Correlativo int64
}

func NewResumen(p NuevoResumen) *Resumen {
	now := time.Now().UTC()
	return &Resumen{
		id:          p.ID,
		tenantID:    p.TenantID,
		tipo:        p.Tipo,
		fechaRef:    p.FechaRef,
		correlativo: p.Correlativo,
		estado:      EstadoPendiente,
		createdAt:   now,
		updatedAt:   now,
	}
}

type ResumenPersistido struct {
	ID           string
	TenantID     string
	Tipo         TipoResumen
	FechaRef     time.Time
	Correlativo  int64
	Estado       Estado
	Ticket       string
	XML          []byte
	CDR          []byte
	CodigoSunat  string
	MensajeSunat string
	Intentos     int
	TomadoAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func ReconstruirResumen(p ResumenPersistido) *Resumen {
	return &Resumen{
		id:           p.ID,
		tenantID:     p.TenantID,
		tipo:         p.Tipo,
		fechaRef:     p.FechaRef,
		correlativo:  p.Correlativo,
		estado:       p.Estado,
		ticket:       p.Ticket,
		xml:          p.XML,
		cdr:          p.CDR,
		codigoSunat:  p.CodigoSunat,
		mensajeSunat: p.MensajeSunat,
		intentos:     p.Intentos,
		tomadoAt:     p.TomadoAt,
		createdAt:    p.CreatedAt,
		updatedAt:    p.UpdatedAt,
	}
}

func (r *Resumen) ID() string           { return r.id }
func (r *Resumen) TenantID() string     { return r.tenantID }
func (r *Resumen) Tipo() TipoResumen    { return r.tipo }
func (r *Resumen) FechaRef() time.Time  { return r.fechaRef }
func (r *Resumen) Correlativo() int64   { return r.correlativo }
func (r *Resumen) Estado() Estado       { return r.estado }
func (r *Resumen) Ticket() string       { return r.ticket }
func (r *Resumen) XML() []byte          { return r.xml }
func (r *Resumen) CDR() []byte          { return r.cdr }
func (r *Resumen) CodigoSunat() string  { return r.codigoSunat }
func (r *Resumen) MensajeSunat() string { return r.mensajeSunat }
func (r *Resumen) Intentos() int        { return r.intentos }
func (r *Resumen) CreatedAt() time.Time { return r.createdAt }
func (r *Resumen) UpdatedAt() time.Time { return r.updatedAt }

// Identificador tal como lo espera SUNAT: RC-20260802-1
func (r *Resumen) Identificador() string {
	return fmt.Sprintf("%s-%s-%d", r.tipo, r.fechaRef.Format("20060102"), r.correlativo)
}

// MarcarTicket: SUNAT acepto el envio y devolvio un ticket. Todavia no se sabe
// si el contenido es valido; eso lo dira la consulta posterior.
func (r *Resumen) MarcarTicket(ticket string, xml []byte) {
	r.ticket = ticket
	if len(xml) > 0 {
		r.xml = xml
	}
	r.estado = EstadoTicketPendiente
	r.updatedAt = time.Now().UTC()
}

func (r *Resumen) Resolver(codigo, mensaje string, cdr []byte) {
	if len(cdr) > 0 {
		r.cdr = cdr
	}
	r.codigoSunat = codigo
	r.mensajeSunat = mensaje
	r.estado = ClasificarCodigo(codigo)
	r.updatedAt = time.Now().UTC()
}

func (r *Resumen) MarcarError(mensaje string) {
	r.estado = EstadoError
	r.mensajeSunat = mensaje
	r.updatedAt = time.Now().UTC()
}

// EstadoDetalle indica a SUNAT que hacer con cada comprobante del resumen.
type EstadoDetalle int

const (
	DetalleAdicionar EstadoDetalle = 1
	DetalleModificar EstadoDetalle = 2
	DetalleAnular    EstadoDetalle = 3
)
