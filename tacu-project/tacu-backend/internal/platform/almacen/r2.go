package almacen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// R2 guarda las imagenes en Cloudflare R2, por su API compatible con S3.
//
// DOS buckets y no uno: en R2 el acceso publico es POR BUCKET —esta atado a un
// dominio o no lo esta—, asi que no hay carpeta publica dentro de un bucket
// privado. Las fotos de plato van al publico porque su destino es un catalogo
// que se reparte por WhatsApp; las hojas de la carta del dueno, su estilo y sus
// referencias van al privado.
//
// OJO: privado significa que NO estan en el dominio publico ni las sirve el CDN.
// Hoy siguen saliendo por /media, que no comprueba nada: quien conozca la clave
// las abre. La clave es un uuidv7 —o sea que hay que adivinar 128 bits— pero eso
// es seguridad por oscuridad, y la carta de papel de un restaurante es un
// documento de su negocio. Falta firmar esas URLs; ver el TODO en core/app.go.
type R2 struct {
	cli      *http.Client
	cred     credenciales
	host     string
	publico  string
	privado  string
	dominio  string // vacio = las publicas tambien se sirven por la API
	base     string // el prefijo de nuestra API, p.ej. "/media"
	log      *slog.Logger
	reintent int
}

// R2 ignora la region pero la firma la exige, y tiene que ser exactamente esta.
const (
	regionR2   = "auto"
	servicioR2 = "s3"
)

// tiempoDePeticion. Generoso porque sube fotos de hasta unos megas desde un
// servidor que puede estar lejos de R2, pero acotado: sin esto una peticion
// colgada retiene un worker para siempre.
const tiempoDePeticion = 30 * time.Second

// maxReintentos ante 5xx y errores de red. Tres en total, no tres extra.
const maxReintentos = 3

func NuevoR2(cuenta, claveID, secreto, publico, privado, dominio, base string, log *slog.Logger) (*R2, error) {
	for _, c := range []struct{ nombre, valor string }{
		{"la cuenta", cuenta}, {"la clave", claveID}, {"el secreto", secreto},
		{"el bucket publico", publico}, {"el bucket privado", privado},
	} {
		if strings.TrimSpace(c.valor) == "" {
			return nil, fmt.Errorf("falta %s de R2", c.nombre)
		}
	}

	return &R2{
		cli:      &http.Client{Timeout: tiempoDePeticion},
		cred:     credenciales{claveID: claveID, secreto: secreto},
		host:     cuenta + ".r2.cloudflarestorage.com",
		publico:  publico,
		privado:  privado,
		dominio:  strings.TrimSuffix(dominio, "/"),
		base:     strings.TrimSuffix(base, "/"),
		log:      log,
		reintent: maxReintentos,
	}, nil
}

// EsPublica decide a que bucket va una clave.
//
// Por el prefijo y no por un parametro: la decision es una propiedad de la
// imagen —una foto de plato ES publica, una hoja de carta NO— y dejarla en la
// llamada seria repetirla en cada sitio de escritura, con una oportunidad de
// equivocarse en cada uno.
func EsPublica(clave string) bool {
	// El prefijo de tenant se salta SOLO si esta. Recortar el primer segmento a
	// ciegas mandaba una clave antigua "fotos/..." al bucket privado, donde el
	// catalogo no la encontraria nunca.
	if resto, conTenant := strings.CutPrefix(clave, "r/"); conTenant {
		if _, tras, hay := strings.Cut(resto, "/"); hay {
			clave = tras
		}
	}
	// Dos prefijos publicos, no uno: la foto del local encabeza el catalogo que
	// se reparte por WhatsApp, igual que las fotos de plato. Lo privado sigue
	// siendo lo del editor —las hojas de la carta, el estilo, las referencias—.
	// Lo publico es lo que sale en el catalogo: las fotos de plato, la foto del
	// local y el logo. El LETRERO no: es la foto de trabajo de la que sale el
	// logo, y solo la ve el dueno en su editor.
	for _, publico := range []string{"fotos/", "portada/", "logo/"} {
		if strings.HasPrefix(clave, publico) {
			return true
		}
	}
	return false
}

func (r *R2) bucketDe(clave string) string {
	if EsPublica(clave) {
		return r.publico
	}
	return r.privado
}

