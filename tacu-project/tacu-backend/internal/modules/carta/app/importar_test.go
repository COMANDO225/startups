package app

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// --- dobles ---

type almacenFalso struct {
	mu       sync.Mutex
	guardado map[string][]byte
	fallar   error
}

func nuevoAlmacen() *almacenFalso {
	return &almacenFalso{guardado: map[string][]byte{}}
}

func (a *almacenFalso) Guardar(_ context.Context, clave string, b []byte) error {
	if a.fallar != nil {
		return a.fallar
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.guardado[clave] = b
	return nil
}

func (r *repoFalso) RestauranteDeImportacion(context.Context, id.ID) (id.ID, error) {
	return id.Nuevo(), nil
}

func (a *almacenFalso) Borrar(_ context.Context, clave string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.guardado, clave)
	return nil
}

func (a *almacenFalso) URL(clave string) string { return "/media/" + clave }

type repoFalso struct {
	creado   bool
	claves   []string
	nombre   string
	tipos    []domain.Tipo
	estado   domain.Estado
	tope     dinero.MicrosUSD
	fallar   error
	tokenSHA []byte

	hojas    []string
	yaSubida bool
}

func (r *repoFalso) CrearBorrador(_ context.Context, _, _ id.ID, nombre string,
	tipos []domain.Tipo, tokenHash []byte, _ *netip.Addr, estado domain.Estado,
	claves []string, tope dinero.MicrosUSD) error {
	if r.fallar != nil {
		return r.fallar
	}
	r.creado, r.nombre, r.claves, r.tope, r.tokenSHA = true, nombre, claves, tope, tokenHash
	r.tipos, r.estado = tipos, estado
	return nil
}

func (r *repoFalso) GuardarHojasIniciales(_ context.Context, _ id.ID,
	claves []string, estado domain.Estado) (bool, error) {
	if r.fallar != nil {
		return false, r.fallar
	}
	if r.yaSubida {
		return false, nil
	}
	r.hojas, r.estado, r.yaSubida = claves, estado, true
	return true, nil
}

func (r *repoFalso) ReleerCarta(context.Context, id.ID) (bool, error) {
	return !r.yaSubida, nil
}

func (r *repoFalso) GuardarCarta(context.Context, id.ID, domain.Carta, []byte, domain.Marcas) error {
	return nil
}
func (r *repoFalso) MarcarFallida(context.Context, id.ID, string) error { return nil }
func (r *repoFalso) Obtener(context.Context, id.ID) (domain.Importacion, error) {
	return domain.Importacion{}, nil
}

func jpeg() Imagen { return Imagen{Bytes: []byte("\xff\xd8\xff bytes"), MIME: "image/jpeg"} }

// --- tests ---

func TestImportarCreaElBorrador(t *testing.T) {
	repo, alm := &repoFalso{}, nuevoAlmacen()
	uc := NuevoImportar(repo, alm, dinero.USD(3.00))

	b, err := uc.Ejecutar(context.Background(), "Pollos Galponcito", nil, []Imagen{jpeg()}, nil)
	if err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}

	if b.Estado != domain.Leyendo {
		t.Errorf("estado = %q, esperaba leyendo", b.Estado)
	}
	if !repo.creado || repo.nombre != "Pollos Galponcito" {
		t.Errorf("no se creo el borrador bien: %+v", repo)
	}
	if repo.tope != dinero.USD(3.00) {
		t.Errorf("presupuesto = %s", repo.tope)
	}
	if n := len(alm.guardado); n != 1 {
		t.Errorf("%d imagenes guardadas, esperaba 1", n)
	}
	// La clave lleva el id de la importacion: dos cartas distintas no pueden
	// pisarse los archivos.
	if len(repo.claves) != 1 || !strings.Contains(repo.claves[0], b.ID.String()) {
		t.Errorf("la clave no lleva el id de la importacion: %v", repo.claves)
	}
}

