// Command logocheck redibuja el logo de un letrero y mide lo que cuesta.
//
// GASTA DINERO en cada corrida: llama al generador de imagenes de verdad. No se
// lanza "para ver". Existe para responder a UNA pregunta que no se puede
// contestar leyendo documentacion —si la calidad barata sirve para un logo, que
// lleva TEXTO— y para volver a contestarla cuando cambie el modelo o el prompt.
//
//	go run ./cmd/logocheck -foto /ruta/al/letrero.jpg -calidades low,medium
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
)

func main() {
	foto := flag.String("foto", "", "foto del letrero o la fachada")
	calidades := flag.String("calidades", "low,medium", "openai: low|medium|high, separadas por coma")
	tamano := flag.String("tamano", "1024x1024", "openai: 1024x1024 | 1024x1536 | 1536x1024")
	repeticiones := flag.Int("repeticiones", 1, "generaciones por calidad, para ver la varianza")
	salida := flag.String("salida", "experimentos/logos", "carpeta donde dejar las imagenes")
	rutaConfig := flag.String("config", "config/config.yaml", "ruta de la configuracion")
	flag.Parse()

	if *foto == "" {
		fmt.Fprintln(os.Stderr, "falta -foto: sin letrero no hay nada que redibujar")
		os.Exit(1)
	}

	bytes, err := os.ReadFile(*foto)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mime := http.DetectContentType(bytes)
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}

	cfg, err := config.Cargar(*rutaConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelar()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cliente, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.GenerarLogo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*salida, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	total := 0.0
	fmt.Printf("letrero: %s\ntamano : %s\n\n", *foto, *tamano)
	fmt.Printf("%-8s %-4s %10s %10s %s\n", "calidad", "n", "segundos", "$", "archivo")

	for _, calidad := range strings.Split(*calidades, ",") {
		calidad = strings.TrimSpace(calidad)
		for n := 1; n <= *repeticiones; n++ {
			inicio := time.Now()
			resp, err := cliente.Ejecutar(ctx, ai.Peticion{
				Tarea:       ai.GenerarLogo,
				Prompt:      promptDeLogo,
				Referencias: []ai.Imagen{{Bytes: bytes, MIME: mime}},
				Tamano:      *tamano,
				Calidad:     calidad,
			})
			if err != nil {
				fmt.Printf("%-8s %-4d %10s %10s %v\n", calidad, n, "-", "-", err)
				continue
			}

			if len(resp.Imagenes) == 0 {
				fmt.Printf("%-8s %-4d %10s %10s %s\n", calidad, n, "-", "-", "no devolvio imagen")
				continue
			}
			nombre := filepath.Join(*salida, fmt.Sprintf("%s-%d.png", calidad, n))
			if err := os.WriteFile(nombre, resp.Imagenes[0].Bytes, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			total += resp.Uso.CostoUSD
			fmt.Printf("%-8s %-4d %10.1f %10.4f %s\n",
				calidad, n, time.Since(inicio).Seconds(), resp.Uso.CostoUSD, nombre)
		}
	}

	fmt.Printf("\ntotal de esta corrida: $%.4f\n", total)
	fmt.Println("\nOJO: el costo sale de la tabla de config.yaml, que hoy es PLANA por modelo.")
	fmt.Println("Si estas comparando calidades, ese numero es el mismo para todas y MIENTE.")
	fmt.Println("Los precios reales: low 1024 = $0.006 · medium 1024 = $0.053 · high 1536 = $0.165")
}

// El prompt del logo. REDIBUJAR, no inventar: lo que hay en el letrero es la
// marca de otro, y un logo "parecido" es una marca adulterada.
const promptDeLogo = `Redraw the logo that appears in this photograph as a clean, flat digital logo file.

This is a photograph of a real restaurant's sign, taken with a phone: it has glare, reflections, plastic wrap, uneven lighting and it is slightly skewed. Your job is to RECOVER the logo that is underneath all of that, not to design a new one.

KEEP EXACTLY, this is the whole point: the same words spelled the same way, the same letterforms, the same colours, the same mascot or emblem and the same arrangement of the parts. Someone who knows this restaurant has to recognise it instantly as their own logo.

FIX ONLY the damage of the photograph: remove the glare, the reflections and the wrinkles of the plastic, straighten it, sharpen the edges of the letters and the shapes, and rebuild the colours as flat solid areas.

The result is the logo ALONE, centred, on a plain white background, with nothing else around it: no sign, no wall, no menu, no plate, no food, no frame and no photograph of the place.

Do not add any word that is not already in the sign. Do not translate anything. Do not invent a tagline.`
