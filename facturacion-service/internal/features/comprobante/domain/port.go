package domain

import (
	"context"
	"time"
)

type Repositorio interface {
	// SiguienteCorrelativo incrementa el contador de la serie y devuelve el
	// nuevo valor. Debe ejecutarse dentro de la transaccion que crea el
	// comprobante: si el worker lo asignara, un reintento generaria un numero
	// distinto y SUNAT rechazaria la numeracion duplicada.
	SiguienteCorrelativo(ctx context.Context, tenantID string, tipoDoc TipoDoc, serie string) (int64, error)

	Crear(ctx context.Context, c *Comprobante) error
	PorID(ctx context.Context, tenantID, id string) (*Comprobante, error)
	PorIdempotencyKey(ctx context.Context, tenantID, key string) (*Comprobante, error)
	Listar(ctx context.Context, tenantID string, limit, offset int) ([]*Comprobante, int, error)

	// Tomar mueve el comprobante a 'procesando' solo si esta en un estado
	// tomable. Devuelve ErrYaTomado si otro worker gano la carrera.
	Tomar(ctx context.Context, id string) (*Comprobante, error)

	Actualizar(ctx context.Context, c *Comprobante) error

	EstadoActual(ctx context.Context, id string) (Estado, error)
	MarcarWebhookEnviado(ctx context.Context, id string) error
	MarcarAnulado(ctx context.Context, id, motivo string) error

	// SinNotificar: llegaron a estado final pero su webhook nunca se entrego.
	SinNotificar(ctx context.Context, antiguedad time.Duration, limite int) ([]*Comprobante, error)

	// Huerfanos: comprobantes sin avanzar hace rato, cuyo job pudo cerrarse o
	// descartarse. Es la red de seguridad del pipeline.
	Huerfanos(ctx context.Context, antiguedad time.Duration, limite int) ([]*Comprobante, error)

	// RequierenAtencion: agotaron reintentos o SUNAT pide decision humana.
	RequierenAtencion(ctx context.Context, tenantID string, limite int) ([]*Comprobante, error)
}

// ResumenRepositorio agrupa lo que necesita el ciclo del resumen diario.
type ResumenRepositorio interface {
	SiguienteCorrelativoResumen(ctx context.Context, tenantID string, tipo TipoResumen, fechaRef time.Time) (int64, error)
	CrearResumen(ctx context.Context, r *Resumen) error
	ResumenPorID(ctx context.Context, id string) (*Resumen, error)
	TomarResumen(ctx context.Context, id string) (*Resumen, error)
	ActualizarResumen(ctx context.Context, r *Resumen) error
	ResumenesEnCurso(ctx context.Context, antiguedad time.Duration, limite int) ([]*Resumen, error)

	// GruposPendientes devuelve las combinaciones tenant+fecha con boletas
	// esperando resumen: cada una produce un RC.
	GruposPendientes(ctx context.Context, limite int) ([]GrupoBoletas, error)
	BoletasParaResumen(ctx context.Context, tenantID string, fecha time.Time, limite int) ([]*Comprobante, error)
	AsignarResumen(ctx context.Context, resumenID string, comprobanteIDs []string) error
	ResolverComprobantesDeResumen(ctx context.Context, resumenID string, estado Estado, codigo, mensaje string) error
	ComprobantesDeResumen(ctx context.Context, resumenID string) ([]*Comprobante, error)
}

// EncoladorResumen separa la creacion del resumen de su envio: el envio es una
// llamada de red y debe poder reintentarse sin rehacer el resumen.
type EncoladorResumen interface {
	EncolarEnvioResumen(ctx context.Context, resumenID string) error
	EncolarConsultaTicket(ctx context.Context, resumenID string) error
}

type GrupoBoletas struct {
	TenantID string
	Fecha    time.Time
}