// El token viaja UNA vez y solo su hash se guarda. Si se guardara en claro, una
// filtracion de la base daria acceso a todos los borradores.
func TestElTokenNoSeGuardaEnClaro(t *testing.T) {
	repo := &repoFalso{}
	uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

	b, err := uc.Ejecutar(context.Background(), "X", nil, []Imagen{jpeg()}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if b.Token == "" {
		t.Fatal("no devolvio token")
	}
	if len(b.Token) < 40 {
		t.Errorf("el token es corto (%d chars): son 32 bytes de crypto/rand", len(b.Token))
	}
	if strings.Contains(string(repo.tokenSHA), b.Token) {
		t.Fatal("el token se guardo en claro")
	}
	if !TokenValido(b.Token, repo.tokenSHA) {
		t.Error("el hash guardado no valida contra el token devuelto")
	}
	if TokenValido("otro-token", repo.tokenSHA) {
		t.Error("valido un token que no es")
	}
}

// Dos borradores no pueden compartir token, ni siquiera creados a la vez.
func TestCadaBorradorTieneSuToken(t *testing.T) {
	vistos := map[string]bool{}
	for range 200 {
		uc := NuevoImportar(&repoFalso{}, nuevoAlmacen(), dinero.USD(3.00))
		b, err := uc.Ejecutar(context.Background(), "X", nil, []Imagen{jpeg()}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if vistos[b.Token] {
			t.Fatalf("token repetido: %s", b.Token)
		}
		vistos[b.Token] = true
	}
}

func TestImportarRechazaLoQueNoPuedeProcesar(t *testing.T) {
	casos := []struct {
		que      string
		nombre   string
		imagenes []Imagen
		esperado error
	}{
		{"sin nombre", "", []Imagen{jpeg()}, ErrNombreVacio},
		{"demasiadas imagenes", "X",
			[]Imagen{jpeg(), jpeg(), jpeg(), jpeg(), jpeg()}, ErrDemasiadas},
		{"un archivo que no es imagen", "X",
			[]Imagen{{Bytes: []byte("MZ"), MIME: "application/x-msdownload"}}, ErrImagenInvalida},
		{"un zip disfrazado", "X",
			[]Imagen{{Bytes: []byte("PK"), MIME: "application/zip"}}, ErrImagenInvalida},
	}

	for _, c := range casos {
		t.Run(c.que, func(t *testing.T) {
			repo := &repoFalso{}
			uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

			_, err := uc.Ejecutar(context.Background(), c.nombre, nil, c.imagenes, nil)
			if !errors.Is(err, c.esperado) {
				t.Fatalf("err = %v, esperaba %v", err, c.esperado)
			}
			if repo.creado {
				t.Error("creo el borrador a pesar de rechazar la peticion")
			}
		})
	}
}

// Una carta de dos hojas por las dos caras son cuatro fotos, y tienen que entrar.
func TestCuatroImagenesSiEntran(t *testing.T) {
	uc := NuevoImportar(&repoFalso{}, nuevoAlmacen(), dinero.USD(3.00))

	_, err := uc.Ejecutar(context.Background(), "X", nil,
		[]Imagen{jpeg(), jpeg(), jpeg(), jpeg()}, nil)
	if err != nil {
		t.Fatalf("cuatro imagenes deberian entrar: %v", err)
	}
}

func TestLosFormatosQueAceptaUnaCarta(t *testing.T) {
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp", "application/pdf"} {
		uc := NuevoImportar(&repoFalso{}, nuevoAlmacen(), dinero.USD(3.00))
		if _, err := uc.Ejecutar(context.Background(), "X", nil,
			[]Imagen{{Bytes: []byte("x"), MIME: mime}}, nil); err != nil {
			t.Errorf("%s deberia aceptarse: %v", mime, err)
		}
	}
}

// Si el almacen falla, no puede quedar una importacion apuntando a fotos que no
// existen: por eso las imagenes se guardan ANTES de crear la fila.
func TestSiElAlmacenFallaNoQuedaBorradorHuerfano(t *testing.T) {
	repo := &repoFalso{}
	alm := nuevoAlmacen()
	alm.fallar = errors.New("disco lleno")

	uc := NuevoImportar(repo, alm, dinero.USD(3.00))
	_, err := uc.Ejecutar(context.Background(), "X", nil, []Imagen{jpeg()}, nil)

	if err == nil {
		t.Fatal("no fallo con el almacen roto")
	}
	if repo.creado {
		t.Fatal("creo la importacion apuntando a fotos que no existen")
	}
}

func TestLaIPSePasaAlRepo(t *testing.T) {
	repo := &repoFalso{}
	uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

	ip := netip.MustParseAddr("190.234.1.1")
	if _, err := uc.Ejecutar(context.Background(), "X", nil, []Imagen{jpeg()}, &ip); err != nil {
		t.Fatal(err)
	}
	// Sin lector todavia, pero la columna existe para que poner un tope diario
	// sea una consulta y no una migracion.
	if !repo.creado {
		t.Fatal("no se creo")
	}
}

// EL PRIMER PASO DEL FLUJO: el restaurante se guarda con su nombre y su tipo de
// negocio antes de que exista una sola foto de la carta. Sin esto, lo que el
// dueno escribe en la primera pantalla no persiste hasta que suba las hojas.
func TestUnRestauranteSePuedeCrearSinCarta(t *testing.T) {
	repo := &repoFalso{}
	uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

	b, err := uc.Ejecutar(context.Background(), "Cevicheria La Tribuna",
		[]domain.Tipo{domain.Cevicheria}, nil, nil)
	if err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}
	if b.Estado != domain.Nueva {
		t.Errorf("estado = %q, esperaba nueva", b.Estado)
	}
	if b.Token == "" {
		t.Error("sin token el dueno pierde el restaurante que acaba de crear")
	}
	if len(repo.tipos) != 1 || repo.tipos[0] != domain.Cevicheria {
		t.Errorf("tipos = %v", repo.tipos)
	}
	if len(repo.claves) != 0 {
		t.Errorf("no habia hojas que guardar: %v", repo.claves)
	}
}

