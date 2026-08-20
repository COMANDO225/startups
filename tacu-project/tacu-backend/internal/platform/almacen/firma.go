package almacen

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// AWS Signature V4, que es lo que habla R2.
//
// A mano y no con aws-sdk-go-v2 porque contra un solo host y tres verbos —PUT,
// GET, DELETE— el SDK arrastra config, credentials, s3 y smithy para envolver
// esto. El algoritmo esta cerrado desde 2012 y no se mueve.
//
// Escrito con cuidado porque es criptografia de cabecera y falla como un 403 sin
// pista: cualquier diferencia de un byte entre lo que se firma y lo que se manda
// —el orden de las cabeceras, una mayuscula, un salto de linea— da el mismo
// error que una clave equivocada.

const algoritmo = "AWS4-HMAC-SHA256"

// credenciales de R2. Nunca se imprimen: el String() de esto no existe a
// proposito, para que no acabe en un log por accidente.
type credenciales struct {
	claveID string
	secreto string
}

// firmar mete en req la cabecera Authorization.
//
// Firma lo que YA esta en la peticion y no anade cabeceras propias salvo la
// fecha: x-amz-content-sha256 lo pone quien llama porque es un requisito de S3,
// no del algoritmo. Esa separacion es lo que permite comprobar esto contra los
// vectores oficiales de AWS, que no la llevan.
//
// region y servicio entran por parametro por lo mismo: son de R2, no de la
// firma.
func firmar(req *http.Request, cred credenciales, region, servicio, hashCuerpo string, ahora time.Time) {
	amzFecha := ahora.UTC().Format("20060102T150405Z")
	dia := ahora.UTC().Format("20060102")

	req.Header.Set("x-amz-date", amzFecha)

	firmadas, canonicas := cabecerasCanonicas(req)

	canonica := strings.Join([]string{
		req.Method,
		rutaCanonica(req.URL.EscapedPath()),
		req.URL.RawQuery,
		canonicas,
		firmadas,
		hashCuerpo,
	}, "\n")

	alcance := strings.Join([]string{dia, region, servicio, "aws4_request"}, "/")
	porFirmar := strings.Join([]string{
		algoritmo,
		amzFecha,
		alcance,
		hex.EncodeToString(sha256Bytes([]byte(canonica))),
	}, "\n")

	clave := []byte("AWS4" + cred.secreto)
	for _, parte := range []string{dia, region, servicio, "aws4_request"} {
		clave = hmacSHA256(clave, parte)
	}
	firma := hex.EncodeToString(hmacSHA256(clave, porFirmar))

	req.Header.Set("Authorization", algoritmo+
		" Credential="+cred.claveID+"/"+alcance+
		",SignedHeaders="+firmadas+
		",Signature="+firma)
}

// cabecerasCanonicas devuelve la lista firmada y el bloque, en el orden que
// exige el algoritmo: nombres en minuscula y ordenados.
//
// Se firman solo host y las x-amz-*: firmar Content-Type obligaria a que el
// proxy de turno no lo normalice, y no hace falta.
func cabecerasCanonicas(req *http.Request) (firmadas, bloque string) {
	nombres := []string{"host"}
	valores := map[string]string{"host": req.URL.Host}

	for nombre, v := range req.Header {
		n := strings.ToLower(nombre)
		if strings.HasPrefix(n, "x-amz-") {
			nombres = append(nombres, n)
			valores[n] = strings.TrimSpace(v[0])
		}
	}
	ordenar(nombres)

	var b strings.Builder
	for _, n := range nombres {
		b.WriteString(n)
		b.WriteByte(':')
		b.WriteString(valores[n])
		b.WriteByte('\n')
	}
	return strings.Join(nombres, ";"), b.String()
}

// rutaCanonica: la ruta ya viene escapada por url.URL, y una vacia es "/".
func rutaCanonica(ruta string) string {
	if ruta == "" {
		return "/"
	}
	return ruta
}

// ordenar es un insercion sobre una lista de tres o cuatro nombres. sort.Strings
// haria lo mismo; esto evita el import por algo que nunca crece.
func ordenar(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

func hmacSHA256(clave []byte, dato string) []byte {
	h := hmac.New(sha256.New, clave)
	h.Write([]byte(dato))
	return h.Sum(nil)
}

func sha256Bytes(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// hashDe es el x-amz-content-sha256 de un cuerpo.
func hashDe(cuerpo []byte) string {
	return hex.EncodeToString(sha256Bytes(cuerpo))
}
