package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// Almacen guarda los bytes de una imagen y sabe devolver su URL publica.
//
// Es una interfaz de UNA implementacion, que normalmente seria de mas. Aqui se
// gana el sitio porque el caso de uso no puede depender de si las fotos viven en
// disco o en R2 —esa decision cambia sin que cambie el flujo— y porque el test
// de importar no necesita escribir en disco para probar que la carta se guarda.
type Almacen interface {
	Guardar(ctx context.Context, clave string, bytes []byte) error

	// Borrar no puede fallar porque la clave no exista: se llama al reemplazar
	// una foto, y reintentar esa operacion no puede romperse por una variante
	// que ya se fue.
	Borrar(ctx context.Context, clave string) error
}

// Repo es lo que Importar necesita de la persistencia, declarado aqui y no en el
// adaptador: el caso de uso define lo que pide, el adaptador se adapta.
type Repo interface {
	CrearBorrador(ctx context.Context, restauranteID, importacionID id.ID,
		nombre string, tipos []domain.Tipo, tokenHash []byte, ip *netip.Addr,
		estado domain.Estado, claves []string, presupuesto dinero.MicrosUSD) error

	// ReleerCarta devuelve la importacion a 'leyendo'. false = no se podia.
	ReleerCarta(ctx context.Context, importacionID id.ID) (bool, error)

	// GuardarHojasIniciales solo muerde mientras la importacion sigue 'nueva':
	// devuelve false si otra pestana ya subio la carta, y asi la segunda no
	// borra los platos de la primera.
	GuardarHojasIniciales(ctx context.Context, importacionID id.ID,
		claves []string, estado domain.Estado) (bool, error)

	GuardarCarta(ctx context.Context, importacionID id.ID,
		carta domain.Carta, cruda []byte, marcas domain.Marcas) error

	MarcarFallida(ctx context.Context, importacionID id.ID, motivo string) error
	Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error)

	// RestauranteDeImportacion abre la clave del objeto: el almacen guarda por
	// tenant y el primer segmento es el restaurante.
	RestauranteDeImportacion(ctx context.Context, importacionID id.ID) (id.ID, error)
}

// Importar crea el borrador y guarda las fotos de la carta. NO la lee: leerla
// tarda diez segundos y eso no puede colgar la peticion del dueno.
type Importar struct {
	repo        Repo
	almacen     Almacen
	presupuesto dinero.MicrosUSD
}

func NuevoImportar(repo Repo, almacen Almacen, presupuesto dinero.MicrosUSD) *Importar {
	return &Importar{repo: repo, almacen: almacen, presupuesto: presupuesto}
}

// Borrador es lo que se le devuelve al dueno al subir la carta.
//
// El Token viaja UNA sola vez, en esta respuesta. No se guarda en claro en
// ningun sitio: la base solo tiene su SHA-256. Si el dueno lo pierde, pierde el
// borrador — es el precio de no tener registro, y es deliberado.
type Borrador struct {
	ID     id.ID
	Token  string
	Estado domain.Estado
}

// Imagen es una foto de la carta tal como llega del formulario.
type Imagen struct {
	Bytes []byte
	MIME  string
}

var (
	ErrSinImagenes     = errors.New("no se recibio ninguna imagen de la carta")
	ErrCartaYaSubida   = errors.New("esta carta ya tiene sus hojas")
	ErrNoSePuedeReleer = errors.New("esta carta no se puede volver a leer ahora")
	ErrDemasiadas      = errors.New("son demasiadas imagenes")
	ErrNombreVacio     = errors.New("falta el nombre del restaurante")
	ErrImagenInvalida  = errors.New("el archivo no es una imagen soportada")
)

// maxImagenes son las fotos que puede tener una carta. Cuatro cubre un menu de
// dos hojas por las dos caras, que es lo mas grande que hemos visto.
const maxImagenes = 4

