package domain

import (
	"strings"
	"time"
)

type Estado string

const (
	EstadoPendiente        Estado = "pendiente"
	EstadoProcesando       Estado = "procesando"
	EstadoPendienteResumen Estado = "pendiente_resumen"
	EstadoEnviado          Estado = "enviado"
	EstadoTicketPendiente  Estado = "ticket_pendiente"
	EstadoAceptado         Estado = "aceptado"
	EstadoObservado        Estado = "observado"
	EstadoRechazado        Estado = "rechazado"
	EstadoDuplicado        Estado = "duplicado"
	EstadoAnulado          Estado = "anulado"
	EstadoError            Estado = "error"
)

// TodosLosEstados alimenta las metricas. Se emiten todos, incluso en cero: un
// estado que aparece y desaparece entre scrapes deja huecos en las graficas.
func TodosLosEstados() []Estado {
	return []Estado{
		EstadoPendiente, EstadoProcesando, EstadoPendienteResumen, EstadoEnviado,
		EstadoTicketPendiente, EstadoAceptado, EstadoObservado, EstadoRechazado,
		EstadoDuplicado, EstadoAnulado, EstadoError,
	}
}

// EsFinal indica que SUNAT ya resolvio el comprobante y no debe reprocesarse.
func (e Estado) EsFinal() bool {
	switch e {
	case EstadoAceptado, EstadoObservado, EstadoRechazado, EstadoDuplicado, EstadoAnulado:
		return true
	}
	return false
}

// RequiereRevision marca los estados que ningun reintento arregla y que alguien
// tiene que mirar.
func (e Estado) RequiereRevision() bool {
	return e == EstadoDuplicado
}

// EstadosTomables son los estados desde los que un worker puede tomar el
// comprobante. La transicion se hace con UPDATE ... WHERE estado IN (...),
// de modo que solo un worker gana aunque River entregue el job dos veces.
func EstadosTomables() []string {
	return []string{string(EstadoPendiente), string(EstadoError)}
}

// TimeoutProcesando: pasado este tiempo se asume que el worker que tomo el
// comprobante murio, y otro puede retomarlo. Debe ser mayor que el timeout del
// motor para no arrebatarle un trabajo que sigue en curso.
const TimeoutProcesando = 5 * time.Minute

// MaxIntentos: pasado este numero, reintentar deja de tener sentido. El
// comprobante pasa a requerir atencion humana en vez de seguir rebotando; sin
// este tope, un payload invalido se reencola para siempre.
const MaxIntentos = 10

type TipoDoc string

const (
	TipoFactura     TipoDoc = "01"
	TipoBoleta      TipoDoc = "03"
	TipoNotaCredito TipoDoc = "07"
	TipoNotaDebito  TipoDoc = "08"
)

func (t TipoDoc) Valido() bool {
	switch t {
	case TipoFactura, TipoBoleta, TipoNotaCredito, TipoNotaDebito:
		return true
	}
	return false
}

// EsSincrono distingue las facturas (CDR inmediato) de las boletas, que viajan
// en un resumen diario y devuelven un ticket a consultar despues.
//
// Una nota de credito sobre una boleta tambien viaja por resumen, por eso no
// alcanza el tipo: hace falta mirar la serie.
func (t TipoDoc) EsSincrono() bool {
	return t != TipoBoleta
}

// ViajaPorResumen: las boletas y las notas que las modifican no se envian
// individualmente a SUNAT, se informan en el Resumen Diario.
func ViajaPorResumen(t TipoDoc, serie string) bool {
	if t == TipoBoleta {
		return true
	}
	if t == TipoNotaCredito || t == TipoNotaDebito {
		return len(serie) > 0 && strings.EqualFold(serie[:1], "B")
	}
	return false
}

// PrefijoSerieValido: SUNAT exige que la serie de una factura empiece con F y la
// de una boleta con B. Se valida antes de asignar correlativo para no quemar un
// numero en un documento que SUNAT va a rechazar igual.
func (t TipoDoc) PrefijoSerieValido(serie string) bool {
	if len(serie) != 4 {
		return false
	}

	prefijo := strings.ToUpper(serie[:1])
	switch t {
	case TipoFactura:
		return prefijo == "F"
	case TipoBoleta:
		return prefijo == "B"
	case TipoNotaCredito, TipoNotaDebito:
		// La nota hereda el prefijo del documento que modifica.
		return prefijo == "F" || prefijo == "B"
	}
	return false
}

// codigosYaRegistrado: SUNAT responde asi cuando ya tiene el comprobante. Pasa
// cuando reenviamos algo que si se envio pero cuyo CDR no llegamos a guardar.
// No es un rechazo (SUNAT lo tiene) ni una aceptacion limpia ("con otros
// datos"), por eso va a un estado propio que exige revision humana.
var codigosYaRegistrado = map[string]bool{
	"1033": true,
	"2109": true,
}

// ClasificarCodigo mapea el ResponseCode del CDR a un estado.
//
//	no numerico  error (fallo de transporte, reintentable)
//	0            aceptado
//	4xxx         observado (aceptado con advertencias)
//	resto        rechazado
//
// El default es rechazado a proposito: ante un codigo desconocido es mucho peor
// dar por bueno un comprobante que SUNAT no acepto, que marcar como rechazado
// uno que si paso.
//
// Pero un codigo no numerico no viene de SUNAT. Greenter extrae los digitos del
// SoapFault (preg_replace '/[^0-9]+/') y solo devuelve el codigo crudo cuando no
// encontro ninguno: "HTTP" en un 401, "SOAP-ENV:Server" en una caida. Eso es un
// fallo de transporte, no un rechazo del documento. Clasificarlo como rechazado
// mataba el comprobante y quemaba el correlativo por un problema de red.
func ClasificarCodigo(codigo string) Estado {
	switch {
	case !soloDigitos(codigo):
		return EstadoError
	case codigo == "0":
		return EstadoAceptado
	case codigosYaRegistrado[codigo]:
		return EstadoDuplicado
	case strings.HasPrefix(codigo, "4"):
		return EstadoObservado
	default:
		return EstadoRechazado
	}
}

// El vacio cuenta como no numerico: si el motor no devolvio codigo, no sabemos
// que dijo SUNAT y reintentar es mas seguro que dar por muerto el comprobante.
func soloDigitos(s string) bool {
	return s != "" && strings.TrimLeft(s, "0123456789") == ""
}
