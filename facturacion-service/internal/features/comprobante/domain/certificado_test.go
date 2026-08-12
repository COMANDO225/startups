package domain

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

type certDePrueba struct {
	pem   string
	clave *rsa.PrivateKey
	cert  []byte
}

func generarCert(t *testing.T, desde, hasta time.Time) certDePrueba {
	t.Helper()

	clave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	plantilla := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "20000000001",
			Organization: []string{"EMPRESA DE PRUEBA SAC"},
		},
		NotBefore: desde,
		NotAfter:  hasta,
	}

	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}

	return certDePrueba{pem: armarPEM(t, clave, der), clave: clave, cert: der}
}

func armarPEM(t *testing.T, clave *rsa.PrivateKey, der []byte) string {
	t.Helper()

	pkcs8, err := x509.MarshalPKCS8PrivateKey(clave)
	if err != nil {
		t.Fatal(err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})) +
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestCertificadoValido(t *testing.T) {
	c := generarCert(t, time.Now().Add(-time.Hour), time.Now().Add(2*365*24*time.Hour))

	datos, err := ValidarCertificado(c.pem)
	if err != nil {
		t.Fatalf("rechazo un certificado valido: %v", err)
	}
	if datos.Titular != "20000000001" {
		t.Fatalf("titular = %q", datos.Titular)
	}
	if datos.PorVencer() {
		t.Fatal("marco como por vencer uno de dos años")
	}
}

// El caso que de verdad importa: si la clave no corresponde al certificado, la
// firma sale bien formada pero invalida y SUNAT rechaza cada comprobante,
// quemando su correlativo. Tiene que detectarse en el alta, no al emitir.
func TestClaveQueNoCorrespondeAlCertificado(t *testing.T) {
	bueno := generarCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour*24*365))
	otro := generarCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour*24*365))

	// La clave de uno con el certificado del otro.
	mezclado := armarPEM(t, otro.clave, bueno.cert)

	if _, err := ValidarCertificado(mezclado); err == nil {
		t.Fatal("acepto una clave privada que no corresponde al certificado")
	}
}

func TestCertificadoVencido(t *testing.T) {
	c := generarCert(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	if _, err := ValidarCertificado(c.pem); err == nil {
		t.Fatal("acepto un certificado vencido: firmaria comprobantes que SUNAT rechaza")
	}
}

func TestCertificadoTodaviaNoVigente(t *testing.T) {
	c := generarCert(t, time.Now().Add(48*time.Hour), time.Now().Add(96*time.Hour))

	if _, err := ValidarCertificado(c.pem); err == nil {
		t.Fatal("acepto un certificado que aun no entra en vigencia")
	}
}

func TestCertificadoPorVencerSeAvisa(t *testing.T) {
	c := generarCert(t, time.Now().Add(-time.Hour), time.Now().Add(10*24*time.Hour))

	datos, err := ValidarCertificado(c.pem)
	if err != nil {
		t.Fatalf("uno vigente por 10 dias sigue siendo valido: %v", err)
	}
	if !datos.PorVencer() {
		t.Fatal("no aviso de un certificado que vence en 10 dias")
	}
}

func TestCertificadoIncompleto(t *testing.T) {
	c := generarCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour*24*365))

	soloCert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert}))
	if _, err := ValidarCertificado(soloCert); err == nil {
		t.Fatal("acepto un PEM sin la clave privada: no podria firmar nada")
	}

	pkcs8, _ := x509.MarshalPKCS8PrivateKey(c.clave)
	soloClave := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	if _, err := ValidarCertificado(soloClave); err == nil {
		t.Fatal("acepto un PEM sin certificado")
	}

	for _, basura := range []string{"", "no es un pem", "-----BEGIN CERTIFICATE-----\nxxx\n-----END CERTIFICATE-----"} {
		if _, err := ValidarCertificado(basura); err == nil {
			t.Fatalf("acepto %q como certificado", basura)
		}
	}
}
