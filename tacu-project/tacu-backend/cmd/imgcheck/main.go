// imgcheck mide lo que pesa una foto antes y despues de normalizarla, y compara
// WebP contra JPEG sobre las MISMAS imagenes ya reducidas.
//
// Existe por la misma razon que cartabench y fotocheck: el ~30% que reclama WebP
// sale de sus autores midiendo fotos genericas, y aqui las fotos son de comida
// —salsas brillantes, mucho detalle fino, fondo liso—. En config.yaml esta el
// caso del modelo que ganaba en los benchmarks y perdio 3-0 sobre la carta real.
//
// A diferencia de cartabench y fotocheck, este NO gasta dinero: no llama a
// ninguna IA, solo decodifica y vuelve a encodear.
//
//	go run ./cmd/imgcheck -salida /tmp/ver datos/media/estilo/*/*.jpg
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"tacu-backend/internal/platform/imagen"
)

// fotosPorVista es lo que carga un telefono al abrir el catalogo, contando lo
// que entra en pantalla mas lo que el lazy loading trae al primer scroll.
const fotosPorVista = 20

type medida struct {
	original int
	webp     int
	jpeg     int
	pequena  int
}

func main() {
	salida := flag.String("salida", "", "carpeta donde dejar las variantes para mirarlas a ojo")
	calidadJPEG := flag.Int("jpeg", 82, "calidad del JPEG con el que se compara")
	calidadWebP := flag.Int("webp", 80, "calidad de WebP")
	metodo := flag.Int("metodo", 6, "esfuerzo de WebP, 0 rapido y 6 lento")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "uso: imgcheck [-salida dir] archivo...")
		os.Exit(2)
	}
	if *salida != "" {
		if err := os.MkdirAll(*salida, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	fmt.Printf("%-30s %10s %10s %10s %9s\n", "archivo", "original", "webp x3", "jpeg x3", "webp/jpeg")

	var total medida
	n := 0
	for _, ruta := range flag.Args() {
		m, err := medir(ruta, *calidadJPEG, *calidadWebP, *metodo, *salida)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", ruta, err)
			continue
		}
		total.original += m.original
		total.webp += m.webp
		total.jpeg += m.jpeg
		total.pequena += m.pequena
		n++

		fmt.Printf("%-30s %9.0fK %9.0fK %9.0fK %8.0f%%\n",
			recortar(filepath.Base(ruta), 30),
			kb(m.original), kb(m.webp), kb(m.jpeg),
			float64(m.webp)*100/float64(m.jpeg))
	}
	if n == 0 {
		os.Exit(1)
	}

	fmt.Printf("\nlas tres variantes juntas pesan el %.0f%% de la original\n",
		float64(total.webp)*100/float64(total.original))

	antes := float64(total.original) / float64(n) * fotosPorVista
	despues := float64(total.pequena) / float64(n) * fotosPorVista
	fmt.Printf("una vista del catalogo (%d fotos): %.1f MB -> %.2f MB  (%.0fx menos)\n",
		fotosPorVista, antes/(1024*1024), despues/(1024*1024), antes/despues)
}

func medir(ruta string, calidadJPEG, calidadWebP, metodo int, salida string) (medida, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return medida{}, err
	}

	// Los MISMOS pixeles para los dos formatos. Encodeando uno sobre la salida
	// del otro se mediria la perdida de la segunda pasada y no el formato: es el
	// error que tuvo la primera version de esto.
	reducidas, err := imagen.Reducidas(b)
	if err != nil {
		return medida{}, err
	}

	m := medida{original: len(b)}
	nombre := sinExtension(filepath.Base(ruta))

	for _, v := range imagen.Variantes {
		enWebP, err := imagen.EncodearWebP(reducidas[v], calidadWebP, metodo)
		if err != nil {
			return medida{}, err
		}
		enJPEG, err := comoJPEG(reducidas[v], calidadJPEG)
		if err != nil {
			return medida{}, err
		}

		m.webp += len(enWebP)
		m.jpeg += len(enJPEG)
		if v == imagen.Pequena {
			m.pequena = len(enWebP)
		}

		if salida != "" {
			base := filepath.Join(salida, nombre+string(v))
			if err := os.WriteFile(base+".webp", enWebP, 0o644); err != nil {
				return medida{}, err
			}
			if err := os.WriteFile(base+".jpg", enJPEG, 0o644); err != nil {
				return medida{}, err
			}
		}
	}
	return m, nil
}

func comoJPEG(img image.Image, calidad int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: calidad}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func kb(n int) float64 { return float64(n) / 1024 }

func recortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func sinExtension(s string) string { return s[:len(s)-len(filepath.Ext(s))] }
