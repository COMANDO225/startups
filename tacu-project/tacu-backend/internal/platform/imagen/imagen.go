// Package imagen normaliza lo que se guarda: una foto entra, salen tres tamanos
// listos para servir.
package imagen

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"path"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

// Variante es uno de los tres tamanos. La grande no lleva sufijo: es la clave
// que ya viaja en la base, asi que anadir tamanos no cambia ni una fila.
type Variante string

const (
	Grande  Variante = ""
	Media   Variante = "_640"
	Pequena Variante = "_320"
)

// Los lados mayores. La grande a 1280 porque es lo que se le vuelve a mandar al
// modelo al corregir una foto, y por encima de eso no aporta: una foto de 12 MP
// del telefono tiene la nitidez real de un FullHD bien tomado.
var lados = map[Variante]int{Grande: 1280, Media: 640, Pequena: 320}

// Variantes en el orden en que se escriben.
var Variantes = []Variante{Grande, Media, Pequena}

// calidad de WebP.
//
// MEDIDO con cmd/imgcheck, y las dos muestras que habia dicen cosas distintas:
//
//	plato generado (1024, casi monocromo)  421K -> 19K las tres   webp = 43% del jpeg
//	foto de carta (720x1280, ya comprimida) 178K -> 277K las tres  webp = 99% del jpeg
//
// La primera sale demasiado bien —un plato blanco sobre fondo blanco es lo mas
// facil que hay— y la segunda demasiado mal: ya venia comprimida y a 1280, asi
// que la grande no reduce y solo suma una generacion de perdida. Un plato de
// verdad, con color y textura, cae entre las dos y mas cerca de la primera.
//
// Lo que SI es solido en las dos: lo que se sirve baja mucho —9x en el peor
// caso, contando la pequena— porque el catalogo pinta 80 px y hoy le mandamos
// 1024. Y que el JPEG que devuelve el generador viene encodeado a lo bruto: el
// mismo tamano a q82 son 27K contra 421K, sin diferencia a la vista.
//
// Falta medir sobre una foto de plato de verdad, que cuesta $0.0336 generar.
const calidad = 80

// maxPixeles frena las bombas de descompresion: un PNG de 50000x50000 son 200 MB
// de RGBA en memoria y unos pocos KB en disco. Se mira ANTES de decodificar.
const maxPixeles = 50 * 1000 * 1000

var (
	ErrDemasiadoGrande = errors.New("la imagen tiene demasiados pixeles")
	ErrNoEsImagen      = errors.New("el archivo no es una imagen que sepamos leer")
)

// Extension es la de todo lo que sale de aqui.
const Extension = ".webp"

// Normalizar decodifica, endereza y devuelve los tres tamanos en WebP.
//
// La original NO sale: se descarta a proposito. 1280 sobra para cualquier
// pantalla y para volver a pasarla por el modelo, y guardar los 4 MB de una foto
// de telefono es almacenamiento que se paga todos los meses sin que nadie los
// mire nunca.
func Normalizar(origen []byte) (map[Variante][]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(origen))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoEsImagen, err)
	}
	if cfg.Width*cfg.Height > maxPixeles {
		return nil, fmt.Errorf("%w: %dx%d", ErrDemasiadoGrande, cfg.Width, cfg.Height)
	}

	src, err := decodificar(origen)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoEsImagen, err)
	}
	src = Enderezar(src, OrientacionDe(origen))

	fuera := make(map[Variante][]byte, len(Variantes))
	for _, v := range Variantes {
		b, err := encodear(reducir(src, lados[v]))
		if err != nil {
			return nil, fmt.Errorf("encodeando la variante %q: %w", v, err)
		}
		fuera[v] = b
	}
	return fuera, nil
}

// decodificar acepta lo mismo que el borde HTTP deja pasar. El webp va aparte
// porque su decodificador no se registra en image.Decode.
func decodificar(b []byte) (image.Image, error) {
	if img, _, err := image.Decode(bytes.NewReader(b)); err == nil {
		return img, nil
	}
	return webp.Decode(bytes.NewReader(b))
}

// reducir escala hasta que el lado mayor quepa en lado, guardando la proporcion.
//
// No recorta a cuadrado aunque los huecos lo sean: object-cover ya recorta al
// pintar, y recortar aqui destruiria lo que el dueno no ha dicho que sobre.
//
// CatmullRom y no un filtro de caja: la caja es correcta reduciendo mucho, pero
// la variante de 640 baja poco desde 1280 y ahi la caja emborrona.
func reducir(src image.Image, lado int) image.Image {
	b := src.Bounds()
	an, al := b.Dx(), b.Dy()
	if an <= lado && al <= lado {
		return src
	}

	if an > al {
		al = al * lado / an
		an = lado
	} else {
		an = an * lado / al
		al = lado
	}
	if an < 1 {
		an = 1
	}
	if al < 1 {
		al = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, an, al))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func encodear(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: calidad}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ConVariante mete el sufijo antes de la extension.
//
// Es la UNICA que sabe como se llama cada tamano. Quien guarda y quien sirve
// salen de aqui, o el dia que cambie el sufijo el catalogo pediria claves que
// nadie escribio.
func ConVariante(clave string, v Variante) string {
	if v == Grande {
		return clave
	}
	ext := path.Ext(clave)
	return strings.TrimSuffix(clave, ext) + string(v) + ext
}
