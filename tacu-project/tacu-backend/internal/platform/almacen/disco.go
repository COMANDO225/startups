// Package almacen guarda las imagenes.
package almacen

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Disco guarda las imagenes en el sistema de archivos.
//
// Sin interfaz aqui: la interfaz la declara quien la consume (app.Almacen), que
// es como se evita que este paquete tenga que adivinar lo que otro necesita.
//
// Se elige disco para el demo porque 60 fotos son ~47 MB por restaurante y no
// vale una dependencia de nube, credenciales y CORS. Mudarse a R2 es un struct
// nuevo con estos dos metodos, mas un rclone copy, mas una variable de entorno —
// y no toca ni el dominio ni los casos de uso.
type Disco struct {
	raiz string // donde viven los archivos
	base string // el prefijo publico, p.ej. "/media"

	// root confina TODA lectura a la raiz, a nivel de sistema operativo: un
	// ".." o un enlace simbolico que apunte fuera devuelven error en vez de
	// abrir el archivo. Es lo que evita que servir /media/../secreto.txt
	// funcione, y no depende de que quien llama recuerde limpiar la ruta.
	root *os.Root
}

func NuevoDisco(raiz, base string) (*Disco, error) {
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return nil, fmt.Errorf("resolviendo %q: %w", raiz, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("creando %q: %w", abs, err)
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, fmt.Errorf("abriendo el almacen %q: %w", abs, err)
	}
	return &Disco{raiz: abs, base: strings.TrimSuffix(base, "/"), root: root}, nil
}

// Abrir devuelve el archivo de una clave, y NO puede salirse del almacen.
//
// Existe porque servir las imagenes con os.Open(raiz + "/" + loQueVengaEnLaURL)
// es una vulnerabilidad de recorrido de rutas, y la escribi asi en el primer
// intento: GET /media/../secreto.txt devolvia 200 con el archivo. Los 404 que
// vi al probar con /etc/passwd me hicieron creer que estaba protegido, y era
// solo que ese archivo no se podia leer por permisos.
//
// os.Root lo resuelve en el sistema operativo: resiste "..", rutas absolutas y
// enlaces simbolicos que apunten fuera, sin depender de que quien llama limpie
// la ruta primero.
func (d *Disco) Abrir(clave string) (*os.File, error) {
	limpia := strings.TrimPrefix(filepath.Clean("/"+clave), "/")
	if limpia == "" || limpia == "." {
		return nil, fmt.Errorf("la clave esta vacia")
	}
	return d.root.Open(limpia)
}

// Leer devuelve los bytes de una clave y su tipo MIME.
//
// Existe aparte de Abrir porque quien sirve la pagina quiere un io.Reader que
// fasthttp consuma despues del handler, y quien genera una foto quiere los bytes
// enteros en memoria para mandarlos al modelo. Devolver un *os.File a este
// segundo obligaria a cada llamante a acordarse de cerrarlo.
//
// El MIME sale de la EXTENSION y no del contenido: estas claves las escribimos
// nosotros, no vienen de fuera, asi que la extension es fiable y olfatear los
// bytes seria trabajo sin ganancia.
func (d *Disco) Leer(_ context.Context, clave string) ([]byte, string, error) {
	f, err := d.Abrir(clave)
	if err != nil {
		return nil, "", fmt.Errorf("abriendo %q: %w", clave, err)
	}
	defer func() { _ = f.Close() }()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, "", fmt.Errorf("leyendo %q: %w", clave, err)
	}

	tipo := mime.TypeByExtension(filepath.Ext(clave))
	if tipo == "" {
		tipo = "image/jpeg"
	}
	return bytes, tipo, nil
}

// Cerrar libera el descriptor de la raiz.
func (d *Disco) Cerrar() error { return d.root.Close() }

// Guardar escribe los bytes bajo esa clave.
//
// La escritura es a un temporal y despues un rename, que en el mismo sistema de
// archivos es atomico. Sin eso, un proceso que muere a mitad deja un JPEG
// truncado que el navegador pinta a medias — y como el archivo existe, nada lo
// vuelve a generar.
func (d *Disco) Guardar(ctx context.Context, clave string, bytes []byte) error {
	destino, err := d.ruta(clave)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return fmt.Errorf("creando el directorio de %q: %w", clave, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(destino), ".parcial-*")
	if err != nil {
		return fmt.Errorf("creando el temporal de %q: %w", clave, err)
	}
	// Si el rename funciono, este Remove falla con "no existe" y da igual: es
	// justo lo que se busca. Si el rename NO funciono, borra el parcial para que
	// no quede basura que nadie limpia.
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(bytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("escribiendo %q: %w", clave, err)
	}
	// Sync antes del rename: sin el, un corte de luz puede dejar el nombre
	// nuevo apuntando a un archivo vacio.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sincronizando %q: %w", clave, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrando el temporal de %q: %w", clave, err)
	}
	if err := os.Rename(tmp.Name(), destino); err != nil {
		return fmt.Errorf("publicando %q: %w", clave, err)
	}
	return os.Chmod(destino, 0o644)
}

// URL arma la direccion publica de una clave. Es la UNICA que sabe como se
// construye, y por eso la base guarda claves y no URLs.
func (d *Disco) URL(clave string) string {
	if clave == "" {
		return ""
	}
	return d.base + "/" + strings.TrimPrefix(clave, "/")
}

// Raiz es donde viven los archivos, para servirlos estaticamente.
func (d *Disco) Raiz() string { return d.raiz }

// ruta traduce una clave a una ruta de disco. Exige que la clave sea una ruta
// relativa limpia y RECHAZA cualquier otra cosa.
//
// El primer intento hacia filepath.Clean("/" + clave) y comprobaba que el
// resultado no se saliera de la raiz. Contenia el escape —"../x" se convierte en
// "/x"— pero lo hacia EN SILENCIO: la base guardaba la clave "../x" y el archivo
// terminaba en otro sitio. Una fila apuntando a donde no esta el archivo es peor
// que un error, porque no se nota hasta que alguien abre la foto.
//
// Hoy las claves las arma el servidor, asi que esto no deberia dispararse nunca.
// Se comprueba igual porque el dia que una clave venga de fuera —el nombre del
// archivo que sube el dueno, un parametro de una URL— un ".." seria escritura
// arbitraria, y para entonces nadie se va a acordar de que aqui faltaba.
func (d *Disco) ruta(clave string) (string, error) {
	if clave == "" {
		return "", fmt.Errorf("la clave esta vacia")
	}
	if strings.HasPrefix(clave, "/") {
		return "", fmt.Errorf("la clave %q es absoluta y tiene que ser relativa", clave)
	}
	for _, parte := range strings.Split(clave, "/") {
		if parte == ".." {
			return "", fmt.Errorf("la clave %q se sale del almacen", clave)
		}
	}

	destino := filepath.Join(d.raiz, filepath.Clean(clave))

	// Cinturon y tirantes: si por algun camino que no previmos el destino queda
	// fuera de la raiz, no se escribe.
	if !strings.HasPrefix(destino, d.raiz+string(os.PathSeparator)) {
		return "", fmt.Errorf("la clave %q se sale del almacen", clave)
	}
	return destino, nil
}
