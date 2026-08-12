package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Catalogo 06 de SUNAT: tipo de documento de identidad del receptor.
const (
	DocSinRUC    = "0"
	DocDNI       = "1"
	DocCarnetExt = "4"
	DocRUC       = "6"
	DocPasaporte = "7"
	DocCedulaDip = "A"
)

// Solo el DNI y el RUC tienen longitud fija. El pasaporte y el carnet de
// extranjeria varian, asi que de esos solo se exige que no vengan vacios.
var longitudDoc = map[string]int{DocDNI: 8, DocRUC: 11}

// TopeBoletaSinDocumento: por encima de este monto SUNAT exige identificar al
// comprador. Por debajo un consumidor final puede no dar documento, que es la
// venta normal de un restaurante y no debe rechazarse.
const TopeBoletaSinDocumento = 700.0

type Receptor struct {
	TipoDoc     string
	NumDoc      string
	RazonSocial string
}

// ValidarReceptor aplica las reglas que SUNAT rechaza, antes de asignar
// correlativo. Un rechazo posterior al contador deja un hueco en la numeracion
// que SUNAT despues observa, asi que todo lo comprobable se comprueba aqui.
func ValidarReceptor(r Receptor, tipoDoc TipoDoc, serie, importeTotal string) error {
	// Entre 3 y 100 caracteres alfanumericos. Menos de 3 es el error 2022
	// "RegistrationName no cumple con el estandar".
	if n := len([]rune(strings.TrimSpace(r.RazonSocial))); n < 3 || n > 100 {
		return ErrReceptorInvalido(fmt.Sprintf("razon_social debe tener entre 3 y 100 caracteres, tiene %d", n))
	}

	if !tipoDocIdentidadValido(r.TipoDoc) {
		return ErrReceptorInvalido("tipo_doc " + r.TipoDoc + " no esta en el catalogo 06")
	}

	if exigeRUC(tipoDoc, serie) && r.TipoDoc != DocRUC {
		return ErrReceptorInvalido("este comprobante exige RUC en el receptor (tipo_doc 6)")
	}

	if r.TipoDoc == DocSinRUC {
		if superaTopeSinDocumento(importeTotal) {
			return ErrReceptorInvalido("una boleta mayor a S/700 exige documento del comprador")
		}
		return nil
	}

	num := strings.TrimSpace(r.NumDoc)
	if num == "" {
		return ErrReceptorInvalido("num_doc no puede estar vacio")
	}

	if largo, fijo := longitudDoc[r.TipoDoc]; fijo {
		if !soloDigitos(num) || len(num) != largo {
			return ErrReceptorInvalido(fmt.Sprintf("num_doc debe tener %d digitos para tipo_doc %s, llego %q", largo, r.TipoDoc, num))
		}
	}

	if r.TipoDoc == DocRUC && !RUCValido(num) {
		return ErrReceptorInvalido("el RUC " + num + " tiene digito verificador invalido")
	}

	return nil
}

// Una factura solo puede emitirse contra un RUC. Una nota hereda la exigencia
// del documento que modifica, que se reconoce por el prefijo de la serie.
func exigeRUC(t TipoDoc, serie string) bool {
	if t == TipoFactura {
		return true
	}
	if t == TipoNotaCredito || t == TipoNotaDebito {
		return len(serie) > 0 && strings.EqualFold(serie[:1], "F")
	}
	return false
}

func tipoDocIdentidadValido(t string) bool {
	switch t {
	case DocSinRUC, DocDNI, DocCarnetExt, DocRUC, DocPasaporte, DocCedulaDip:
		return true
	}
	return false
}

// Un importe ilegible se trata como si superara el tope: es preferible pedir el
// documento de mas que emitir una boleta que SUNAT va a rechazar.
func superaTopeSinDocumento(importe string) bool {
	v, err := strconv.ParseFloat(strings.TrimSpace(importe), 64)
	if err != nil {
		return true
	}
	return v > TopeBoletaSinDocumento
}

// RUCValido verifica el digito de control por modulo 11. Detecta el error real
// de digitacion, que es lo que SUNAT rechaza con codigo 2017.
func RUCValido(ruc string) bool {
	if len(ruc) != 11 || !soloDigitos(ruc) {
		return false
	}

	factores := [10]int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	suma := 0
	for i, f := range factores {
		suma += int(ruc[i]-'0') * f
	}

	esperado := 11 - suma%11
	switch esperado {
	case 10:
		esperado = 0
	case 11:
		esperado = 1
	}

	return esperado == int(ruc[10]-'0')
}
