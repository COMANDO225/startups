package almacen

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nuevo(t *testing.T) *Disco {
	t.Helper()
	d, err := NuevoDisco(t.TempDir(), "/media")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGuardarYRecuperar(t *testing.T) {
	d := nuevo(t)
	ctx := context.Background()

	contenido := []byte("los bytes de un jpeg")
	if err := d.Guardar(ctx, "cartas/abc/1.jpg", contenido); err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	leido, err := os.ReadFile(filepath.Join(d.Raiz(), "cartas", "abc", "1.jpg"))
	if err != nil {
		t.Fatalf("el archivo no quedo donde dice la clave: %v", err)
	}
	if string(leido) != string(contenido) {
		t.Fatalf("el contenido cambio: %q", leido)
	}
}

// La clave se convierte en una ruta de disco. Un "../" seria escritura fuera del
// almacen: hoy las claves las arma el servidor, pero el dia que una venga de
// fuera —el nombre del archivo que sube el dueno— esto es lo unico que lo para.
func TestUnaClaveNoSePuedeEscaparDelAlmacen(t *testing.T) {
	d := nuevo(t)
	ctx := context.Background()

	fuera := filepath.Join(filepath.Dir(d.Raiz()), "robado.txt")

	escapes := []string{
		"../robado.txt",
		"../../robado.txt",
		"cartas/../../robado.txt",
		"cartas/abc/../../../robado.txt",
		"/../robado.txt",
		"..",
	}
	for _, clave := range escapes {
		err := d.Guardar(ctx, clave, []byte("no deberia existir"))
		if err == nil {
			t.Errorf("la clave %q se acepto", clave)
		} else if !strings.Contains(err.Error(), "se sale del almacen") &&
			!strings.Contains(err.Error(), "es absoluta") {
			t.Errorf("la clave %q fallo por otro motivo: %v", clave, err)
		}
		if _, err := os.Stat(fuera); err == nil {
			t.Fatalf("la clave %q escribio FUERA del almacen", clave)
		}
	}
}

// Una clave absoluta se RECHAZA, no se reescribe. Aceptarla y guardarla en
// raiz/etc/passwd dejaria la fila de la base apuntando a "/etc/passwd" y el
// archivo en otro sitio: nadie lo nota hasta que alguien abre la foto.
func TestUnaClaveAbsolutaSeRechaza(t *testing.T) {
	d := nuevo(t)

	err := d.Guardar(context.Background(), "/etc/passwd", []byte("x"))
	if err == nil {
		t.Fatal("acepto una clave absoluta")
	}
	if _, err := os.Stat(filepath.Join(d.Raiz(), "etc", "passwd")); err == nil {
		t.Fatal("la escribio de todos modos, con otra ruta")
	}
}

// Una clave vacia es un bug del que llama, y silenciarlo escribe un archivo con
// el nombre de un directorio.
func TestUnaClaveVaciaSeRechaza(t *testing.T) {
	if err := nuevo(t).Guardar(context.Background(), "", []byte("x")); err == nil {
		t.Fatal("acepto una clave vacia")
	}
}

// Regenerar una foto sobrescribe la anterior sin dejar restos.
func TestGuardarDosVecesSobrescribe(t *testing.T) {
	d := nuevo(t)
	ctx := context.Background()

	_ = d.Guardar(ctx, "fotos/p/1.jpg", []byte("la primera version"))
	if err := d.Guardar(ctx, "fotos/p/1.jpg", []byte("la segunda")); err != nil {
		t.Fatal(err)
	}

	leido, _ := os.ReadFile(filepath.Join(d.Raiz(), "fotos", "p", "1.jpg"))
	if string(leido) != "la segunda" {
		t.Fatalf("contenido = %q", leido)
	}
}

// La escritura va a un temporal y luego rename. Si quedaran temporales sueltos,
// el directorio de fotos se llenaria de basura que nadie borra.
func TestNoQuedanTemporalesSueltos(t *testing.T) {
	d := nuevo(t)
	ctx := context.Background()

	for i := range 5 {
		if err := d.Guardar(ctx, "fotos/p/"+string(rune('a'+i))+".jpg", []byte("x")); err != nil {
			t.Fatal(err)
		}
	}

	entradas, err := os.ReadDir(filepath.Join(d.Raiz(), "fotos", "p"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		if strings.HasPrefix(e.Name(), ".parcial-") {
			t.Errorf("quedo un temporal sin limpiar: %s", e.Name())
		}
	}
	if len(entradas) != 5 {
		t.Errorf("%d archivos, esperaba 5", len(entradas))
	}
}

// La URL es lo unico que sabe como se sirve una clave. Por eso la base guarda
// claves: mudarse a R2 cambia esta funcion y nada mas.
func TestURL(t *testing.T) {
	d, err := NuevoDisco(t.TempDir(), "/media")
	if err != nil {
		t.Fatal(err)
	}

	casos := map[string]string{
		"cartas/abc/1.jpg":  "/media/cartas/abc/1.jpg",
		"/cartas/abc/1.jpg": "/media/cartas/abc/1.jpg",
		"":                  "", // sin foto no hay URL, ni una rota
	}
	for clave, esperado := range casos {
		if got := d.URL(clave); got != esperado {
			t.Errorf("URL(%q) = %q, esperaba %q", clave, got, esperado)
		}
	}

	// La barra final de la base no puede duplicarse.
	conBarra, _ := NuevoDisco(t.TempDir(), "/media/")
	if got := conBarra.URL("x.jpg"); got != "/media/x.jpg" {
		t.Errorf("URL con base terminada en barra = %q", got)
	}
}

func TestNuevoDiscoCreaLaRaiz(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "no", "existe", "todavia")

	d, err := NuevoDisco(raiz, "/media")
	if err != nil {
		t.Fatalf("NuevoDisco: %v", err)
	}
	if info, err := os.Stat(d.Raiz()); err != nil || !info.IsDir() {
		t.Fatalf("no creo la raiz: %v", err)
	}
	if !filepath.IsAbs(d.Raiz()) {
		t.Errorf("la raiz deberia ser absoluta: %q", d.Raiz())
	}
}

// LA VULNERABILIDAD QUE ESTO EVITA.
//
// Servir las imagenes con os.Open(raiz + "/" + loQueVengaEnLaURL) es recorrido
// de rutas, y asi lo escribi en el primer intento: GET /media/../secreto.txt
// devolvia 200 con el contenido. Lo peor fue el falso negativo — probe con
// /etc/passwd, vi 404, y lo di por seguro; el 404 era por permisos.
//
// Por eso este test crea un archivo REAL justo fuera de la raiz y comprueba que
// no se pueda leer, en vez de confiar en que un 404 significa lo que parece.
func TestAbrirNoPuedeSalirseDelAlmacen(t *testing.T) {
	base := t.TempDir()
	raiz := filepath.Join(base, "media")

	d, err := NuevoDisco(raiz, "/media")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Cerrar() })

	const secreto = "SECRETO-QUE-NO-DEBE-SALIR"
	if err := os.WriteFile(filepath.Join(base, "secreto.txt"), []byte(secreto), 0o600); err != nil {
		t.Fatal(err)
	}

	intentos := []string{
		"../secreto.txt",
		"../../secreto.txt",
		"cartas/../../secreto.txt",
		"cartas/abc/../../../secreto.txt",
		"/../secreto.txt",
		"/etc/passwd",
		"..",
		"",
	}
	for _, clave := range intentos {
		f, err := d.Abrir(clave)
		if err != nil {
			continue // rechazado, que es lo correcto
		}
		contenido, _ := io.ReadAll(f)
		f.Close()
		if strings.Contains(string(contenido), secreto) {
			t.Fatalf("la clave %q FILTRO el archivo de fuera del almacen", clave)
		}
		t.Errorf("la clave %q se abrio sin error", clave)
	}
}

