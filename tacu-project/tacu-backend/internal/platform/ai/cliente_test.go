package ai

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// proveedorFalso devuelve una respuesta programada y anota con que modelo se lo
// llamo: es lo que permite afirmar que la cadena se recorrio en orden.
type proveedorFalso struct {
	nombre       string
	capacidad    Capacidad
	err          error
	uso          Uso
	llamado      int
	ultimoModelo string
}

func (p *proveedorFalso) Nombre() string           { return p.nombre }
func (p *proveedorFalso) Soporta(c Capacidad) bool { return c == p.capacidad }

func (p *proveedorFalso) Ejecutar(_ context.Context, modelo string, _ Peticion) (Respuesta, error) {
	p.llamado++
	p.ultimoModelo = modelo
	if p.err != nil {
		return Respuesta{Uso: p.uso}, p.err
	}
	return Respuesta{JSON: []byte(`{"ok":true}`), Uso: p.uso}, nil
}

type libroFalso struct{ anotaciones []Uso }

func (l *libroFalso) Anotar(_ context.Context, u Uso) { l.anotaciones = append(l.anotaciones, u) }

func silencioso() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func modelo(s string) Modelo {
	m, err := ParsearModelo(s)
	if err != nil {
		panic(err)
	}
	return m
}

// cadena de leer_carta: primero Gemini, despues OpenAI.
func cadenaLeerCarta() Cadenas {
	return Cadenas{LeerCarta: {modelo("gemini/gemini-3.7-flash"), modelo("openai/gpt-5.6-luna")}}
}

func peticion() Peticion { return Peticion{Tarea: LeerCarta, Prompt: "lee la carta"} }

// Un 429 en el primer modelo debe pasar al siguiente de la cadena.
func TestFallbackTrasErrorTransitorio(t *testing.T) {
	primero := &proveedorFalso{nombre: "gemini", capacidad: Vision,
		err: Transitorio("gemini", "429", "rate limit", nil)}
	segundo := &proveedorFalso{nombre: "openai", capacidad: Vision}

	libro := &libroFalso{}
	c := NuevoCliente(cadenaLeerCarta(), Precios{}, libro, silencioso()).
		Registrar(primero).Registrar(segundo)

	resp, err := c.Ejecutar(context.Background(), peticion())
	if err != nil {
		t.Fatalf("deberia haber caido al segundo modelo: %v", err)
	}

	if primero.llamado != 1 || segundo.llamado != 1 {
		t.Fatalf("llamadas: primero=%d segundo=%d", primero.llamado, segundo.llamado)
	}
	if resp.Uso.Intentos != 2 {
		t.Fatalf("Intentos = %d, esperaba 2", resp.Uso.Intentos)
	}
	if resp.Uso.Modelo != "openai/gpt-5.6-luna" {
		t.Fatalf("Modelo = %q", resp.Uso.Modelo)
	}
}

// Cada proveedor debe recibir el nombre del modelo que le toca de la cadena, no
// uno fijo: es lo que permite que una misma tarea use modelos distintos.
func TestCadaProveedorRecibeSuModelo(t *testing.T) {
	gemini := &proveedorFalso{nombre: "gemini", capacidad: Vision}
	c := NuevoCliente(cadenaLeerCarta(), Precios{}, &libroFalso{}, silencioso()).Registrar(gemini)

	if _, err := c.Ejecutar(context.Background(), peticion()); err != nil {
		t.Fatal(err)
	}
	if gemini.ultimoModelo != "gemini-3.7-flash" {
		t.Fatalf("se llamo con el modelo %q, esperaba gemini-3.7-flash", gemini.ultimoModelo)
	}
}

// Tareas distintas usan modelos distintos: leer una carta necesita el bueno,
// verificar una foto resuelve con el barato.
func TestCadaTareaUsaSuPropioModelo(t *testing.T) {
	gemini := &proveedorFalso{nombre: "gemini", capacidad: Vision}

	cadenas := Cadenas{
		LeerCarta:     {modelo("gemini/gemini-3.7-flash")},
		VerificarFoto: {modelo("gemini/gemini-3.5-flash-lite")},
	}
	c := NuevoCliente(cadenas, Precios{}, &libroFalso{}, silencioso()).Registrar(gemini)

	if _, err := c.Ejecutar(context.Background(), Peticion{Tarea: LeerCarta}); err != nil {
		t.Fatal(err)
	}
	if gemini.ultimoModelo != "gemini-3.7-flash" {
		t.Fatalf("leer_carta uso %q", gemini.ultimoModelo)
	}

	if _, err := c.Ejecutar(context.Background(), Peticion{Tarea: VerificarFoto}); err != nil {
		t.Fatal(err)
	}
	if gemini.ultimoModelo != "gemini-3.5-flash-lite" {
		t.Fatalf("verificar_foto uso %q, deberia usar el barato", gemini.ultimoModelo)
	}
}