func TestLasHojasLleganDespuesYDejanLaCartaLeyendo(t *testing.T) {
	repo := &repoFalso{}
	uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

	if err := uc.SubirCarta(context.Background(), id.Nuevo(), []Imagen{jpeg(), jpeg()}); err != nil {
		t.Fatalf("SubirCarta: %v", err)
	}
	if len(repo.hojas) != 2 {
		t.Errorf("hojas = %v", repo.hojas)
	}
	if repo.estado != domain.Leyendo {
		t.Errorf("estado = %q, esperaba leyendo", repo.estado)
	}
}

// Dos pestanas subiendo la misma carta: la segunda NO puede reemplazar las
// hojas de la primera, porque la lectura que ya arranco borra los platos y los
// de la primera se perderian sin que nada avise.
func TestLaSegundaSubidaDeLaMismaCartaSeRechaza(t *testing.T) {
	repo := &repoFalso{yaSubida: true}
	uc := NuevoImportar(repo, nuevoAlmacen(), dinero.USD(3.00))

	err := uc.SubirCarta(context.Background(), id.Nuevo(), []Imagen{jpeg()})
	if !errors.Is(err, ErrCartaYaSubida) {
		t.Fatalf("err = %v, esperaba ErrCartaYaSubida", err)
	}
}

func TestSubirLaCartaSinNingunaHojaSeRechaza(t *testing.T) {
	uc := NuevoImportar(&repoFalso{}, nuevoAlmacen(), dinero.USD(3.00))
	if err := uc.SubirCarta(context.Background(), id.Nuevo(), nil); !errors.Is(err, ErrSinImagenes) {
		t.Fatalf("err = %v, esperaba ErrSinImagenes", err)
	}
}
