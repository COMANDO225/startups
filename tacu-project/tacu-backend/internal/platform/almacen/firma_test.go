package almacen

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Las credenciales del banco de pruebas oficial de AWS (aws4_testsuite). No son
// de nadie: existen para que cualquier implementacion pueda comprobarse contra
// las mismas firmas.
const (
	claveDePrueba   = "AKIDEXAMPLE"
	secretoDePrueba = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
)

var momentoDePrueba = time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)

// La firma falla como un 403 sin pista: una mayuscula de mas, un salto de linea
// donde no toca o una cabecera fuera de orden dan exactamente el mismo error que
// una clave equivocada. Por eso se comprueba contra los vectores publicados y no
// contra "me funciono una vez", que es lo unico que demuestra la prueba manual
// que hicimos contra el bucket.
func TestLaFirmaCoincideConElVectorDeAWS(t *testing.T) {
	casos := []struct {
		nombre    string
		metodo    string
		url       string
		cabeceras map[string]string
		firma     string
	}{
		{
			// aws4_testsuite: get-vanilla
			nombre: "get pelado",
			metodo: http.MethodGet,
			url:    "https://example.amazonaws.com/",
			firma:  "5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
		},
		{
			// aws4_testsuite: get-vanilla-query-order-key-case
			nombre: "get con parametros",
			metodo: http.MethodGet,
			url:    "https://example.amazonaws.com/?Param1=value1&Param2=value2",
			firma:  "b97d918cfa904a5beff61c982a1b6f458b799221646efd99d3219ec94cdf2500",
		},
		{
			// Una cabecera que NO es x-amz no entra en la firma, asi que tiene
			// que dar exactamente la misma que el get pelado.
			nombre:    "una cabecera que no se firma no la cambia",
			metodo:    http.MethodGet,
			url:       "https://example.amazonaws.com/",
			cabeceras: map[string]string{"My-Header1": "value1"},
			firma:     "5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			req, err := http.NewRequest(c.metodo, c.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			for n, v := range c.cabeceras {
				req.Header.Set(n, v)
			}

			// El hash del cuerpo vacio, que es lo que usa el banco de pruebas.
			firmar(req, credenciales{claveDePrueba, secretoDePrueba},
				"us-east-1", "service", hashDe(nil), momentoDePrueba)

			auth := req.Header.Get("Authorization")
			if !strings.Contains(auth, "Signature="+c.firma) {
				t.Fatalf("firma distinta a la del vector oficial.\nobtenida: %s\nesperada: Signature=%s", auth, c.firma)
			}
			if !strings.Contains(auth, "Credential="+claveDePrueba+"/20150830/us-east-1/service/aws4_request") {
				t.Fatalf("el alcance no es el del vector: %s", auth)
			}
		})
	}
}

// Las cabeceras van en minuscula y ordenadas, y solo se firman host y las
// x-amz-*. Si esto se desordena la firma sigue calculandose sin error y R2
// responde 403 sin decir por que.
func TestSeFirmanHostYLasXAmzEnOrden(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPut, "https://cuenta.r2.cloudflarestorage.com/bucket/clave.webp", nil)
	req.Header.Set("Content-Type", "image/webp")
	req.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	req.Header.Set("x-amz-content-sha256", hashDe(nil))

	firmadas, bloque := cabecerasCanonicas(req)

	if firmadas != "host;x-amz-content-sha256" {
		t.Fatalf("SignedHeaders = %q, quiere %q", firmadas, "host;x-amz-content-sha256")
	}
	if strings.Contains(bloque, "content-type") || strings.Contains(bloque, "cache-control") {
		t.Fatalf("se colo una cabecera que no se firma:\n%s", bloque)
	}
	if !strings.HasPrefix(bloque, "host:cuenta.r2.cloudflarestorage.com\n") {
		t.Fatalf("el host no abre el bloque canonico:\n%s", bloque)
	}
}

// El bucket sale del PREFIJO de la clave, no de un parametro: una foto de plato
// es publica y una hoja de carta no, y eso es una propiedad de la imagen. Si
// esto se equivoca, las cartas de papel de los restaurantes acaban en el bucket
// que sirve al publico.
func TestSoloLasFotosDePlatoSonPublicas(t *testing.T) {
	casos := map[string]bool{
		"r/rest-1/fotos/plato-1/abc.webp":       true,
		"r/rest-1/cartas/imp-1/1.jpg":           false,
		"r/rest-1/estilo/imp-1/abc.webp":        false,
		"r/rest-1/referencias/plato-1/abc.webp": false,

		// Sin el prefijo de tenant tambien tiene que acertar: es lo que hay
		// guardado de antes.
		"fotos/plato-1/abc.webp": true,
		"cartas/imp-1/1.jpg":     false,

		// Y nada que se le parezca de lejos cuela como publico.
		"r/rest-1/fotoss/x.webp":    false,
		"r/rest-1/mis-fotos/x.webp": false,
		"":                          false,
	}
	for clave, quiere := range casos {
		if got := EsPublica(clave); got != quiere {
			t.Fatalf("EsPublica(%q) = %v, quiere %v", clave, got, quiere)
		}
	}
}