// Un error terminal (peticion mal formada, moderacion) NO se reintenta en otro
// modelo: solo gasta dinero para fallar igual.
func TestErrorTerminalCortaLaCadena(t *testing.T) {
	primero := &proveedorFalso{nombre: "gemini", capacidad: Vision,
		err: Terminal("gemini", "400", "schema invalido", nil)}
	segundo := &proveedorFalso{nombre: "openai", capacidad: Vision}

	c := NuevoCliente(cadenaLeerCarta(), Precios{}, &libroFalso{}, silencioso()).
		Registrar(primero).Registrar(segundo)

	if _, err := c.Ejecutar(context.Background(), peticion()); err == nil {
		t.Fatal("un error terminal deberia propagarse")
	}
	if segundo.llamado != 0 {
		t.Fatalf("se llamo al segundo modelo %d veces tras un error terminal", segundo.llamado)
	}
}

func TestTodosFallanDevuelveTodosLosErrores(t *testing.T) {
	fallaA := errors.New("timeout de gemini")
	fallaB := errors.New("503 de openai")

	c := NuevoCliente(cadenaLeerCarta(), Precios{}, &libroFalso{}, silencioso()).
		Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision,
			err: Transitorio("gemini", "timeout", "timeout", fallaA)}).
		Registrar(&proveedorFalso{nombre: "openai", capacidad: Vision,
			err: Transitorio("openai", "503", "503", fallaB)})

	_, err := c.Ejecutar(context.Background(), peticion())
	if err == nil {
		t.Fatal("esperaba error")
	}
	if !errors.Is(err, fallaA) || !errors.Is(err, fallaB) {
		t.Fatalf("el error no conserva las causas de cada modelo: %v", err)
	}
}

// Una tarea sin cadena configurada es error de config, y tiene que decirlo asi
// en vez de "todos fallaron".
func TestTareaSinCadena(t *testing.T) {
	c := NuevoCliente(Cadenas{}, Precios{}, &libroFalso{}, silencioso()).
		Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision})

	if _, err := c.Ejecutar(context.Background(), peticion()); err == nil {
		t.Fatal("esperaba error de configuracion")
	}
}

// Un YAML con un proveedor que no existe debe matar el arranque, no descubrirse
// cuando un dueno sube su carta.
func TestValidarDetectaProveedorInexistente(t *testing.T) {
	cadenas := Cadenas{
		LeerCarta:      {modelo("inventado/modelo-x")},
		LeerFachada:    {modelo("gemini/g")},
		VerificarFoto:  {modelo("gemini/g")},
		ExpandirPrompt: {modelo("gemini/g")},
		GenerarFoto:    {modelo("gemini/g")},
		GenerarLogo:    {modelo("gemini/g")},
	}
	c := NuevoCliente(cadenas, Precios{}, &libroFalso{}, silencioso()).
		Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision})

	if err := c.Validar(); err == nil {
		t.Fatal("Validar acepto una cadena con un proveedor no registrado")
	}
}

// Y una tarea sin cadena tambien debe matar el arranque.
func TestValidarExigeCadenaParaTodaTarea(t *testing.T) {
	c := NuevoCliente(Cadenas{LeerCarta: {modelo("gemini/g")}}, Precios{}, &libroFalso{}, silencioso()).
		Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision})

	if err := c.Validar(); err == nil {
		t.Fatal("Validar acepto una config sin cadena para las demas tareas")
	}
}

