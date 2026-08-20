package imagen

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/gen2brain/webp"
)

func medidas(b []byte) (image.Config, error) {
	return webp.DecodeConfig(bytes.NewReader(b))
}

func TestLasTresVariantesRespetanLaProporcion(t *testing.T) {
	origen := jpegDe(2000, 1000)

	vs, err := Normalizar(origen)
	if err != nil {
		t.Fatalf("Normalizar: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("salieron %d variantes, quiere 3", len(vs))
	}

	quiere := map[Variante][2]int{
		Grande:  {1280, 640},
		Media:   {640, 320},
		Pequena: {320, 160},
	}
	for v, wh := range quiere {
		cfg, err := medidas(vs[v])
		if err != nil {
			t.Fatalf("variante %q no decodifica: %v", v, err)
		}
		if cfg.Width != wh[0] || cfg.Height != wh[1] {
			t.Fatalf("variante %q = %dx%d, quiere %dx%d", v, cfg.Width, cfg.Height, wh[0], wh[1])
		}
	}
}

// Lo pequeno no se agranda: una foto de 200px no gana nada estirandose a 1280 y
// perderia nitidez.
func TestNoAgrandaLoQueYaEsPequeno(t *testing.T) {
	vs, err := Normalizar(jpegDe(200, 200))
	if err != nil {
		t.Fatalf("Normalizar: %v", err)
	}
	cfg, err := medidas(vs[Grande])
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 200 || cfg.Height != 200 {
		t.Fatalf("grande = %dx%d, quiere 200x200", cfg.Width, cfg.Height)
	}
}

// El que rompe en silencio: sin esto, las fotos verticales del telefono salen
// tumbadas y solo se nota mirandolas una a una.
func TestLaOrientacionDelTelefonoSeEndereza(t *testing.T) {
	casos := []struct {
		orientacion  Orientacion
		giraUnCuarto bool
	}{
		{1, false}, {2, false}, {3, false}, {4, false},
		{5, true}, {6, true}, {7, true}, {8, true},
	}

	for _, c := range casos {
		t.Run(string(rune('0'+c.orientacion)), func(t *testing.T) {
			origen := conOrientacion(c.orientacion)

			if leida := OrientacionDe(origen); leida != c.orientacion {
				t.Fatalf("OrientacionDe = %d, quiere %d", leida, c.orientacion)
			}

			vs, err := Normalizar(origen)
			if err != nil {
				t.Fatalf("Normalizar: %v", err)
			}
			cfg, err := medidas(vs[Grande])
			if err != nil {
				t.Fatal(err)
			}

			// Girar un cuarto intercambia ancho y alto: una foto tumbada de
			// 400x200 tiene que salir de 200x400.
			anchoMayor := cfg.Width > cfg.Height
			if c.giraUnCuarto == anchoMayor {
				t.Fatalf("orientacion %d dio %dx%d: %s",
					c.orientacion, cfg.Width, cfg.Height,
					map[bool]string{true: "no giro y tenia que girar", false: "giro y no tenia que girar"}[c.giraUnCuarto])
			}
		})
	}
}

// Un EXIF roto no puede tumbar la subida: como mucho la foto sale girada, que es
// lo que pasaba antes de que esto existiera.
func TestUnExifRotoNoRompeNada(t *testing.T) {
	casos := map[string][]byte{
		"vacio":          {},
		"no es jpeg":     []byte("esto no es una imagen"),
		"jpeg pelado":    jpegDe(10, 10),
		"app1 truncado":  append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x20}, []byte("Exif\x00\x00II")...),
		"orden invalido": conTIFF([]byte("XX\x2a\x00\x08\x00\x00\x00")),
	}
	for nombre, b := range casos {
		t.Run(nombre, func(t *testing.T) {
			if o := OrientacionDe(b); o != sinGirar {
				t.Fatalf("OrientacionDe = %d, quiere %d", o, sinGirar)
			}
		})
	}
}

// Una bomba de descompresion son pocos KB en disco y cientos de MB en memoria.
// Se mira la cabecera ANTES de reservar un solo pixel.
func TestRechazaLaBombaDeDescompresion(t *testing.T) {
	if _, err := Normalizar(pngDeCabecera(50000, 50000)); err == nil {
		t.Fatal("acepto una imagen de 2500 megapixeles")
	}
}

func TestConVarianteMeteElSufijoAntesDeLaExtension(t *testing.T) {
	casos := []struct {
		clave  string
		v      Variante
		quiere string
	}{
		{"r/1/fotos/2/abc.webp", Grande, "r/1/fotos/2/abc.webp"},
		{"r/1/fotos/2/abc.webp", Media, "r/1/fotos/2/abc_640.webp"},
		{"r/1/fotos/2/abc.webp", Pequena, "r/1/fotos/2/abc_320.webp"},
	}
	for _, c := range casos {
		if got := ConVariante(c.clave, c.v); got != c.quiere {
			t.Fatalf("ConVariante(%q, %q) = %q, quiere %q", c.clave, c.v, got, c.quiere)
		}
	}
}

// --- ayudantes ---

func jpegDe(an, al int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, an, al))
	for y := range al {
		for x := range an {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// conOrientacion inserta un APP1 con el tag puesto, que es lo que hace el
// telefono al guardar la foto.
func conOrientacion(o Orientacion) []byte {
	tiff := make([]byte, 0, 26)
	tiff = append(tiff, 'I', 'I', 0x2A, 0x00) // little endian
	tiff = binary.LittleEndian.AppendUint32(tiff, 8)
	tiff = binary.LittleEndian.AppendUint16(tiff, 1) // una entrada
	tiff = binary.LittleEndian.AppendUint16(tiff, tagOrientacion)
	tiff = binary.LittleEndian.AppendUint16(tiff, 3) // SHORT
	tiff = binary.LittleEndian.AppendUint32(tiff, 1)
	tiff = binary.LittleEndian.AppendUint16(tiff, uint16(o))
	tiff = append(tiff, 0, 0)
	tiff = binary.LittleEndian.AppendUint32(tiff, 0) // sin IFD siguiente
	return conTIFF(tiff)
}

func conTIFF(tiff []byte) []byte {
	cuerpo := append([]byte("Exif\x00\x00"), tiff...)

	fuera := []byte{0xFF, 0xD8, 0xFF, 0xE1}
	fuera = binary.BigEndian.AppendUint16(fuera, uint16(len(cuerpo)+2))
	fuera = append(fuera, cuerpo...)
	return append(fuera, jpegDe(400, 200)[2:]...) // sin su FFD8
}

// pngDeCabecera arma un PNG cuyo IHDR miente sobre el tamano. No hay pixeles
// detras: DecodeConfig solo lee la cabecera, que es justo lo que se comprueba.
func pngDeCabecera(an, al uint32) []byte {
	ihdr := []byte("IHDR")
	ihdr = binary.BigEndian.AppendUint32(ihdr, an)
	ihdr = binary.BigEndian.AppendUint32(ihdr, al)
	ihdr = append(ihdr, 8, 6, 0, 0, 0) // 8 bits, RGBA

	fuera := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	fuera = binary.BigEndian.AppendUint32(fuera, 13)
	fuera = append(fuera, ihdr...)
	return binary.BigEndian.AppendUint32(fuera, 0) // CRC de mentira: no se llega a el
}
