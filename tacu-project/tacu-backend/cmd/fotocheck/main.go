// fotocheck genera la foto del MISMO plato con varios modelos y guarda los
// resultados lado a lado para poder mirarlos.
//
// Existe por lo mismo que cartabench: comparar generadores de imagen leyendo
// benchmarks es opinion. Aca lo que decide es si el ceviche parece un ceviche
// peruano, y eso hay que verlo.
//
//	go run ./cmd/fotocheck "Ceviche a la Tribuna"          <- usa la cadena del YAML
//	go run ./cmd/fotocheck -modelos a/b,c/d "1/4 pollo"     <- compara modelos concretos
//	go run ./cmd/fotocheck -salida /tmp/fotos -repeticiones 2 "Lomo saltado"
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
	"tacu-backend/internal/platform/db"
)

func main() {
	// Vacio = la cadena que dice config.yaml, que es la que corre en produccion.
	// Tenia una lista por defecto y eso hacia que este CLI NUNCA ejercitara la
	// config: el YAML se quedo apuntando a un modelo caro durante dias sin que
	// ninguna corrida lo delatara.
	modelos := flag.String("modelos", "", "modelos a comparar separados por coma; vacio = la cadena del YAML")
	repeticiones := flag.Int("repeticiones", 1, "generaciones por modelo (para ver la varianza)")
	salida := flag.String("salida", "", "carpeta donde guardar las imagenes (por defecto una temporal)")
	tamano := flag.String("tamano", "", "openai: 1024x1024 · gemini: 1K")
	calidad := flag.String("calidad", "", "openai: low|medium|high")
	proporcion := flag.String("proporcion", "", "gemini: 1:1|4:3|16:9... (openai lo ignora, va en -tamano)")
	rutaConfig := flag.String("config", "config/config.yaml", "ruta de la configuracion")

	// El tipo y la categoria son lo que decide el FORMATO del plato: sin ellos,
	// "Ronda Marina" se genera como un plato individual cualquiera y probar el
	// conocimiento de cevicheria desde aqui seria imposible.
	tipo := flag.String("tipo", "cevicheria", "tipo de restaurante: cevicheria|polleria|chifa|pizzeria|parrilla|criollo|sanguicheria|generico")
	categoria := flag.String("categoria", "", "categoria del plato; 'Tríos' es lo que convierte un combinado en bandeja de tres compartimentos")
	fondo := flag.String("fondo", "", "el fondo de la base del dueno; vacio = el blanco de catalogo")
	tipico := flag.String("tipico", "", "clave del banco de platos: lo que el sistema SABE de ese plato. Necesita TACU_BD_DSN")
	ajuste := flag.String("ajuste", "", "lo que el dueno escribio para corregir la foto")

	// La vista previa del estilo: el recipiente vacio, sin comida. Es otra
	// plantilla, asi que sin poder generarla desde aqui no habria como mirarla
	// antes de ponerla delante del dueno.
	vajilla := flag.Bool("vajilla", false, "dibuja el RECIPIENTE VACIO de la base en vez de un plato de comida")
	recipiente := flag.String("recipiente", "", "el plato de la base del dueno; con -vajilla es lo que se dibuja")

	// Imprime el prompt y NO llama a la IA. Es lo que contesta "por que salio
	// asi": el prompt es la unica entrada, y verlo cuesta cero.
	soloPrompt := flag.Bool("prompt", false, "imprime el prompt y no genera nada")

	// EDITAR en vez de generar de cero: se le manda la foto actual y solo la
	// instruccion de que cambiar. Es lo que contesta si el modelo sabe corregir
	// una foto o solo sabe hacer una nueva.
	editar := flag.String("editar", "", "ruta de una imagen a corregir; el prompt pasa a ser solo -ajuste")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, `uso: fotocheck [-modelos a,b] [-repeticiones N] "Nombre del plato"`)
		os.Exit(2)
	}

	var lista []string
	if strings.TrimSpace(*modelos) != "" {
		lista = strings.Split(*modelos, ",")
	}

	if *soloPrompt {
		fmt.Println(app.PromptFoto(
			domain.Plato{
				Nombre:     strings.Join(flag.Args(), " "),
				Categoria:  *categoria,
				FotoAjuste: *ajuste,
			},
			[]domain.Tipo{domain.Tipo(*tipo)},
			domain.Receta{Fondo: *fondo},
		))
		return
	}

	if err := correr(opciones{
		rutaConfig:   *rutaConfig,
		modelos:      lista,
		repeticiones: *repeticiones,
		salida:       *salida,
		tamano:       *tamano,
		calidad:      *calidad,
		proporcion:   *proporcion,
		tipo:         *tipo,
		categoria:    *categoria,
		fondo:        *fondo,
		vajilla:      *vajilla,
		recipiente:   *recipiente,
		tipico:       *tipico,
		ajuste:       *ajuste,
		editar:       *editar,
		plato:        strings.Join(flag.Args(), " "),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// El prompt NO se escribe aca: sale de app.PromptFoto, que es el mismo que usa
// el producto. Un CLI de comparacion con su propio prompt mide un prompt que
// nadie va a usar.
type corrida struct {
	modelo   string
	archivo  string
	bytes    int
	uso      ai.Uso
	duracion time.Duration
	err      error
}

// opciones son las banderas ya parseadas. Es un struct y no once parametros
// posicionales porque a la novena cadena suelta nadie acierta el orden.
// recetaDelBanco trae lo que el banco sabe de un plato. Es lo que convierte este
// CLI en la forma de VALIDAR el banco: sin esto genera con el prompt de antes de
// que existiera.
func tipicoDelBanco(ctx context.Context, clave string) (domain.PlatoTipico, error) {
	dsn := os.Getenv("TACU_BD_DSN")
	if dsn == "" {
		return domain.PlatoTipico{}, fmt.Errorf("-tipico necesita TACU_BD_DSN")
	}

	pool, err := db.Abrir(ctx, dsn, 1)
	if err != nil {
		return domain.PlatoTipico{}, err
	}
	defer pool.Close()

	banco, err := postgres.NuevoRepo(pool).BancoDePlatos(ctx)
	if err != nil {
		return domain.PlatoTipico{}, err
	}
	for _, t := range banco {
		if t.Clave == clave {
			return t, nil
		}
	}
	return domain.PlatoTipico{}, fmt.Errorf("no hay ningun plato %q en el banco", clave)
}

type opciones struct {
	rutaConfig   string
	modelos      []string
	repeticiones int
	salida       string
	tamano       string
	calidad      string
	proporcion   string

	// tipo, categoria y fondo son las capas de la receta: deciden el formato y
	// el ambiente. Sin ellas este CLI mediria un prompt que el producto no usa.
	tipo       string
	categoria  string
	fondo      string
	recipiente string
	ajuste     string

	// vajilla cambia de plantilla: el recipiente vacio en vez del plato servido.
	vajilla bool

	// tipico es la clave del banco. Se resuelve contra la base por el MISMO
	// camino que produccion: si esto y ReclamarFoto divergieran, validar el
	// banco aqui no probaria nada de lo que sale de verdad.
	tipico string
	editar string

	plato string
}

func correr(o opciones) error {
	if o.salida == "" {
		var err error
		o.salida, err = os.MkdirTemp("", "fotocheck-*")
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(o.salida, 0o755); err != nil {
		return err
	}

	fmt.Printf("plato     %s\n", o.plato)
	fmt.Printf("tipo      %s", o.tipo)
	if o.categoria != "" {
		fmt.Printf("  ·  categoria %s", o.categoria)
	}
	fmt.Printf("\nsalida    %s\n\n", o.salida)

	// Sin modelos explicitos se corre UNA vez con la cadena del YAML, failover
	// incluido: es lo que de verdad va a pasar en produccion.
	if len(o.modelos) == 0 {
		o.modelos = []string{""}
	}

	var todas []corrida
	for _, m := range o.modelos {
		for i := range o.repeticiones {
			c := generar(o, strings.TrimSpace(m), i+1)
			todas = append(todas, c)
			imprimir(c)
		}
	}

	tabla(todas)
	fmt.Printf("\nMiralas: %s\n", o.salida)
	return nil
}

func generar(o opciones, modelo string, n int) corrida {
	c := corrida{modelo: modelo}
	if modelo == "" {
		c.modelo = "(cadena del YAML)"
	}

	cfg, err := config.Cargar(o.rutaConfig)
	if err != nil {
		c.err = err
		return c
	}
	if modelo != "" {
		cfg.IA.Tareas[string(ai.GenerarFoto)] = []string{modelo}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cliente, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.GenerarFoto)
	if err != nil {
		c.err = err
		return c
	}

	base := domain.Receta{Fondo: o.fondo, Recipiente: o.recipiente}
	if o.tipico != "" {
		delBanco, err := tipicoDelBanco(ctx, o.tipico)
		if err != nil {
			c.err = err
			return c
		}
		// El mismo plegado que produccion: la vajilla del dueno no le gana el
		// recipiente a una bebida. Si esto se hiciera aqui a mano, el
		// laboratorio dejaria de medir lo que corre de verdad.
		base = delBanco.ConLaBaseDelDueno(base)
	}

	prompt := app.PromptFoto(
		domain.Plato{Nombre: o.plato, Categoria: o.categoria, FotoAjuste: o.ajuste},
		[]domain.Tipo{domain.Tipo(o.tipo)},
		base,
	)
	if o.vajilla {
		prompt = app.PromptDeVajilla(base)
	}

	var referencias []ai.Imagen
	if o.editar != "" {
		bytes, err := os.ReadFile(o.editar)
		if err != nil {
			c.err = err
			return c
		}
		referencias = []ai.Imagen{{Bytes: bytes, MIME: "image/jpeg"}}
		// Editando, la receta entera sobra y estorba: se pide SOLO el cambio.
		prompt = "Keep this exact photograph and change only one thing: " + o.ajuste +
			". Everything else stays identical: the same dish, the same plate, the same " +
			"food, the same background, the same lighting and the same framing."
	}

	pet := ai.Peticion{
		Tarea:       ai.GenerarFoto,
		Referencias: referencias,
		Prompt:      prompt,
		Tamano:      o.tamano,
		Calidad:     o.calidad,
		Proporcion:  o.proporcion,
	}

	inicio := time.Now()
	resp, err := cliente.Ejecutar(ctx, pet)
	c.duracion = time.Since(inicio)
	c.uso = resp.Uso
	if err != nil {
		c.err = err
		return c
	}
	if len(resp.Imagenes) == 0 {
		c.err = fmt.Errorf("no devolvio ninguna imagen")
		return c
	}

	img := resp.Imagenes[0]
	c.bytes = len(img.Bytes)
	// Una carpeta por plato y otra por MODELO, no por proveedor: lo que se compara
	// son modelos, y dos modelos del mismo proveedor en la misma carpeta obligan a
	// descifrar nombres de archivo para saber cual es cual.
	//
	// El tamano y la calidad van en el nombre porque si no, 1K y 512 se
	// sobrescriben y te quedas mirando la misma imagen creyendo que son dos.
	// El modelo que de verdad respondio, que con la cadena del YAML puede ser el
	// respaldo y no el primero.
	usado := resp.Uso.Modelo
	if usado == "" {
		usado = modelo
	}
	c.modelo = usado
	dir := filepath.Join(o.salida, sanear(o.plato), sanear(strings.ReplaceAll(usado, "/", "-")))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.err = err
		return c
	}

	partes := []string{fmt.Sprint(n)}
	if o.tamano != "" {
		partes = append(partes, sanear(o.tamano))
	}
	if o.calidad != "" {
		partes = append(partes, sanear(o.calidad))
	}
	if o.proporcion != "" {
		partes = append(partes, strings.ReplaceAll(o.proporcion, ":", "-"))
	}
	c.archivo = filepath.Join(dir, strings.Join(partes, "-")+extension(img.MIME))
	if err := os.WriteFile(c.archivo, img.Bytes, 0o644); err != nil {
		c.err = err
		return c
	}

	// El prompt se guarda al lado de la imagen. Sin esto, dentro de dos semanas
	// nadie sabe con que texto salio cada foto.
	texto := "=== ENVIADO ===\n" + pet.Prompt
	if resp.PromptReescrito != "" {
		texto += "\n\n=== EL PROVEEDOR LO REESCRIBIO ASI ===\n" + resp.PromptReescrito
	}
	_ = os.WriteFile(strings.TrimSuffix(c.archivo, filepath.Ext(c.archivo))+".prompt.txt",
		[]byte(texto+"\n"), 0o644)

	return c
}

func imprimir(c corrida) {
	if c.err != nil {
		fmt.Printf("\033[1m%s\033[0m\n  \033[31mERROR: %v\033[0m\n\n", c.modelo, c.err)
		return
	}
	fmt.Printf("\033[1m%s\033[0m\n", c.modelo)
	fmt.Printf("  tiempo  %.1fs   costo $%.4f   peso %d KB\n",
		c.duracion.Seconds(), c.uso.CostoUSD, c.bytes/1024)
	fmt.Printf("  %s\n\n", c.archivo)
}

func tabla(todas []corrida) {
	fmt.Println(strings.Repeat("─", 74))
	fmt.Printf("%-34s %9s %10s %8s %14s\n", "modelo", "tiempo", "costo", "KB", "x60 platos")
	fmt.Println(strings.Repeat("─", 74))
	for _, c := range todas {
		if c.err != nil {
			fmt.Printf("%-34s %9s\n", c.modelo, "ERROR")
			continue
		}
		fmt.Printf("%-34s %8.1fs %10s %8d %14s\n",
			c.modelo, c.duracion.Seconds(),
			fmt.Sprintf("$%.4f", c.uso.CostoUSD), c.bytes/1024,
			fmt.Sprintf("$%.2f", c.uso.CostoUSD*60))
	}
	fmt.Println(strings.Repeat("─", 74))
	fmt.Println("\"x60 platos\" es lo que costaria ilustrar una carta entera.")
	fmt.Println("El costo sale de la tabla de precios del YAML, no del proveedor: es un ESTIMADO.")
}

func sanear(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == 'k', r == 'x':
			b.WriteRune(r)
		case r == ' ', r == '/', r == '-':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func extension(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	return ".png"
}