func (r *R2) Guardar(ctx context.Context, clave string, datos []byte) error {
	inicio := time.Now()
	bucket := r.bucketDe(clave)

	cabeceras := http.Header{}
	if t := mime.TypeByExtension(path.Ext(clave)); t != "" {
		cabeceras.Set("Content-Type", t)
	}
	// Cada escritura genera una clave nueva —decision tomada por la cache del
	// navegador— asi que el objeto es inmutable de verdad y el CDN no tiene por
	// que revalidarlo nunca.
	cabeceras.Set("Cache-Control", "public, max-age=31536000, immutable")

	_, err := r.pedir(ctx, http.MethodPut, bucket, clave, datos, cabeceras)
	if err != nil {
		return fmt.Errorf("guardando %q en %s: %w", clave, bucket, err)
	}

	r.log.DebugContext(ctx, "objeto guardado",
		"bucket", bucket, "clave", clave, "bytes", len(datos), "ms", time.Since(inicio).Milliseconds())
	return nil
}

func (r *R2) Leer(ctx context.Context, clave string) ([]byte, string, error) {
	bucket := r.bucketDe(clave)

	cuerpo, err := r.pedir(ctx, http.MethodGet, bucket, clave, nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("leyendo %q de %s: %w", clave, bucket, err)
	}
	return cuerpo, mime.TypeByExtension(path.Ext(clave)), nil
}

// Borrar quita un objeto. Que no exista NO es error, igual que en disco: se
// llama al reemplazar una foto, y reintentar esa operacion no puede romperse
// porque una variante ya se fuera en el intento anterior.
func (r *R2) Borrar(ctx context.Context, clave string) error {
	bucket := r.bucketDe(clave)

	if _, err := r.pedir(ctx, http.MethodDelete, bucket, clave, nil, nil); err != nil {
		if errors.Is(err, ErrNoEsta) {
			return nil
		}
		return fmt.Errorf("borrando %q de %s: %w", clave, bucket, err)
	}
	return nil
}

// URL decide por donde ve el navegador una imagen.
//
// Con dominio propio, las publicas van derechas al CDN de Cloudflare y el egreso
// es gratis. Sin el, salen por nuestra API igual que las privadas: asi el dueno
// ve sus fotos DESDE EL PRIMER DIA, sin activar el subdominio r2.dev —que
// Cloudflare marca como solo para desarrollo y limita por tasa— y sin esperar a
// tener el dominio. El dia que lo ate, es una linea de config: las imagenes se
// guardan por CLAVE, nunca por URL.
func (r *R2) URL(clave string) string {
	if clave == "" {
		return ""
	}
	if r.dominio != "" && EsPublica(clave) {
		return r.dominio + "/" + clave
	}
	return r.base + "/" + clave
}

// Comprobar verifica que los dos buckets responden con estas credenciales.
//
// Al arrancar y no a la primera foto: unas credenciales mal puestas se
// descubririan cuando un dueno pulsa "generar", despues de cobrarle $0.0336 por
// un error de configuracion nuestro.
func (r *R2) Comprobar(ctx context.Context) error {
	for _, bucket := range []string{r.publico, r.privado} {
		if _, err := r.pedir(ctx, http.MethodHead, bucket, "", nil, nil); err != nil {
			return fmt.Errorf("el bucket %q no responde: %w", bucket, err)
		}
	}
	return nil
}

// ErrNoEsta distingue el 404 del resto: Borrar lo trata como exito.
var ErrNoEsta = errors.New("el objeto no esta")

// pedir firma y manda, reintentando lo que merece reintentarse.
func (r *R2) pedir(ctx context.Context, metodo, bucket, clave string, cuerpo []byte, cabeceras http.Header) ([]byte, error) {
	url := "https://" + r.host + "/" + bucket
	if clave != "" {
		url += "/" + clave
	}
	hash := hashDe(cuerpo)

	var ultimo error
	for intento := 1; intento <= r.reintent; intento++ {
		if intento > 1 {
			// Espera creciente. El ctx manda: si quien llama se rindio, no se
			// duerme esperando a reintentar algo que ya nadie quiere.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(intento-1) * 250 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, metodo, url, bytes.NewReader(cuerpo))
		if err != nil {
			return nil, err
		}
		maps.Copy(req.Header, cabeceras)
		req.Header.Set("x-amz-content-sha256", hash)
		firmar(req, r.cred, regionR2, servicioR2, hash, time.Now())

		resp, err := r.cli.Do(req)
		if err != nil {
			ultimo = err
			continue // de red: se reintenta
		}

		leido, errLeer := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusNotFound:
			return nil, ErrNoEsta

		case resp.StatusCode >= 500:
			// Un 500 de R2 suele irse solo; un 4xx no mejora reintentando.
			ultimo = fmt.Errorf("%s respondio %d", metodo, resp.StatusCode)
			continue

		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("%s respondio %d: %s", metodo, resp.StatusCode, recorte(leido))

		case errLeer != nil:
			ultimo = errLeer
			continue
		}
		return leido, nil
	}
	return nil, fmt.Errorf("tras %d intentos: %w", r.reintent, ultimo)
}

// recorte deja el error de S3 legible sin volcar un XML entero en el log.
func recorte(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