// Un enlace simbolico que apunte fuera tampoco vale: os.Root los sigue, pero no
// deja que salgan de la raiz.
func TestAbrirNoSigueEnlacesQueSalen(t *testing.T) {
	base := t.TempDir()
	raiz := filepath.Join(base, "media")

	d, err := NuevoDisco(raiz, "/media")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Cerrar() })

	const secreto = "SECRETO-POR-ENLACE"
	fuera := filepath.Join(base, "fuera.txt")
	if err := os.WriteFile(fuera, []byte(secreto), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fuera, filepath.Join(raiz, "atajo.txt")); err != nil {
		t.Skipf("sin enlaces simbolicos en este sistema: %v", err)
	}

	f, err := d.Abrir("atajo.txt")
	if err != nil {
		return // rechazado, correcto
	}
	contenido, _ := io.ReadAll(f)
	f.Close()
	if strings.Contains(string(contenido), secreto) {
		t.Fatal("un enlace simbolico saco un archivo de fuera del almacen")
	}
}

// Y lo que SI tiene que funcionar: leer una imagen del almacen.
func TestAbrirLeeLoQueEstaDentro(t *testing.T) {
	d := nuevo(t)
	t.Cleanup(func() { _ = d.Cerrar() })

	if err := d.Guardar(context.Background(), "cartas/abc/1.jpg", []byte("bytes")); err != nil {
		t.Fatal(err)
	}

	f, err := d.Abrir("cartas/abc/1.jpg")
	if err != nil {
		t.Fatalf("no pudo abrir un archivo que si esta dentro: %v", err)
	}
	defer f.Close()

	contenido, _ := io.ReadAll(f)
	if string(contenido) != "bytes" {
		t.Fatalf("contenido = %q", contenido)
	}
}
