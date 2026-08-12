package domain

import domainerr "facturacion-service/internal/shared/domain/errors"

func ErrNoEncontrado() *domainerr.Error {
	return domainerr.NotFound("El comprobante no existe").WithCode("COMPROBANTE_NOT_FOUND")
}

func ErrTenantNoEncontrado() *domainerr.Error {
	return domainerr.NotFound("El emisor no existe").WithCode("TENANT_NOT_FOUND")
}

func ErrTipoDocInvalido(t string) *domainerr.Error {
	return domainerr.Validation("Tipo de comprobante no soportado: " + t).
		WithCode("TIPO_DOC_INVALIDO").
		WithSuggestion("Use 01 (factura), 03 (boleta), 07 (nota de credito) o 08 (nota de debito)")
}

func ErrSerieNoConfigurada(serie string) *domainerr.Error {
	return domainerr.Validation("La serie " + serie + " no esta configurada para este emisor").
		WithCode("SERIE_NO_CONFIGURADA")
}

func ErrSerieIncoherente(serie, tipoDoc string) *domainerr.Error {
	return domainerr.Validation("La serie " + serie + " no corresponde al tipo de comprobante " + tipoDoc).
		WithCode("SERIE_INCOHERENTE").
		WithSuggestion("Las facturas usan series que empiezan con F y las boletas con B")
}

func ErrFechaFutura() *domainerr.Error {
	return domainerr.Validation("La fecha de emision no puede estar en el futuro").
		WithCode("FECHA_FUTURA")
}

func ErrPayloadInvalido(detalle string) *domainerr.Error {
	return domainerr.Validation("El comprobante enviado es invalido: " + detalle).
		WithCode("PAYLOAD_INVALIDO")
}

func ErrImporteIncoherente(declarado, enPayload string) *domainerr.Error {
	return domainerr.Validation("importe_total (" + declarado + ") no coincide con totales.importe_total (" + enPayload + ")").
		WithCode("IMPORTE_INCOHERENTE").
		WithSuggestion("El monto registrado debe ser identico al del documento que se envia a SUNAT")
}

func ErrCertificadoInvalido(detalle string) *domainerr.Error {
	return domainerr.Validation("El certificado digital es invalido: " + detalle).
		WithCode("CERTIFICADO_INVALIDO").
		WithSuggestion("Debe ser un PEM con la clave privada y el certificado emitido por una entidad acreditada")
}

func ErrEmisorDuplicado(ruc string) *domainerr.Error {
	return domainerr.Conflict("Ya existe un emisor con el RUC " + ruc).
		WithCode("EMISOR_DUPLICADO")
}

func ErrNotaInvalida(detalle string) *domainerr.Error {
	return domainerr.Validation("La nota es invalida: " + detalle).
		WithCode("NOTA_INVALIDA").
		WithSuggestion("SUNAT rechazaria el comprobante y el correlativo se perderia")
}

func ErrReceptorInvalido(detalle string) *domainerr.Error {
	return domainerr.Validation("El receptor es invalido: " + detalle).
		WithCode("RECEPTOR_INVALIDO").
		WithSuggestion("SUNAT rechazaria el comprobante y el correlativo se perderia")
}

// ErrYaTomado: otro worker gano la carrera. No se puede cerrar el job sin mas,
// porque ese otro worker podria morir: hay que consultar el estado real y
// decidir entre cerrar o reprogramar.
func ErrYaTomado() *domainerr.Error {
	return domainerr.Conflict("El comprobante ya fue tomado por otro proceso").
		WithCode("COMPROBANTE_YA_TOMADO")
}

// ErrEnVuelo pide al worker que vuelva mas tarde en vez de cerrar el job.
//
// Cerrar el job aqui es la trampa: si el worker que lo tenia murio, el
// comprobante queda en 'procesando' y su job en 'completed', de modo que la
// condicion de rescate nunca se evalua porque nadie vuelve a ejecutarla.
func ErrEnVuelo() *domainerr.Error {
	return domainerr.Conflict("El comprobante esta siendo procesado por otro worker").
		WithCode("COMPROBANTE_EN_VUELO")
}

func ErrMotor(msg string) *domainerr.Error {
	return domainerr.Internal("Fallo al comunicarse con SUNAT: " + msg).WithCode("MOTOR_ERROR")
}

func ErrResumenNoEncontrado() *domainerr.Error {
	return domainerr.NotFound("El resumen no existe").WithCode("RESUMEN_NOT_FOUND")
}

func ErrSinBoletas() *domainerr.Error {
	return domainerr.Validation("No hay boletas pendientes para resumir").WithCode("SIN_BOLETAS")
}

func ErrResumenSinTicket() *domainerr.Error {
	return domainerr.Internal("El resumen no tiene ticket para consultar").WithCode("RESUMEN_SIN_TICKET")
}

func ErrYaAnulado() *domainerr.Error {
	return domainerr.Conflict("El comprobante ya fue anulado").WithCode("YA_ANULADO")
}

func ErrNoAnulable(estado string) *domainerr.Error {
	return domainerr.Conflict("Un comprobante en estado " + estado + " no puede anularse").
		WithCode("NO_ANULABLE").
		WithSuggestion("SUNAT nunca lo registro, asi que no hay nada que dar de baja")
}

func ErrAnularEnVuelo() *domainerr.Error {
	return domainerr.Conflict("El comprobante aun se esta procesando").
		WithCode("ANULAR_EN_VUELO").
		WithSuggestion("Espere a que SUNAT responda antes de anularlo")
}
