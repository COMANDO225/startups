package almacen

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Firmante autoriza las imagenes privadas por su URL.
//
// Un <img src> del navegador NO manda cabeceras, asi que el token de la
// importacion no puede autorizar lo que se pinta: el permiso tiene que viajar en
// la propia URL. Es lo que hacen S3 y CloudFront, y por el mismo motivo.
//
// EsPublica decide en los DOS lados, al firmar y al comprobar. Si cada lado
// tuviera su regla, en cuanto discreparan unas imagenes serian inalcanzables y
// otras se quedarian abiertas.
type Firmante struct {
	secreto []byte
}

// ventanaFirma redondea la caducidad a un limite fijo, asi que una URL vive
// entre una y dos horas.
//
// El redondeo no es adorno: sin el, cada respuesta de la API traeria una URL
// distinta para la misma imagen —caduca en ahora+X y ahora se mueve—, cada una
// con su entrada de cache, y el navegador volveria a bajarse las hojas de la
// carta enteras en cada carga del editor. Redondeando, todas las URLs de una
// misma ventana salen identicas byte a byte.
//
// UNA HORA Y NO DOCE porque esto es lo unico que acota el dano de una URL que se
// escape, y el editor ya no depende de que aguante: refresca sus URLs mientras
// la pestana esta a la vista y al volver a ella. Sin ese refresco, bajar de doce
// horas habria sido cambiar un riesgo por imagenes rotas.
//
// Si se toca, mirar tambien REFRESCO_DE_URLS y staleTime en el frontend: tienen
// que quedar holgadamente por debajo.
const ventanaFirma = time.Hour

// minSecreto. Un secreto corto se adivina offline: quien tenga una URL firmada
// tiene el mensaje y su HMAC, y puede probar a su ritmo sin tocar el servidor.
const minSecreto = 32

var (
	ErrSinFirma  = errors.New("esta imagen necesita una URL firmada")
	ErrFirmaMala = errors.New("la firma no vale para esta imagen")
	ErrCaducada  = errors.New("el enlace de la imagen caduco")
)

func NuevoFirmante(secreto string) (*Firmante, error) {
	if len(secreto) < minSecreto {
		return nil, fmt.Errorf("el secreto de firma necesita al menos %d caracteres y tiene %d",
			minSecreto, len(secreto))
	}
	return &Firmante{secreto: []byte(secreto)}, nil
}

// Firmar anade caducidad y firma a la URL de una clave privada. Las publicas
// salen tal cual: el catalogo se abre desde un enlace de WhatsApp, sin sesion.
func (f *Firmante) Firmar(url, clave string, ahora time.Time) string {
	if url == "" || EsPublica(clave) {
		return url
	}
	exp := ahora.Truncate(ventanaFirma).Add(2 * ventanaFirma).Unix()
	return fmt.Sprintf("%s?exp=%d&f=%s", url, exp, f.calcular(clave, exp))
}

// Envolver decora la funcion que traduce clave a URL, que es por donde salen
// todas las que ve el navegador.
func (f *Firmante) Envolver(url func(string) string) func(string) string {
	return func(clave string) string {
		return f.Firmar(url(clave), clave, time.Now())
	}
}

func (f *Firmante) Comprobar(clave, exp, firma string, ahora time.Time) error {
	if EsPublica(clave) {
		return nil
	}
	if exp == "" || firma == "" {
		return ErrSinFirma
	}
	segundos, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return ErrFirmaMala
	}

	// La firma ANTES que la caducidad: hasta comprobarla, exp es un numero que
	// escribio quien pide, y contestarle "caduco" seria opinar sobre un dato
	// suyo. hmac.Equal y no ==, que corta en el primer byte distinto y deja
	// medir por tiempo cuanto prefijo se acerto.
	if !hmac.Equal([]byte(firma), []byte(f.calcular(clave, segundos))) {
		return ErrFirmaMala
	}
	if ahora.Unix() > segundos {
		return ErrCaducada
	}
	return nil
}

// calcular firma la clave Y la caducidad juntas. Firmando solo una de las dos,
// mover la otra dejaria la firma buena: con la clave sola se estira la
// caducidad, con la caducidad sola se cambia que imagen se pide.
func (f *Firmante) calcular(clave string, exp int64) string {
	m := hmac.New(sha256.New, f.secreto)
	_, _ = m.Write([]byte(clave + "\n" + strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
