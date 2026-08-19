// cartabench corre varias configuraciones de extraccion contra la MISMA carta y
// las puntua contra una verdad de referencia verificada a mano.
//
// Existe para reemplazar "lo verifique a mano y salio bien" por un numero
// repetible. Sin esto, comparar single-shot contra multipaso es opinion.
//
//	go run ./cmd/cartabench testdata/cartas/galponcito.jpeg
//	go run ./cmd/cartabench -modelos gemini/gemini-3.5-flash,gemini/gemini-3.7-flash carta.jpg
//	go run ./cmd/cartabench -repeticiones 3 carta.jpg
//	go run ./cmd/cartabench -densidades alta,maxima carta.jpg
//	go run ./cmd/cartabench testdata/cartas/tribuna-del-sur/   (carta de varias fotos)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
)

func main() {
	modelos := flag.String("modelos", "gemini/gemini-3.5-flash,gemini/gemini-3.7-flash,gemini/gemini-3.5-flash-lite",
		"modelos a comparar, separados por coma")
	repeticiones := flag.Int("repeticiones", 1, "corridas por modelo (para ver la varianza)")
	densidades := flag.String("densidades", "", "densidades de imagen a comparar: baja,media,alta,maxima (vacio = la del proveedor)")
	rutaConfig := flag.String("config", "config/config.yaml", "ruta de la configuracion")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "uso: cartabench [-modelos a,b] [-repeticiones N] carta.jpg|carpeta/")
		fmt.Fprintln(os.Stderr, "una carta de varias fotos va en una carpeta con su esperado.json dentro")
		os.Exit(2)
	}

	if err := correr(*rutaConfig, strings.Split(*modelos, ","), niveles(*densidades), *repeticiones, flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// niveles traduce el flag a densidades. Vacio significa "no tocar nada", que es
// distinto de "alta": deja que el proveedor decida su default.
func niveles(s string) []ai.Densidad {
	if strings.TrimSpace(s) == "" {
		return []ai.Densidad{ai.DensidadPorDefecto}
	}
	var out []ai.Densidad
	for _, d := range strings.Split(s, ",") {
		out = append(out, ai.Densidad(strings.TrimSpace(d)))
	}
	return out
}

type corrida struct {
	modelo   string
	puntaje  domain.Puntaje
	uso      ai.Uso
	duracion time.Duration
	err      error
}

func correr(rutaConfig string, modelos []string, densidades []ai.Densidad, repeticiones int, rutaCarta string) error {
	verdad, err := cargarVerdad(rutaCarta)
	if err != nil {
		return err
	}

	imagenes, err := cargarImagenes(rutaCarta)
	if err != nil {
		return err
	}

	fmt.Printf("carta     %s", filepath.Base(rutaCarta))
	if len(imagenes) > 1 {
		fmt.Printf(" (%d fotos)", len(imagenes))
	}
	fmt.Println()
	fmt.Printf("verdad    %d platos, %d con precio corregido a mano\n\n",
		len(verdad.Platos), contarManuscritos(verdad))

	var todas []corrida

	for _, modelo := range modelos {
		for _, d := range densidades {
			imgs := make([]ai.Imagen, len(imagenes))
			copy(imgs, imagenes)
			for i := range imgs {
				imgs[i].Densidad = d
			}

			nombre := strings.TrimSpace(modelo)
			if d != ai.DensidadPorDefecto {
				nombre += " @" + string(d)
			}

			for i := range repeticiones {
				c := unaCorrida(rutaConfig, strings.TrimSpace(modelo), imgs, verdad)
				c.modelo = nombre
				todas = append(todas, c)

				etiqueta := nombre
				if repeticiones > 1 {
					etiqueta = fmt.Sprintf("%s #%d", nombre, i+1)
				}
				imprimirCorrida(etiqueta, c)
			}
		}
	}

	imprimirTabla(todas, verdad)
	return nil
}

func unaCorrida(rutaConfig, modelo string, imagenes []ai.Imagen, verdad domain.Verdad) corrida {
	c := corrida{modelo: modelo}

	cfg, err := config.Cargar(rutaConfig)
	if err != nil {
		c.err = err
		return c
	}
	cfg.IA.Tareas[string(ai.LeerCarta)] = []string{modelo}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cliente, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.LeerCarta)
	if err != nil {
		c.err = err
		return c
	}

	inicio := time.Now()
	res, err := app.NuevoLeer(cliente).Ejecutar(ctx, imagenes)
	c.duracion = time.Since(inicio)
	if err != nil {
		c.err = err
		return c
	}

	c.uso = res.Uso
	c.puntaje = domain.Puntuar(res.Carta, verdad)
	return c
}

