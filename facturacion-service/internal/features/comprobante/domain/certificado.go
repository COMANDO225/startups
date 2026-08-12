package domain

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// VigenciaMinima: por debajo de esto conviene avisar al emisor antes de que se
// le venza en plena operacion. Un certificado vencido rechaza todo lo que firme.
const VigenciaMinima = 30 * 24 * time.Hour

// DatosCertificado es lo que se puede contar del certificado sin exponerlo.
type DatosCertificado struct {
	Titular string
	Emisor  string
	Desde   time.Time
	Hasta   time.Time
}

// PorVencer avisa cuando queda poco: renovarlo tarda dias en la entidad
// certificadora, y sin certificado vigente no se puede facturar.
func (d DatosCertificado) PorVencer() bool {
	return time.Until(d.Hasta) < VigenciaMinima
}

// ValidarCertificado comprueba que el PEM sirva realmente para firmar, al dar de
// alta al emisor y no al emitir. Descubrirlo despues significa un comprobante
// rechazado y un correlativo quemado en cada intento.
func ValidarCertificado(pemStr string) (DatosCertificado, error) {
	var cert *x509.Certificate
	var priv crypto.PrivateKey

	resto := []byte(pemStr)
	for {
		bloque, siguiente := pem.Decode(resto)
		if bloque == nil {
			break
		}
		resto = siguiente

		switch bloque.Type {
		case "CERTIFICATE":
			c, err := x509.ParseCertificate(bloque.Bytes)
			if err != nil {
				return DatosCertificado{}, ErrCertificadoInvalido("el certificado no se pudo leer: " + err.Error())
			}
			// El primero es el del titular; los siguientes son la cadena.
			if cert == nil {
				cert = c
			}
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			k, err := parseClavePrivada(bloque)
			if err != nil {
				return DatosCertificado{}, ErrCertificadoInvalido("la clave privada no se pudo leer: " + err.Error())
			}
			priv = k
		}
	}

	if cert == nil {
		return DatosCertificado{}, ErrCertificadoInvalido("no contiene un bloque CERTIFICATE")
	}
	if priv == nil {
		return DatosCertificado{}, ErrCertificadoInvalido("no contiene la clave privada; el PEM debe traer clave y certificado juntos")
	}

	// Que la clave corresponda al certificado es lo que de verdad evita el
	// desastre: si no coinciden, la firma sale bien formada pero invalida, y
	// SUNAT rechaza cada comprobante quemando su correlativo.
	if err := clavePerteneceAlCertificado(priv, cert); err != nil {
		return DatosCertificado{}, err
	}

	ahora := time.Now()
	if ahora.Before(cert.NotBefore) {
		return DatosCertificado{}, ErrCertificadoInvalido(
			fmt.Sprintf("todavia no es valido: empieza el %s", cert.NotBefore.Format("2006-01-02")))
	}
	if ahora.After(cert.NotAfter) {
		return DatosCertificado{}, ErrCertificadoInvalido(
			fmt.Sprintf("vencio el %s", cert.NotAfter.Format("2006-01-02")))
	}

	return DatosCertificado{
		Titular: cert.Subject.CommonName,
		Emisor:  cert.Issuer.CommonName,
		Desde:   cert.NotBefore,
		Hasta:   cert.NotAfter,
	}, nil
}

func parseClavePrivada(b *pem.Block) (crypto.PrivateKey, error) {
	if k, err := x509.ParsePKCS8PrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	return x509.ParseECPrivateKey(b.Bytes)
}

func clavePerteneceAlCertificado(priv crypto.PrivateKey, cert *x509.Certificate) error {
	firmante, ok := priv.(crypto.Signer)
	if !ok {
		return ErrCertificadoInvalido("el tipo de clave privada no sirve para firmar")
	}

	publica, ok := cert.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return ErrCertificadoInvalido("el certificado usa un tipo de clave no soportado")
	}

	if !publica.Equal(firmante.Public()) {
		return ErrCertificadoInvalido("la clave privada no corresponde al certificado")
	}
	return nil
}
