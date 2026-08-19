// cartacheck lee una carta de restaurante desde una o varias imagenes y muestra
// lo extraido, lo que quedo marcado para revision, y cuanto costo.
//
// Es la herramienta de la puerta de calidad del fin de semana 1: se le pasan 10
// cartas reales y se mira si al menos 8 salen usables.
//
//	export GEMINI_API_KEY=...
//	go run ./cmd/cartacheck carta.jpg [carta2.jpg ...]
//	go run ./cmd/cartacheck -json carta.jpg > carta.json
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
	"strings"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
)

func main() {
	salidaJSON := flag.Bool("json", false, "imprimir la carta como JSON")
	modelo := flag.String("modelo", "", "forzar un modelo (proveedor/modelo); por defecto usa la cadena del YAML")
	rutaConfig := flag.String("config", "config/config.yaml", "ruta del archivo de configuracion")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "uso: cartacheck [-json] carta.jpg [carta2.jpg ...]")
		os.Exit(2)
	}

	if err := correr(*salidaJSON, *modelo, *rutaConfig, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func correr(salidaJSON bool, modelo, rutaConfig string, rutas []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cfg, err := config.Cargar(rutaConfig)
	if err != nil {
		return err
	}

	// La cadena de leer_carta sale del YAML. -modelo la pisa solo para probar
	// uno concreto sin editar el archivo.
	if modelo != "" {
		cfg.IA.Tareas[string(ai.LeerCarta)] = []string{modelo}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Esta herramienta solo lee cartas: no exige las credenciales de las
	// tareas de imagen.
	cliente, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.LeerCarta)
	if err != nil {
		return err
	}

	imagenes, err := cargar(rutas)
	if err != nil {
		return err
	}

	inicio := time.Now()
	res, err := app.NuevoLeer(cliente).Ejecutar(ctx, imagenes)
	if err != nil {
		return err
	}
	transcurrido := time.Since(inicio)

	if salidaJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Carta)
	}

	imprimir(res, transcurrido)
	return nil
}

func cargar(rutas []string) ([]ai.Imagen, error) {
	imagenes := make([]ai.Imagen, 0, len(rutas))

	for _, ruta := range rutas {
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return nil, fmt.Errorf("leyendo %s: %w", ruta, err)
		}

		mime := porExtension(ruta)
		if mime == "" {
			return nil, fmt.Errorf("%s: extension no soportada (usa jpg, png, webp o pdf)", ruta)
		}

		imagenes = append(imagenes, ai.Imagen{Bytes: datos, MIME: mime})
	}
	return imagenes, nil
}

func porExtension(ruta string) string {
	switch strings.ToLower(filepath.Ext(ruta)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	}
	return ""
}

func imprimir(res *app.Resultado, transcurrido time.Duration) {
	total := 0

	for _, cat := range res.Carta.Categorias {
		fmt.Printf("\n\033[1m%s\033[0m\n", strings.ToUpper(cat.Nombre))
		for _, p := range cat.Platos {
			total++
			marca := "  "
			switch {
			case p.NecesitaRevision():
				marca = "\033[31m!!\033[0m"
			case p.NecesitaConfirmacion():
				marca = "\033[2m··\033[0m"
			}
			for k, pr := range p.Precios {
				nombre := recortar(p.Nombre, 42)
				if k > 0 {
					// Las variantes cuelgan del plato, no repiten su nombre: la
					// carta tampoco lo repite.
					nombre = "  ↳ " + recortar(etiquetaOSinNombre(pr, k), 38)
					marca = "  "
				} else if pr.Etiqueta != "" {
					nombre = recortar(p.Nombre+" ("+pr.Etiqueta+")", 42)
				}
				fmt.Printf("%s %-42s %11s   \033[2mimpreso: %s\033[0m\n",
					marca, nombre, pr.Centimos.String(), pr.Texto)
			}
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 78))
	fmt.Printf("platos            %d en %d categorias", total, len(res.Carta.Categorias))
	if variantes := contarVariantes(res.Carta); variantes > 0 {
		fmt.Printf(", %d con varios precios", variantes)
	}
	fmt.Println()
	fmt.Printf("tiempo            %.1fs\n", transcurrido.Seconds())
	fmt.Printf("tokens            %d entrada + %d salida\n", res.Uso.TokensEntrada, res.Uso.TokensSalida)
	fmt.Printf("modelo            %s\n", res.Uso.Modelo)
	fmt.Printf("costo             $%.5f\n", res.Uso.CostoUSD)

	if res.Marcas.Revisar == 0 {
		fmt.Printf("no cuadra         \033[32mnada\033[0m\n")
	} else {
		pct := float64(res.Marcas.Revisar) / float64(max(total, 1)) * 100
		fmt.Printf("no cuadra         \033[31m%d de %d (%.0f%%)\033[0m\n", res.Marcas.Revisar, total, pct)
	}
	if res.Marcas.Confirmar > 0 {
		fmt.Printf("por confirmar     \033[2m%d (precios corregidos a mano, ambas lecturas coinciden)\033[0m\n",
			res.Marcas.Confirmar)
	}
	if res.Marcas.Revisar == 0 {
		return
	}

	fmt.Printf("\n\033[1mNO CUADRA — hay que mirarlo antes de publicar\033[0m\n")
	for _, p := range res.Carta.ParaRevisar() {
		fmt.Printf("  %-42s %s\n", recortar(p.Nombre, 42), p.Revisar.Explicacion())
		for _, pr := range p.Precios {
			if pr.Texto == "" {
				continue
			}
			leido, err := dinero.Parsear(pr.Texto)
			detalle := "ilegible"
			if err == nil {
				detalle = leido.String()
			}
			fmt.Printf("  %-42s   impreso %q -> %s | el modelo dijo %s\n",
				"", pr.Texto, detalle, pr.Centimos.String())
		}
	}
}

// etiquetaOSinNombre describe una variante. Sin etiqueta impresa se dice, no se
// inventa: es justo lo que el dueno tiene que completar.
func etiquetaOSinNombre(pr domain.Precio, k int) string {
	if pr.Etiqueta != "" {
		return pr.Etiqueta
	}
	return fmt.Sprintf("(opcion %d, sin nombre)", k+1)
}

func contarVariantes(c domain.Carta) int {
	n := 0
	for _, p := range c.Platos() {
		if len(p.Precios) > 1 {
			n++
		}
	}
	return n
}

func recortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