func imprimirCorrida(etiqueta string, c corrida) {
	if c.err != nil {
		fmt.Printf("\033[1m%s\033[0m\n  ERROR: %v\n\n", etiqueta, c.err)
		return
	}

	p := c.puntaje
	fmt.Printf("\033[1m%s\033[0m\n", etiqueta)
	fmt.Printf("  exactitud   %.1f%%  (%d de %d con su precio correcto)\n",
		p.Exactitud()*100, p.Aciertos, p.Esperados)
	fmt.Printf("  tiempo      %.1fs   costo $%.5f\n", c.duracion.Seconds(), c.uso.CostoUSD)

	if n := len(p.PrecioErroneo); n > 0 {
		fmt.Printf("  \033[31mprecios mal  %d\033[0m\n", n)
		for _, d := range p.PrecioErroneo {
			fmt.Printf("      %-30s esperaba %s, dijo %s\n", d.Nombre, d.Esperado, d.Obtenido)
		}
	}
	if n := len(p.Faltantes); n > 0 {
		fmt.Printf("  faltantes   %d\n", n)
		for _, f := range p.Faltantes {
			fmt.Printf("      %-30s %s\n", f.Nombre, f.Centimos)
		}
	}
	if n := len(p.Sobrantes); n > 0 {
		fmt.Printf("  inventados  %d\n", n)
		for _, s := range p.Sobrantes {
			fmt.Printf("      %-30s %s\n", s.Nombre, s.Centimos)
		}
	}
	if p.ManuscritosEsperados > 0 {
		fmt.Printf("  manuscritos %d de %d detectados", p.ManuscritosAcertados, p.ManuscritosEsperados)
		if p.ManuscritosFalsos > 0 {
			fmt.Printf(", %d falsos positivos", p.ManuscritosFalsos)
		}
		fmt.Println()
	}
	if n := len(p.Duplicados); n > 0 {
		fmt.Printf("  \033[31mduplicados   %d: %s\033[0m\n", n, strings.Join(p.Duplicados, ", "))
		fmt.Println("      (mismo plato dos veces en la misma seccion: deberia ser uno con dos precios)")
	}
	if n := len(p.CombosSinContenido); n > 0 {
		fmt.Printf("  \033[33mcombos sin contenido  %d: %s\033[0m\n", n, strings.Join(p.CombosSinContenido, ", "))
	}
	fmt.Println()
}

func imprimirTabla(todas []corrida, verdad domain.Verdad) {
	fmt.Println(strings.Repeat("─", 92))
	fmt.Printf("%-32s %9s %8s %9s %7s %7s %9s %9s %9s\n",
		"modelo", "exactitud", "precios", "faltan", "inven", "manus", "dupl", "tok.sal", "costo")
	fmt.Println(strings.Repeat("─", 92))

	for _, c := range todas {
		if c.err != nil {
			fmt.Printf("%-32s %9s\n", c.modelo, "ERROR")
			continue
		}
		p := c.puntaje
		manus := fmt.Sprintf("%d/%d", p.ManuscritosAcertados, p.ManuscritosEsperados)
		fmt.Printf("%-32s %8.1f%% %8d %9d %7d %7s %9d %9d %9s\n",
			c.modelo, p.Exactitud()*100, len(p.PrecioErroneo),
			len(p.Faltantes), len(p.Sobrantes), manus, len(p.Duplicados), c.uso.TokensSalida,
			fmt.Sprintf("$%.5f", c.uso.CostoUSD))
	}
	fmt.Println(strings.Repeat("─", 92))
	fmt.Println("\"precios\" son los que salieron con el nombre bien y el monto mal:")
	fmt.Println("es el fallo que se publica sin que nadie lo note.")
}

func cargarVerdad(rutaCarta string) (domain.Verdad, error) {
	ruta := filepath.Join(rutaCarta, "esperado.json")
	if info, err := os.Stat(rutaCarta); err != nil || !info.IsDir() {
		ext := filepath.Ext(rutaCarta)
		ruta = strings.TrimSuffix(rutaCarta, ext) + ".esperado.json"
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		return domain.Verdad{}, fmt.Errorf("no hay verdad de referencia en %s: %w", ruta, err)
	}

	var v domain.Verdad
	if err := json.Unmarshal(datos, &v); err != nil {
		return domain.Verdad{}, fmt.Errorf("%s: %w", ruta, err)
	}
	if len(v.Platos) == 0 {
		return domain.Verdad{}, fmt.Errorf("%s no tiene platos", ruta)
	}
	return v, nil
}

// cargarImagenes acepta una foto suelta o una carpeta con la carta completa.
//
// Una carta de dos paginas son dos fotos de la MISMA carta, no dos cartas: van
// juntas en la misma peticion o el modelo no puede fusionar las secciones.
// Se ordenan por nombre para que 1.jpeg vaya antes que 2.jpeg y la corrida sea
// reproducible.
func cargarImagenes(ruta string) ([]ai.Imagen, error) {
	info, err := os.Stat(ruta)
	if err != nil {
		return nil, err
	}

	rutas := []string{ruta}
	if info.IsDir() {
		entradas, err := os.ReadDir(ruta)
		if err != nil {
			return nil, err
		}
		rutas = nil
		for _, e := range entradas {
			if !e.IsDir() && mimeDe(e.Name()) != "" {
				rutas = append(rutas, filepath.Join(ruta, e.Name()))
			}
		}
		slices.Sort(rutas)
		if len(rutas) == 0 {
			return nil, fmt.Errorf("%s no tiene ninguna imagen", ruta)
		}
	}

	imagenes := make([]ai.Imagen, 0, len(rutas))
	for _, r := range rutas {
		mime := mimeDe(r)
		if mime == "" {
			return nil, fmt.Errorf("%s: extension no soportada", r)
		}
		datos, err := os.ReadFile(r)
		if err != nil {
			return nil, err
		}
		imagenes = append(imagenes, ai.Imagen{Bytes: datos, MIME: mime})
	}
	return imagenes, nil
}

func mimeDe(ruta string) string {
	return map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".webp": "image/webp", ".pdf": "application/pdf",
	}[strings.ToLower(filepath.Ext(ruta))]
}

func contarManuscritos(v domain.Verdad) int {
	n := 0
	for _, p := range v.Platos {
		if p.Manuscrito {
			n++
		}
	}
	return n
}