// El costo se anota SIEMPRE. Si no, el unit economics del producto es invisible.
func TestElUsoSeAnotaSiempre(t *testing.T) {
	precios := Precios{
		"gemini/gemini-3.7-flash": {EntradaPorMillon: 0.75, SalidaPorMillon: 3.75},
		"openai/gpt-5.6-luna":     {EntradaPorMillon: 0.20, SalidaPorMillon: 1.20},
	}

	t.Run("en exito", func(t *testing.T) {
		libro := &libroFalso{}
		c := NuevoCliente(cadenaLeerCarta(), precios, libro, silencioso()).
			Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision,
				uso: Uso{TokensEntrada: 1_000_000, TokensSalida: 0}})

		if _, err := c.Ejecutar(context.Background(), peticion()); err != nil {
			t.Fatal(err)
		}
		if len(libro.anotaciones) != 1 {
			t.Fatalf("se anotaron %d usos, esperaba 1", len(libro.anotaciones))
		}
		if got := libro.anotaciones[0].CostoUSD; got != 0.75 {
			t.Fatalf("costo = %v, esperaba 0.75", got)
		}
		if libro.anotaciones[0].Tarea != LeerCarta {
			t.Fatalf("no se anoto la tarea: %q", libro.anotaciones[0].Tarea)
		}
	})

	// Un intento fallido que ya consumio tokens tambien se paga.
	t.Run("en fallo que consumio tokens", func(t *testing.T) {
		libro := &libroFalso{}
		c := NuevoCliente(cadenaLeerCarta(), precios, libro, silencioso()).
			Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision,
				err: Transitorio("gemini", "500", "se corto", nil),
				uso: Uso{TokensEntrada: 1120}}).
			Registrar(&proveedorFalso{nombre: "openai", capacidad: Vision,
				uso: Uso{TokensEntrada: 1000}})

		if _, err := c.Ejecutar(context.Background(), peticion()); err != nil {
			t.Fatal(err)
		}
		if len(libro.anotaciones) != 2 {
			t.Fatalf("se anotaron %d usos, esperaba 2 (el fallido tambien se paga)", len(libro.anotaciones))
		}
	})
}

func TestContextoCanceladoNoGasta(t *testing.T) {
	prov := &proveedorFalso{nombre: "gemini", capacidad: Vision}
	c := NuevoCliente(cadenaLeerCarta(), Precios{}, &libroFalso{}, silencioso()).Registrar(prov)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Ejecutar(ctx, peticion()); err == nil {
		t.Fatal("esperaba error de contexto")
	}
	if prov.llamado != 0 {
		t.Fatalf("se llamo al proveedor %d veces con el contexto cancelado", prov.llamado)
	}
}

func TestParsearModelo(t *testing.T) {
	m, err := ParsearModelo("gemini/gemini-3.7-flash")
	if err != nil {
		t.Fatal(err)
	}
	if m.Proveedor != "gemini" || m.Nombre != "gemini-3.7-flash" {
		t.Fatalf("%+v", m)
	}
	if m.String() != "gemini/gemini-3.7-flash" {
		t.Fatalf("String() = %q", m.String())
	}

	for _, malo := range []string{"", "sinbarra", "/sinproveedor", "sinmodelo/"} {
		if _, err := ParsearModelo(malo); err == nil {
			t.Errorf("ParsearModelo(%q) deberia fallar", malo)
		}
	}
}

// Una carta de 60 platos con Gemini 3.7 Flash debe costar centavos.
func TestCostoDeUnaCarta(t *testing.T) {
	precios := Precios{"gemini/gemini-3.7-flash": {EntradaPorMillon: 0.75, SalidaPorMillon: 3.75}}
	costo := precios.Costo(modelo("gemini/gemini-3.7-flash"), Uso{TokensEntrada: 3360, TokensSalida: 3000})

	if costo > 0.02 {
		t.Fatalf("una carta cuesta $%.5f; se esperaba por debajo de $0.02", costo)
	}
}

// El bug real: el YAML declaraba gemini como respaldo de generar_foto cuando el
// proveedor de gemini solo sabia hacer vision y texto. Validar solo miraba que
// el proveedor estuviera REGISTRADO, asi que la config arrancaba prometiendo un
// respaldo que no existia — y se habria descubierto el dia que OpenAI fallara,
// que es justo el dia en que el respaldo importa.
func TestValidarExigeQueElProveedorSepaHacerLaTarea(t *testing.T) {
	todas := func(m Modelo) Cadenas {
		return Cadenas{
			LeerCarta: {m}, LeerFachada: {m}, VerificarFoto: {m},
			ExpandirPrompt: {m}, OrganizarCarta: {m}, GenerarFoto: {m}, GenerarLogo: {m},
		}
	}

	// Un proveedor que solo hace vision no puede atender generar_foto.
	c := NuevoCliente(todas(modelo("gemini/g")), Precios{}, &libroFalso{}, silencioso()).
		Registrar(&proveedorFalso{nombre: "gemini", capacidad: Vision})

	err := c.Validar()
	if err == nil {
		t.Fatal("Validar acepto un proveedor que no sabe generar imagenes")
	}
	if !strings.Contains(err.Error(), string(GenerarFoto)) {
		t.Fatalf("el error no dice que tarea fallo: %v", err)
	}
}
