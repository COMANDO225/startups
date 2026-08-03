package domain

import (
	"fmt"
	"time"
)

type Comprobante struct {
	id             string
	tenantID       string
	idempotencyKey string
	tipoDoc        TipoDoc
	serie          string
	correlativo    int64
	estado         Estado
	payload        []byte
	moneda         string
	importeTotal   string
	fechaEmision   time.Time
	xml            []byte
	cdr            []byte
	ticket         string
	codigoSunat    string
	mensajeSunat   string
	intentos       int
	tomadoAt       *time.Time
	createdAt      time.Time
	updatedAt      time.Time
	motivoBaja     string
}

type NuevoComprobante struct {
	ID             string
	TenantID       string
	IdempotencyKey string
	TipoDoc        TipoDoc
	Serie          string
	Correlativo    int64
	Payload        []byte
	Moneda         string
	ImporteTotal   string
	FechaEmision   time.Time
}

func New(p NuevoComprobante) *Comprobante {
	now := time.Now().UTC()
	return &Comprobante{
		id:             p.ID,
		tenantID:       p.TenantID,
		idempotencyKey: p.IdempotencyKey,
		tipoDoc:        p.TipoDoc,
		serie:          p.Serie,
		correlativo:    p.Correlativo,
		estado:         EstadoPendiente,
		payload:        p.Payload,
		moneda:         p.Moneda,
		importeTotal:   p.ImporteTotal,
		fechaEmision:   p.FechaEmision,
		createdAt:      now,
		updatedAt:      now,
	}
}

// Reconstruir arma la entidad desde la base de datos. Recibe un struct y no
// parametros posicionales para que no se puedan intercambiar por error.
type EstadoPersistido struct {
	ID             string
	TenantID       string
	IdempotencyKey string
	TipoDoc        TipoDoc
	Serie          string
	Correlativo    int64
	Estado         Estado
	Payload        []byte
	Moneda         string
	ImporteTotal   string
	FechaEmision   time.Time
	XML            []byte
	CDR            []byte
	Ticket         string
	CodigoSunat    string
	MensajeSunat   string
	Intentos       int
	TomadoAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	MotivoBaja     string
}

func Reconstruir(p EstadoPersistido) *Comprobante {
	return &Comprobante{
		id:             p.ID,
		tenantID:       p.TenantID,
		idempotencyKey: p.IdempotencyKey,
		tipoDoc:        p.TipoDoc,
		serie:          p.Serie,
		correlativo:    p.Correlativo,
		estado:         p.Estado,
		payload:        p.Payload,
		moneda:         p.Moneda,
		importeTotal:   p.ImporteTotal,
		fechaEmision:   p.FechaEmision,
		xml:            p.XML,
		cdr:            p.CDR,
		ticket:         p.Ticket,
		codigoSunat:    p.CodigoSunat,
		mensajeSunat:   p.MensajeSunat,
		intentos:       p.Intentos,
		tomadoAt:       p.TomadoAt,
		createdAt:      p.CreatedAt,
		updatedAt:      p.UpdatedAt,
		motivoBaja:     p.MotivoBaja,
	}
}

func (c *Comprobante) ID() string              { return c.id }
func (c *Comprobante) TenantID() string        { return c.tenantID }
func (c *Comprobante) IdempotencyKey() string  { return c.idempotencyKey }
func (c *Comprobante) TipoDoc() TipoDoc        { return c.tipoDoc }
func (c *Comprobante) Serie() string           { return c.serie }
func (c *Comprobante) Correlativo() int64      { return c.correlativo }
func (c *Comprobante) Estado() Estado          { return c.estado }
func (c *Comprobante) Payload() []byte         { return c.payload }
func (c *Comprobante) Moneda() string          { return c.moneda }
func (c *Comprobante) ImporteTotal() string    { return c.importeTotal }
func (c *Comprobante) FechaEmision() time.Time { return c.fechaEmision }
func (c *Comprobante) XML() []byte             { return c.xml }
func (c *Comprobante) CDR() []byte             { return c.cdr }
func (c *Comprobante) Ticket() string          { return c.ticket }
func (c *Comprobante) CodigoSunat() string     { return c.codigoSunat }
func (c *Comprobante) MensajeSunat() string    { return c.mensajeSunat }
func (c *Comprobante) Intentos() int           { return c.intentos }
func (c *Comprobante) CreatedAt() time.Time    { return c.createdAt }
func (c *Comprobante) UpdatedAt() time.Time    { return c.updatedAt }
func (c *Comprobante) MotivoBaja() string      { return c.motivoBaja }

// Numero es el identificador legal del comprobante: F001-123.
func (c *Comprobante) Numero() string {
	return fmt.Sprintf("%s-%d", c.serie, c.correlativo)
}

func (c *Comprobante) Resolver(codigo, mensaje string, xml, cdr []byte) {
	if len(xml) > 0 {
		c.xml = xml
	}
	if len(cdr) > 0 {
		c.cdr = cdr
	}
	c.codigoSunat = codigo
	c.mensajeSunat = mensaje
	c.estado = ClasificarCodigo(codigo)
	c.updatedAt = time.Now().UTC()
}

func (c *Comprobante) MarcarTicket(ticket string, xml []byte) {
	c.ticket = ticket
	c.xml = xml
	c.estado = EstadoTicketPendiente
	c.updatedAt = time.Now().UTC()
}

// MarcarPendienteResumen: la boleta ya tiene XML firmado y espera que un
// resumen diario la lleve a SUNAT.
func (c *Comprobante) MarcarPendienteResumen(xml []byte) {
	if len(xml) > 0 {
		c.xml = xml
	}
	c.estado = EstadoPendienteResumen
	c.updatedAt = time.Now().UTC()
}

// MarcarAnulado deja la entidad coherente con lo que se acaba de escribir en la
// base: sin esto la respuesta al cliente devuelve el estado anterior.
func (c *Comprobante) MarcarAnulado(motivo string) {
	c.estado = EstadoAnulado
	c.motivoBaja = motivo
	c.updatedAt = time.Now().UTC()
}

func (c *Comprobante) MarcarError(mensaje string) {
	c.estado = EstadoError
	c.mensajeSunat = mensaje
	c.updatedAt = time.Now().UTC()
}