type Tenant struct {
	ID              string
	RUC             string
	RazonSocial     string
	NombreComercial string
	Direccion       string
	Ubigeo          string
	Departamento    string
	Provincia       string
	Distrito        string
	CertPEM         string
	SolUser         string
	SolPass         string
	Produccion      bool
	WebhookURL      string
	WebhookSecret   string
}

type TenantRepositorio interface {
	PorID(ctx context.Context, id string) (*Tenant, error)
	PorAPIKey(ctx context.Context, apiKey string) (*Tenant, error)
}

type ResultadoEmision struct {
	Nombre        string
	XML           []byte
	CDR           []byte
	Estado        string
	Codigo        string
	Mensaje       string
	Ticket        string
	Observaciones []string
}

// Motor abstrae el contenedor que construye el XML UBL, lo firma y lo envia a
// SUNAT. Hoy lo implementa Greenter sobre PHP.
type Motor interface {
	Emitir(ctx context.Context, t *Tenant, payload []byte, tipoDoc TipoDoc, serie string, correlativo int64, fecha time.Time) (*ResultadoEmision, error)

	// Firmar genera el XML de una boleta sin enviarlo: viaja en el resumen.
	Firmar(ctx context.Context, t *Tenant, payload []byte, tipoDoc TipoDoc, serie string, correlativo int64, fecha time.Time) (*ResultadoEmision, error)

	EnviarResumen(ctx context.Context, t *Tenant, r *Resumen, detalles []DetalleResumen) (*ResultadoEmision, error)
	EnviarBaja(ctx context.Context, t *Tenant, r *Resumen, detalles []DetalleBaja) (*ResultadoEmision, error)
	ConsultarTicket(ctx context.Context, t *Tenant, ticket string) (*ResultadoEmision, error)
}

// DetalleResumen es una linea del RC: un comprobante informado a SUNAT.
type DetalleResumen struct {
	TipoDoc       string
	SerieNumero   string
	Estado        EstadoDetalle
	ClienteTipo   string
	ClienteNumero string
	Total         string
	OperGravadas  string
	IGV           string
}

type DetalleBaja struct {
	TipoDoc     string
	Serie       string
	Correlativo int64
	Motivo      string
}

// Avisador encola la notificacion al cliente. Se encola en vez de enviarse en
// linea para que un webhook lento no bloquee el pipeline de emision.
type Avisador interface {
	EncolarNotificacion(ctx context.Context, tenantID, comprobanteID string) error
}

// EnviadorWebhook entrega la notificacion. La firma HMAC viaja aparte para que
// el cliente pueda verificar que el aviso salio de aqui.
type EnviadorWebhook interface {
	Enviar(ctx context.Context, url string, cuerpo []byte, firma string) error
}

// Encolador permite que el caso de uso encole el trabajo dentro de la misma
// transaccion que crea el comprobante, evitando el dual-write.
type Encolador interface {
	EncolarEmision(ctx context.Context, comprobanteID string) error
}

// SaludPipeline es la foto que necesitan las metricas y la alerta: cuantos
// comprobantes hay en cada estado, cuantos requieren intervencion humana y
// cuanto lleva esperando el mas antiguo sin resolver.
type SaludPipeline struct {
	PorEstado          map[Estado]int64
	RequierenAtencion  int64
	AntiguedadMasViejo time.Duration
}

// SerieConfig es una serie habilitada para un emisor. Sin al menos una, no
// puede emitir nada.
type SerieConfig struct {
	TipoDoc TipoDoc
	Serie   string
}

// NuevoTenant junta todo lo que hace falta para dar de alta un emisor. El
// certificado y la clave SOL viajan en claro hasta el repositorio, que es donde
// se cifran antes de tocar la base.
type NuevoTenant struct {
	Tenant     Tenant
	APIKeyHash string
	Series     []SerieConfig
}

// AltaEmisor separa la escritura de tenants de la lectura: solo el alta la
// necesita, y es la unica operacion que recibe secretos en claro.
type AltaEmisor interface {
	CrearTenant(ctx context.Context, nuevo NuevoTenant) error
}