// Ejecutar guarda las imagenes y crea el borrador.
//
// Las imagenes se guardan ANTES de crear la fila: si el almacen falla, no queda
// una importacion apuntando a fotos que no existen. Al reves —fila primero—
// habria que limpiar, y limpiar es lo que nunca se ejecuta cuando hace falta.
func (uc *Importar) Ejecutar(
	ctx context.Context,
	nombre string,
	tipos []domain.Tipo,
	imagenes []Imagen,
	ip *netip.Addr,
) (*Borrador, error) {
	if nombre == "" {
		return nil, ErrNombreVacio
	}
	// SIN imagenes es el camino normal ahora: el dueno da su nombre y su tipo
	// de negocio, se le guarda con su token, y las hojas llegan en el paso
	// siguiente. Con imagenes tambien vale, que es el flujo de una sola vuelta.
	if len(imagenes) > maxImagenes {
		return nil, fmt.Errorf("%w: %d, el maximo es %d", ErrDemasiadas, len(imagenes), maxImagenes)
	}

	restauranteID := id.Nuevo()
	importacionID := id.Nuevo()

	claves, err := uc.guardarHojas(ctx, restauranteID, importacionID, imagenes)
	if err != nil {
		return nil, err
	}

	// Sin hojas no hay nada que leer, y decir 'leyendo' seria mentir en la
	// pantalla: el dueno veria un cargando eterno esperando un job que nadie
	// encolo.
	estado := domain.Nueva
	if len(claves) > 0 {
		estado = domain.Leyendo
	}

	token, hash, err := nuevoToken()
	if err != nil {
		return nil, err
	}

	if err := uc.repo.CrearBorrador(ctx, restauranteID, importacionID,
		nombre, tipos, hash, ip, estado, claves, uc.presupuesto); err != nil {
		return nil, err
	}

	return &Borrador{ID: importacionID, Token: token, Estado: estado}, nil
}

// SubirCarta guarda las hojas de un restaurante que se creo sin ellas y lo deja
// listo para que lo lea el job.
func (uc *Importar) SubirCarta(ctx context.Context, impID id.ID, imagenes []Imagen) error {
	switch {
	case len(imagenes) == 0:
		return ErrSinImagenes
	case len(imagenes) > maxImagenes:
		return fmt.Errorf("%w: %d, el maximo es %d", ErrDemasiadas, len(imagenes), maxImagenes)
	}

	restauranteID, err := uc.repo.RestauranteDeImportacion(ctx, impID)
	if err != nil {
		return err
	}

	claves, err := uc.guardarHojas(ctx, restauranteID, impID, imagenes)
	if err != nil {
		return err
	}

	subida, err := uc.repo.GuardarHojasIniciales(ctx, impID, claves, domain.Leyendo)
	if err != nil {
		return err
	}
	if !subida {
		return ErrCartaYaSubida
	}
	return nil
}

// Releer vuelve a leer la carta entera desde las hojas que ya estan guardadas.
//
// DESTRUYE lo que hay: la lectura empieza borrando los platos anteriores, asi
// que se llevan por delante las fotos ya generadas y las correcciones del
// dueno. Por eso es una accion suya y explicita, nunca un efecto lateral de
// quitar una hoja — que es lo que uno esperaria y seria carisimo.
func (uc *Importar) Releer(ctx context.Context, impID id.ID) error {
	puede, err := uc.repo.ReleerCarta(ctx, impID)
	if err != nil {
		return err
	}
	if !puede {
		return ErrNoSePuedeReleer
	}
	return nil
}

// guardarHojas escribe los bytes ANTES de tocar la base: al reves habria que
// limpiar la fila cuando el almacen falla, y limpiar es lo que nunca se ejecuta
// cuando hace falta.
func (uc *Importar) guardarHojas(ctx context.Context, restauranteID, impID id.ID, imagenes []Imagen) ([]string, error) {
	claves := make([]string, 0, len(imagenes))
	for i, img := range imagenes {
		ext, ok := extensionDe(img.MIME)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrImagenInvalida, img.MIME)
		}
		clave := ClaveDeHoja(restauranteID, impID, ext)
		if err := GuardarHoja(ctx, uc.almacen, clave, img.Bytes); err != nil {
			return nil, fmt.Errorf("guardando la imagen %d: %w", i+1, err)
		}
		claves = append(claves, clave)
	}
	return claves, nil
}

// nuevoToken devuelve el token en claro y su hash.
//
// 32 bytes de crypto/rand, no el id: el id es un UUIDv7 y su prefijo revela
// cuando se creo. Un secreto se genera secreto entero.
func nuevoToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("generando el token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	suma := sha256.Sum256([]byte(token))
	return token, suma[:], nil
}

// TokenValido compara en tiempo constante.
//
// Con bytes.Equal, el tiempo de respuesta depende de cuantos bytes coinciden, y
// eso deja adivinar el token byte a byte a base de medir. Aqui el coste de
// evitarlo es una linea.
func TokenValido(token string, hash []byte) bool {
	suma := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(suma[:], hash) == 1
}

func extensionDe(mime string) (string, bool) {
	switch mime {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "application/pdf":
		return ".pdf", true
	}
	return "", false
}

// ImagenesParaIA convierte las imagenes del formulario a lo que espera la capa
// de IA. Existe para que el worker no tenga que conocer los dos tipos.
func ImagenesParaIA(imagenes []Imagen) []ai.Imagen {
	out := make([]ai.Imagen, len(imagenes))
	for i, img := range imagenes {
		out[i] = ai.Imagen{Bytes: img.Bytes, MIME: img.MIME}
	}
	return out
}
