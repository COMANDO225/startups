package imagen

import (
	"encoding/binary"
	"image"
)

// Orientacion es el valor del tag EXIF 0x0112. 1 es "ya esta derecha".
type Orientacion int

const sinGirar Orientacion = 1

// tagOrientacion en el IFD0 del bloque TIFF que va dentro del APP1.
const tagOrientacion = 0x0112

// OrientacionDe lee la orientacion de un JPEG.
//
// Existe porque la stdlib no la mira: image.Decode entrega los pixeles tal como
// estan guardados, y el telefono guarda la foto en horizontal con un tag que
// dice cuanto girarla. El navegador si lo obedece, asi que hoy —que servimos los
// bytes tal cual— las fotos se ven derechas. En cuanto se decodifica para
// redimensionar, el tag se pierde y salen tumbadas.
//
// Devuelve sinGirar ante cualquier cosa rara. Un EXIF corrupto no puede tumbar
// la subida de una foto: como mucho sale girada, que es lo que pasaba antes.
func OrientacionDe(b []byte) Orientacion {
	app1, ok := segmentoAPP1(b)
	if !ok {
		return sinGirar
	}
	return orientacionEnTIFF(app1)
}

// segmentoAPP1 recorre los segmentos del JPEG hasta encontrar el Exif.
func segmentoAPP1(b []byte) ([]byte, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return nil, false // no es un JPEG
	}

	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return nil, false
		}
		marca := b[i+1]

		// SOS: a partir de aqui vienen los datos comprimidos y ya no hay mas
		// cabeceras que leer.
		if marca == 0xDA || marca == 0xD9 {
			return nil, false
		}

		largo := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if largo < 2 || i+2+largo > len(b) {
			return nil, false
		}
		cuerpo := b[i+4 : i+2+largo]

		if marca == 0xE1 && len(cuerpo) > 6 && string(cuerpo[:4]) == "Exif" {
			return cuerpo[6:], true // se salta "Exif\0\0"
		}
		i += 2 + largo
	}
	return nil, false
}

// orientacionEnTIFF busca el tag dentro del IFD0.
func orientacionEnTIFF(t []byte) Orientacion {
	if len(t) < 8 {
		return sinGirar
	}

	// El bloque TIFF trae su propio orden de bytes: "II" intel, "MM" motorola.
	var orden binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		orden = binary.LittleEndian
	case "MM":
		orden = binary.BigEndian
	default:
		return sinGirar
	}
	if orden.Uint16(t[2:4]) != 0x002A {
		return sinGirar
	}

	ifd := int(orden.Uint32(t[4:8]))
	if ifd+2 > len(t) {
		return sinGirar
	}
	n := int(orden.Uint16(t[ifd : ifd+2]))

	for i := range n {
		e := ifd + 2 + i*12
		if e+12 > len(t) {
			return sinGirar
		}
		if orden.Uint16(t[e:e+2]) != tagOrientacion {
			continue
		}
		// El valor es un SHORT y cabe en el propio campo de 4 bytes, asi que se
		// lee ahi mismo en vez de seguir un desplazamiento.
		v := Orientacion(orden.Uint16(t[e+8 : e+10]))
		if v < 1 || v > 8 {
			return sinGirar
		}
		return v
	}
	return sinGirar
}

// Enderezar aplica la orientacion. Los ocho valores del estandar, incluidos los
// cuatro espejados, que salen de fotos hechas con la camara frontal.
func Enderezar(src image.Image, o Orientacion) image.Image {
	if o == sinGirar {
		return src
	}

	b := src.Bounds()
	an, al := b.Dx(), b.Dy()

	// Las que giran un cuarto intercambian ancho y alto.
	if o >= 5 {
		an, al = al, an
	}
	dst := image.NewRGBA(image.Rect(0, 0, an, al))

	for y := range b.Dy() {
		for x := range b.Dx() {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			var nx, ny int
			switch o {
			case 2:
				nx, ny = b.Dx()-1-x, y
			case 3:
				nx, ny = b.Dx()-1-x, b.Dy()-1-y
			case 4:
				nx, ny = x, b.Dy()-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = b.Dy()-1-y, x
			case 7:
				nx, ny = b.Dy()-1-y, b.Dx()-1-x
			case 8:
				nx, ny = y, b.Dx()-1-x
			}
			dst.Set(nx, ny, c)
		}
	}
	return dst
}
