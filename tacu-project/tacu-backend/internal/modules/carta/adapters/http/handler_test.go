package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/almacen"
)

// Estos tests son lo que antes se comprobaba a mano con curl: asi se encontro
// que un ejecutable renombrado a .jpg entraba. Un hallazgo que solo vive en la
// terminal de alguien vuelve a aparecer en el siguiente despliegue.
//
// Todo va contra dobles en memoria: el handler recibe Lector y Guardador por
// constructor justamente para poder probarse sin base de datos y sin pagarle a
// la IA.

// --- dobles ---

type lectorFalso struct {
	carta domain.Carta
	err   error

	// llamadas cuenta cuantas veces se pidio leer. En asincrono tiene que
	// quedarse en cero: el POST encola y es el worker quien llama a la IA.
	llamadas int
}

func (l *lectorFalso) Ejecutar(context.Context, []ai.Imagen) (*app.Resultado, error) {
	l.llamadas++
	if l.err != nil {
		return nil, l.err
	}
	// Se verifica aqui como lo hace el caso de uso real: sin esto los platos
	// llegarian sin motivo de revision y el DTO no probaria nada.
	carta := l.carta
	marcas := carta.Verificar()
	return &app.Resultado{Carta: carta, Marcas: marcas, Uso: ai.Uso{CostoUSD: 0.002}}, nil
}

// repoFalso hace de las dos caras de la persistencia a la vez —app.Repo y
// Guardador— igual que el repo de Postgres real.
type repoFalso struct {
	mu    sync.Mutex
	filas map[id.ID]*fila
}

type fila struct {
	imp       domain.Importacion
	tokenHash []byte
}

func nuevoRepo() *repoFalso { return &repoFalso{filas: map[id.ID]*fila{}} }

func (r *repoFalso) CrearBorrador(_ context.Context, restauranteID, importacionID id.ID,
	nombre string, _ []domain.Tipo, tokenHash []byte, _ *netip.Addr, estado domain.Estado,
	_ []string, presupuesto dinero.MicrosUSD) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filas[importacionID] = &fila{
		imp: domain.Importacion{
			ID:          importacionID,
			Restaurante: domain.Restaurante{ID: restauranteID, Nombre: nombre},
			Estado:      estado,
			Presupuesto: presupuesto,
		},
		tokenHash: tokenHash,
	}
	return nil
}

func (r *repoFalso) ReleerCarta(_ context.Context, impID id.ID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.filas[impID]
	if !ok || f.imp.Estado != domain.Lista {
		return false, nil
	}
	f.imp.Estado = domain.Leyendo
	return true, nil
}

func (r *repoFalso) GuardarHojasIniciales(_ context.Context, impID id.ID,
	claves []string, estado domain.Estado) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.filas[impID]
	if !ok || f.imp.Estado != domain.Nueva {
		return false, nil
	}
	f.imp.Imagenes, f.imp.Estado = claves, estado
	return true, nil
}

func (r *repoFalso) GuardarCarta(_ context.Context, importacionID id.ID,
	carta domain.Carta, _ []byte, marcas domain.Marcas) error {
	return r.conFila(importacionID, func(f *fila) {
		f.imp.Carta, f.imp.Marcas, f.imp.Estado = carta, marcas, domain.Lista
	})
}

func (r *repoFalso) MarcarFallida(_ context.Context, importacionID id.ID, motivo string) error {
	return r.conFila(importacionID, func(f *fila) {
		f.imp.Estado, f.imp.Error = domain.Fallida, motivo
	})
}

func (r *repoFalso) AnotarGastoDeLectura(_ context.Context, importacionID id.ID, uso ai.Uso) error {
	return r.conFila(importacionID, func(f *fila) {
		f.imp.Gastado += dinero.USD(uso.CostoUSD)
	})
}

func (r *repoFalso) Obtener(_ context.Context, importacionID id.ID) (domain.Importacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.filas[importacionID]
	if !ok {
		return domain.Importacion{}, postgres.ErrNoExiste
	}
	return f.imp, nil
}

// El banco vacio es lo correcto para estos tests: prueban el borde HTTP, no el
// emparejamiento, y con el banco vacio ningun plato empareja y todo sigue como
// antes de que el banco existiera.
func (r *repoFalso) BancoDePlatos(context.Context) ([]domain.PlatoTipico, error) {
	return nil, nil
}

func (r *repoFalso) GuardarNombre(_ context.Context, importacionID id.ID, nombre string) error {
	return r.conFila(importacionID, func(f *fila) { f.imp.Restaurante.Nombre = nombre })
}

func (r *repoFalso) ImportacionDePlato(context.Context, id.ID) (id.ID, error) {
	return id.Nulo, nil
}

func (r *repoFalso) TokenHash(_ context.Context, importacionID id.ID) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.filas[importacionID]
	if !ok {
		return nil, postgres.ErrNoExiste
	}
	return f.tokenHash, nil
}

func (r *repoFalso) conFila(importacionID id.ID, aplicar func(*fila)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.filas[importacionID]
	if !ok {
		return postgres.ErrNoExiste
	}
	aplicar(f)
	return nil
}

// --- montaje ---

type entorno struct {
	f       *fiber.App
	repo    *repoFalso
	lector  *lectorFalso
	cola    *encoladorFalso
	paginas *paginasFalsas
}

// montar arma el borde entero salvo la base y la IA. El almacen es el de disco
// de verdad, sobre un directorio temporal: su URL() es la funcion que el handler
// usa en produccion, y probar con otra no probaria el contrato de las URLs.
func montar(t *testing.T, sincrono bool, lector *lectorFalso) *entorno {
	t.Helper()

	disco, err := almacen.NuevoDisco(t.TempDir(), "/media")
	if err != nil {
		t.Fatalf("almacen: %v", err)
	}

	repo := nuevoRepo()
	cola := &encoladorFalso{}
	paginas := &paginasFalsas{}
	h := NuevoHandler(
		app.NuevoImportar(repo, disco, dinero.USD(3.00)),
		lector, cola, &fotosFalsas{}, &editorFalso{}, &negocioFalso{}, &estiloFalso{}, &referenciasFalsas{},
		paginas, &reconocedorFalso{}, &publicadorFalso{},
		repo, disco.URL,
		slog.New(slog.DiscardHandler),
		sincrono, 10, 0.0336,
	)

	f := fiber.New()
	h.Montar(f.Group("/v1"))
	return &entorno{f: f, repo: repo, lector: lector, cola: cola, paginas: paginas}
}

// fotosFalsas no hace nada: los tests del borde HTTP de importaciones no tocan
// fotos, y las suyas propias se probaran aparte.
type fotosFalsas struct{ encoladas int }

func (f *fotosFalsas) Generar(context.Context, id.ID, []id.ID, bool) (int, error) {
	return f.encoladas, nil
}
func (f *fotosFalsas) SubirPropia(context.Context, id.ID, []byte, string) error { return nil }
func (f *fotosFalsas) Quitar(context.Context, id.ID) error                      { return nil }
func (f *fotosFalsas) AjustarFoto(context.Context, id.ID, string) error         { return nil }

// editorFalso no edita nada: los tests del borde de importaciones no tocan
// platos.
type editorFalso struct{}

func (editorFalso) Etiquetas(context.Context, id.ID, []string) (domain.Plato, domain.Marcas, error) {
	return domain.Plato{}, domain.Marcas{}, nil
}

func (editorFalso) Quitar(context.Context, id.ID) error    { return nil }
func (editorFalso) Recuperar(context.Context, id.ID) error { return nil }

type negocioFalso struct{}

func (negocioFalso) Base(context.Context, id.ID, string) ([]domain.Tipo, domain.Estilo, error) {
	return []domain.Tipo{domain.Generico}, domain.Estilo{}, nil
}
func (negocioFalso) GuardarTipos(context.Context, id.ID, []domain.Tipo) error { return nil }

// estiloFalso no dibuja: dibujar cuesta una llamada de imagen. Lo que se
// ejercita aqui es el borde —el id, el permiso, la ranura— no el modelo.
type estiloFalso struct{}

func (estiloFalso) Leer(context.Context, id.ID, string) (domain.Estilo, error) {
	return domain.Estilo{}, nil
}
func (estiloFalso) GuardarTextos(context.Context, id.ID, string, string, string) error { return nil }

func (estiloFalso) SubirFoto(context.Context, id.ID, string, string, []byte, string) (domain.Estilo, error) {
	return domain.Estilo{Vajilla: domain.Ranura{Foto: "estilo/x.jpg"}}, nil
}

func (estiloFalso) Vaciar(context.Context, id.ID, string, string) (domain.Estilo, error) {
	return domain.Estilo{}, nil
}

func (estiloFalso) Dibujar(context.Context, id.ID, string, string) (domain.Estilo, error) {
	return domain.Estilo{Vajilla: domain.Ranura{Vista: "estilo/x.jpg"}}, nil
}

type referenciasFalsas struct{}

func (referenciasFalsas) AgregarAPlato(context.Context, id.ID, []byte, string) ([]string, error) {
	return nil, nil
}
func (referenciasFalsas) QuitarDePlato(context.Context, id.ID, string) ([]string, error) {
	return nil, nil
}

// paginasFalsas recuerda lo ultimo que se le pidio, que es lo que los tests del
// borde tienen que comprobar: el handler no lee cartas ni guarda archivos.
type paginasFalsas struct {
	claves     []string
	ultimoMIME string
	err        error
}

func (p *paginasFalsas) Agregar(_ context.Context, _ id.ID, bytes []byte, mime string) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	p.ultimoMIME = mime
	p.claves = append(p.claves, fmt.Sprintf("cartas/nueva-%d-%dB.jpg", len(p.claves)+1, len(bytes)))
	return p.claves, nil
}

func (p *paginasFalsas) Quitar(_ context.Context, _ id.ID, clave string) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	fuera := p.claves[:0:0]
	for _, c := range p.claves {
		if c != clave {
			fuera = append(fuera, c)
		}
	}
	p.claves = fuera
	return p.claves, nil
}

func (p *paginasFalsas) QuitarConPlatos(context.Context, id.ID, string) (int, error) {
	return 0, nil
}

func (p *paginasFalsas) Reordenar(_ context.Context, _ id.ID, claves []string) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	p.claves = claves
	return p.claves, nil
}

// reconocedorFalso no habla con ningun modelo: los tests del borde comprueban
// autorizacion y forma de la respuesta, no el emparejamiento.
type reconocedorFalso struct{ emparejados, aprendidos int }

func (r *reconocedorFalso) Reconocer(context.Context, id.ID) (int, int, error) {
	return r.emparejados, r.aprendidos, nil
}

type publicadorFalso struct{}

func (publicadorFalso) Ejecutar(context.Context, id.ID) (string, error) { return "x", nil }
func (publicadorFalso) Carta(context.Context, string) (domain.Importacion, error) {
	return domain.Importacion{}, nil
}

// encoladorFalso registra lo que se encolo, sin cola de verdad.
type encoladorFalso struct {
	encolados []id.ID
	fallar    error
}

func (e *encoladorFalso) EncolarLectura(_ context.Context, importacionID id.ID) error {
	if e.fallar != nil {
		return e.fallar
	}
	e.encolados = append(e.encolados, importacionID)
	return nil
}

func (e *entorno) pedir(t *testing.T, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	// El timeout por defecto de Test es 1 s y con -race el arranque en frio se
	// lo come; el handler en si no espera nada porque el lector es un doble.
	resp, err := e.f.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	defer resp.Body.Close()
	cuerpo, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("leyendo la respuesta: %v", err)
	}
	return resp, cuerpo
}

// peticionCrear arma el multipart. El Content-Type de cada parte se declara
// image/jpeg SIEMPRE, aunque los bytes sean otra cosa: es lo que hace un cliente
// malicioso y es lo que el servidor no debe creerse.
func peticionCrear(t *testing.T, nombre string, fotos ...[]byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if nombre != "" {
		if err := w.WriteField("nombre", nombre); err != nil {
			t.Fatal(err)
		}
	}
	for i, foto := range fotos {
		parte, err := w.CreateFormFile("fotos", "carta.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parte.Write(foto); err != nil {
			t.Fatalf("foto %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httpNuevaPeticion(t, http.MethodPost, "/v1/importaciones", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func peticionObtener(t *testing.T, id, token string) *http.Request {
	t.Helper()
	req := httpNuevaPeticion(t, http.MethodGet, "/v1/importaciones/"+id, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func httpNuevaPeticion(t *testing.T, metodo, url string, cuerpo io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(metodo, url, cuerpo)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// jpeg son bytes que EMPIEZAN como un JPEG: es lo unico que mira
// http.DetectContentType.
func jpeg() []byte { return append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte("x"), 64)...) }

// ejecutable es un PE de Windows. Con extension .jpg y Content-Type image/jpeg
// es exactamente el archivo que hay que rechazar.
func ejecutable() []byte {
	return append([]byte("MZ\x90\x00\x03"), bytes.Repeat([]byte("\x00"), 64)...)
}

func cartaDeEjemplo() domain.Carta {
	return domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Criollos",
		Platos: []domain.Plato{
			{
				ID:          id.Nuevo(),
				Nombre:      "Lomo saltado",
				Descripcion: "con papas fritas",
				Precios:     []domain.Precio{{Texto: "S/ 32.00", Centimos: 3200}},
				Foto: domain.Foto{
					Estado: domain.FotoLista,
					Origen: domain.FotoDeIA,
					Clave:  "cartas/abc/lomo.jpg",
				},
			},
			{
				ID:     id.Nuevo(),
				Nombre: "Ceviche",
				Precios: []domain.Precio{
					{Etiqueta: "personal", Texto: "S/ 25.00", Centimos: 2500},
					{Etiqueta: "fuente", Texto: "S/ 45.00", Centimos: 4500},
				},
				// Sin foto: la tarjeta pinta el recuadro gris.
				Foto: domain.Foto{Estado: domain.SinFoto},
			},
		},
	}}}
}

// crear sube una carta y devuelve el borrador ya deserializado.
func (e *entorno) crear(t *testing.T, nombre string, fotos ...[]byte) BorradorDTO {
	t.Helper()
	resp, cuerpo := e.pedir(t, peticionCrear(t, nombre, fotos...))
	if resp.StatusCode >= 300 {
		t.Fatalf("crear = %d, cuerpo %s", resp.StatusCode, cuerpo)
	}
	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatalf("json del borrador: %v (%s)", err, cuerpo)
	}
	return b
}

// --- POST /v1/importaciones ---

func TestCrearDevuelveElBorradorConIdTokenYEstado(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})

	resp, cuerpo := e.pedir(t, peticionCrear(t, "Pollos Galponcito", jpeg()))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("estado = %d, esperaba 201. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if !id.Valido(b.ID) {
		t.Errorf("id = %q, no es un UUID", b.ID)
	}
	if b.Token == "" {
		t.Error("no devolvio token: sin el, el dueno pierde el borrador para siempre")
	}
	if b.Estado != string(domain.Lista) {
		t.Errorf("estado = %q, esperaba lista", b.Estado)
	}
	if b.Restaurante.Nombre != "Pollos Galponcito" {
		t.Errorf("nombre = %q", b.Restaurante.Nombre)
	}
}

// EL PRIMER PASO DEL FLUJO: el dueno da su nombre y su tipo de negocio, y eso
// ya queda guardado con su token antes de que exista una sola foto de la carta.
func TestCrearSinCartaDejaLaImportacionNueva(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})

	resp, cuerpo := e.pedir(t, peticionCrear(t, "Cevicheria La Tribuna"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("estado = %d, esperaba 201. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if b.Estado != string(domain.Nueva) {
		t.Errorf("estado = %q, esperaba nueva", b.Estado)
	}
	if b.Token == "" {
		t.Error("sin token el dueno pierde el restaurante que acaba de crear")
	}
}

// El token es la unica credencial que existe. Si volviera en el GET, cualquiera
// que consiguiera leer una respuesta se quedaria con el borrador.
func TestElTokenViajaUnaVezYNoVuelveEnElGet(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Galponcito", jpeg())

	resp, cuerpo := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("estado = %d: %s", resp.StatusCode, cuerpo)
	}
	if strings.Contains(string(cuerpo), b.Token) {
		t.Errorf("el token aparece en el GET: %s", cuerpo)
	}
	if strings.Contains(string(cuerpo), `"token"`) {
		t.Errorf("el GET trae un campo token: %s", cuerpo)
	}
}

func TestCrearRechazaLoQueNoPuedeProcesar(t *testing.T) {
	casos := []struct {
		que    string
		nombre string
		fotos  [][]byte
	}{
		{"sin nombre", "", [][]byte{jpeg()}},
		// "sin fotos" ya NO se rechaza: crear el restaurante antes que su carta
		// es el primer paso del flujo. Lo cubre TestCrearSinCartaDejaLaImportacionNueva.
		{"mas de cuatro fotos", "Galponcito",
			[][]byte{jpeg(), jpeg(), jpeg(), jpeg(), jpeg()}},
	}

	for _, c := range casos {
		t.Run(c.que, func(t *testing.T) {
			e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})

			resp, cuerpo := e.pedir(t, peticionCrear(t, c.nombre, c.fotos...))
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("estado = %d, esperaba 400. cuerpo: %s", resp.StatusCode, cuerpo)
			}

			var problema ErrorDTO
			if err := json.Unmarshal(cuerpo, &problema); err != nil || problema.Error == "" {
				t.Errorf("el 400 no explica nada: %s", cuerpo)
			}
			if len(e.repo.filas) != 0 {
				t.Error("creo la importacion a pesar de rechazar la peticion")
			}
		})
	}
}

// EL TEST QUE MAS IMPORTA. El Content-Type del formulario lo pone el cliente y
// puede decir lo que quiera: el tipo se SNIFFEA con http.DetectContentType. Con
// curl se comprobo que sin eso un .exe llamado carta.jpg terminaba en el
// almacen, servido despues desde nuestro dominio.
func TestUnEjecutableRenombradoAJpgSeRechaza(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})

	resp, cuerpo := e.pedir(t, peticionCrear(t, "Galponcito", ejecutable()))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("estado = %d, esperaba 400: se creyo el Content-Type del cliente. cuerpo: %s",
			resp.StatusCode, cuerpo)
	}
	if len(e.repo.filas) != 0 {
		t.Error("el ejecutable creo una importacion")
	}
}

// Un JPEG de verdad entre ejecutables no debe caer con ellos: el sniffeo tiene
// que dejar pasar lo que si es una carta.
func TestUnJpegDeVerdadPasaElSniffeo(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	if b := e.crear(t, "Galponcito", jpeg(), jpeg()); b.ID == "" {
		t.Error("rechazo dos JPEG validos")
	}
}

// Si la IA falla, lo unico que no puede pasar es perder el borrador: las fotos
// ya se subieron y el dueno tendria que volver a empezar.
func TestSiLaIAFallaLaImportacionQuedaFallidaYNoSePierde(t *testing.T) {
	e := montar(t, true, &lectorFalso{err: errors.New("gemini se cayo")})

	resp, cuerpo := e.pedir(t, peticionCrear(t, "Galponcito", jpeg()))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("estado = %d, esperaba 201 con estado fallida. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatal(err)
	}
	if b.Estado != string(domain.Fallida) {
		t.Errorf("estado = %q, esperaba fallida", b.Estado)
	}

	// Y el borrador sigue ahi, consultable con su token y con el motivo.
	_, cuerpoGet := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpoGet, &imp); err != nil {
		t.Fatal(err)
	}
	if imp.Estado != string(domain.Fallida) {
		t.Errorf("el GET dice %q, esperaba fallida", imp.Estado)
	}
	if imp.Error == "" {
		t.Error("no guardo el motivo: el dueno ve 'fallida' y nadie sabe por que")
	}
	if imp.PuedePublicarse {
		t.Error("una importacion fallida no puede publicarse")
	}
}

// --- GET /v1/importaciones/:id ---

func TestObtenerSinCabeceraAutorizacionEs401(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Galponcito", jpeg())

	resp, cuerpo := e.pedir(t, peticionObtener(t, b.ID, ""))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("estado = %d, esperaba 401. cuerpo: %s", resp.StatusCode, cuerpo)
	}
}

// 404 y NO 403: un 403 confirmaria que ese id existe, y con ids inadivinables
// eso es lo unico que hay que proteger.
func TestObtenerConTokenAjenoEs404YNo403(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Galponcito", jpeg())

	resp, cuerpo := e.pedir(t, peticionObtener(t, b.ID, "token-de-otro"))
	if resp.StatusCode == http.StatusForbidden {
		t.Fatal("respondio 403: eso confirma que la importacion existe")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("estado = %d, esperaba 404. cuerpo: %s", resp.StatusCode, cuerpo)
	}
}

// ESTE TEST FALLA A PROPOSITO: documenta una fuga que sigue viva.
//
// autorizar() escribe la respuesta con problema(), pero problema() devuelve lo
// que devuelve c.JSON, que es NIL cuando escribe bien. Asi que obtener() ve
// err == nil, sigue adelante y sobreescribe el cuerpo con el catalogo entero.
// El codigo queda en 401/404 y el JSON trae la carta completa: el id NO es un
// secreto (lo dice id.go), asi que cualquiera que lo tenga se lleva los datos
// sin token.
//
// El arreglo es de una linea en importaciones.go: que autorizar devuelva un
// bool y obtener corte con `if !h.autorizar(c, impID) { return nil }`.
func TestSinTokenValidoNoSeFiltraElCatalogo(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	casos := map[string]string{
		"sin token":     "",
		"token de otro": "token-de-otro",
	}
	for que, token := range casos {
		t.Run(que, func(t *testing.T) {
			_, cuerpo := e.pedir(t, peticionObtener(t, b.ID, token))
			if strings.Contains(string(cuerpo), "Pollos Galponcito") ||
				strings.Contains(string(cuerpo), "Lomo saltado") {
				t.Errorf("devolvio el catalogo a quien no tiene token: %s", cuerpo)
			}
		})
	}
}

// Un id que existe pero con el token de otro borrador tampoco pasa.
func TestElTokenDeUnBorradorNoAbreOtro(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	uno := e.crear(t, "Galponcito", jpeg())
	otro := e.crear(t, "Dona Pancha", jpeg())

	resp, _ := e.pedir(t, peticionObtener(t, uno.ID, otro.Token))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("estado = %d, esperaba 404", resp.StatusCode)
	}
}

// "Esto no es un id" es distinto de "este id no existe": confundirlos manda a
// buscar un bug donde no lo hay.
func TestObtenerConUnIdQueNoEsUUIDEs400(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})

	resp, cuerpo := e.pedir(t, peticionObtener(t, "no-soy-un-uuid", "cualquiera"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("estado = %d, esperaba 400. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	// Un UUID bien formado que no existe es 404, no 400.
	resp, cuerpo = e.pedir(t, peticionObtener(t, id.Nuevo().String(), "cualquiera"))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("estado = %d para un id inexistente, esperaba 404. cuerpo: %s",
			resp.StatusCode, cuerpo)
	}
}

func TestObtenerConElTokenCorrectoDevuelveElCatalogo(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	resp, cuerpo := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("estado = %d: %s", resp.StatusCode, cuerpo)
	}

	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpo, &imp); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if imp.ID != b.ID || imp.Estado != string(domain.Lista) {
		t.Errorf("id = %q estado = %q", imp.ID, imp.Estado)
	}
	if imp.Restaurante.Nombre != "Pollos Galponcito" {
		t.Errorf("nombre = %q", imp.Restaurante.Nombre)
	}
	if len(imp.Categorias) != 1 || imp.Categorias[0].Nombre != "Criollos" {
		t.Fatalf("categorias = %+v", imp.Categorias)
	}

	platos := imp.Categorias[0].Platos
	if len(platos) != 2 {
		t.Fatalf("%d platos, esperaba 2", len(platos))
	}
	if platos[0].Nombre != "Lomo saltado" || platos[0].Descripcion != "con papas fritas" {
		t.Errorf("plato = %+v", platos[0])
	}

	// El precio viaja formateado y en centimos: el frontend no reimplementa el
	// formato de moneda peruano ni calcula sobre texto.
	if len(platos[0].Precios) != 1 {
		t.Fatalf("precios = %+v", platos[0].Precios)
	}
	pr := platos[0].Precios[0]
	if pr.Soles != "S/ 32.00" || pr.Centimos != 3200 || pr.Impreso != "S/ 32.00" {
		t.Errorf("precio = %+v", pr)
	}

	// Desde es el mas barato de las variantes, que es lo que va en la tarjeta.
	if platos[1].Desde != "S/ 25.00" {
		t.Errorf("desde = %q, esperaba el mas barato de las dos variantes", platos[1].Desde)
	}

	// El gasto de la lectura queda a la vista junto a su tope.
	if imp.Gasto.GastadoUSD <= 0 || imp.Gasto.PresupuestoUSD != 3.00 {
		t.Errorf("gasto = %+v", imp.Gasto)
	}
	if !imp.PuedePublicarse {
		t.Error("una carta sin marcas de revision deberia poder publicarse")
	}
}

// El frontend recorre categorias sin comprobar si es null. Un null aqui rompe la
// pantalla justo mientras se espera, que es cuando mas se mira.
func TestMientrasLeeLasCategoriasVienenVaciasPeroNuncaNull(t *testing.T) {
	// Asincrono: se responde antes de leer la carta, asi que el estado es
	// "leyendo" y la carta esta vacia de verdad.
	e := montar(t, false, &lectorFalso{carta: cartaDeEjemplo()})

	resp, cuerpo := e.pedir(t, peticionCrear(t, "Galponcito", jpeg()))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("estado = %d, esperaba 202. cuerpo: %s", resp.StatusCode, cuerpo)
	}
	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatal(err)
	}
	if b.Estado != string(domain.Leyendo) {
		t.Errorf("estado = %q, esperaba leyendo", b.Estado)
	}

	resp, cuerpo = e.pedir(t, peticionObtener(t, b.ID, b.Token))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("estado = %d: %s", resp.StatusCode, cuerpo)
	}
	if !strings.Contains(string(cuerpo), `"categorias":[]`) {
		t.Errorf("categorias no es una lista vacia en el JSON: %s", cuerpo)
	}

	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpo, &imp); err != nil {
		t.Fatal(err)
	}
	if imp.Categorias == nil {
		t.Error("categorias es null")
	}
	if imp.PuedePublicarse {
		t.Error("no se puede publicar una carta que todavia se esta leyendo")
	}
}

// --- el DTO ---

// Los dos niveles son toda la razon de que exista MotivoRevision.Nivel: si el
// manuscrito bloqueara igual que el discordante, media carta queda en rojo y el
// dueno aprende a aprobar sin leer.
func TestElManuscritoQueCuadraNoBloqueaYElDiscordanteSi(t *testing.T) {
	carta := domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Menu",
		Platos: []domain.Plato{
			{
				// Sticker pegado encima: el numero coincide con el texto, solo
				// hay que mirarlo.
				ID:      id.Nuevo(),
				Nombre:  "Arroz con pollo",
				Precios: []domain.Precio{{Texto: "S/ 18.00", Centimos: 1800, Procedencia: domain.Manuscrito}},
			},
			{
				// El modelo dijo 99.00 donde la carta imprime 18.00: eso no se
				// publica.
				ID:      id.Nuevo(),
				Nombre:  "Seco de res",
				Precios: []domain.Precio{{Texto: "S/ 18.00", Centimos: 9900}},
			},
		},
	}}}

	e := montar(t, true, &lectorFalso{carta: carta})
	b := e.crear(t, "Galponcito", jpeg())

	_, cuerpo := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpo, &imp); err != nil {
		t.Fatal(err)
	}

	platos := imp.Categorias[0].Platos
	manuscrito, discordante := platos[0].Revisar, platos[1].Revisar
	if manuscrito == nil || discordante == nil {
		t.Fatalf("faltan marcas: %+v", platos)
	}
	if manuscrito.Bloquea {
		t.Errorf("el precio manuscrito que cuadra bloquea la publicacion: %+v", manuscrito)
	}
	if !discordante.Bloquea {
		t.Errorf("el precio discordante no bloquea: %+v", discordante)
	}
	if manuscrito.Explicacion == "" || discordante.Explicacion == "" {
		t.Error("la marca no trae la frase que se le ensena al dueno")
	}

	// Y el contador los separa igual: uno a revisar, uno a confirmar.
	if imp.Marcas.Revisar != 1 || imp.Marcas.Confirmar != 1 {
		t.Errorf("marcas = %+v, esperaba 1 y 1", imp.Marcas)
	}
	if imp.PuedePublicarse {
		t.Error("con un precio discordante no se puede publicar")
	}
}

// La URL la arma quien sirve las imagenes, no el dominio. Y un plato sin foto
// sale con URL vacia: una URL rota pinta el icono de imagen partida en vez del
// recuadro gris de "subir imagen".
func TestLaURLDeLaFotoLaArmaElAlmacenYSinFotoVaVacia(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Galponcito", jpeg())

	_, cuerpo := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpo, &imp); err != nil {
		t.Fatal(err)
	}

	platos := imp.Categorias[0].Platos
	if platos[0].Foto.URL != "/media/cartas/abc/lomo.jpg" {
		t.Errorf("url = %q, esperaba la que arma el almacen a partir de la clave",
			platos[0].Foto.URL)
	}
	if platos[0].Foto.Estado != string(domain.FotoLista) || platos[0].Foto.Origen != string(domain.FotoDeIA) {
		t.Errorf("foto = %+v", platos[0].Foto)
	}
	if platos[1].Foto.URL != "" {
		t.Errorf("url = %q para un plato sin foto, esperaba vacia", platos[1].Foto.URL)
	}
	if platos[1].Foto.Estado != string(domain.SinFoto) {
		t.Errorf("estado de foto = %q", platos[1].Foto.Estado)
	}
}

// En asincrono el POST no lee la carta: encola y responde en milisegundos. Es la
// diferencia entre 10 segundos y 5, y es lo que hace que la pantalla pueda
// pintar esqueletos en vez de un spinner ciego.
func TestElPostAsincronoEncolaYNoLee(t *testing.T) {
	e := montar(t, false, &lectorFalso{carta: cartaDeEjemplo()}) // asincrono

	res, cuerpo := e.pedir(t, peticionCrear(t, "Pollos Galponcito", jpeg()))

	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("estado = %d, esperaba 202: %s", res.StatusCode, cuerpo)
	}

	var b BorradorDTO
	if err := json.Unmarshal(cuerpo, &b); err != nil {
		t.Fatal(err)
	}
	if b.Estado != string(domain.Leyendo) {
		t.Errorf("estado = %q, esperaba leyendo", b.Estado)
	}

	if n := len(e.cola.encolados); n != 1 {
		t.Fatalf("se encolaron %d lecturas, esperaba 1", n)
	}
	if e.cola.encolados[0].String() != b.ID {
		t.Errorf("se encolo %s y la importacion es %s", e.cola.encolados[0], b.ID)
	}

	// Y NO se llamo a la IA: eso lo hace el worker.
	if e.lector.llamadas != 0 {
		t.Errorf("el handler llamo a la IA %d veces: el POST tiene que devolver ya",
			e.lector.llamadas)
	}
}

// Si encolar falla, la importacion YA existe con sus imagenes guardadas. Un
// borrador que existe y dice por que fallo se puede reintentar; uno que existe y
// parece colgado para siempre, no.
func TestSiNoSePuedeEncolarLaImportacionQuedaFallida(t *testing.T) {
	e := montar(t, false, &lectorFalso{carta: cartaDeEjemplo()})
	e.cola.fallar = errors.New("la cola no responde")

	res, cuerpo := e.pedir(t, peticionCrear(t, "X", jpeg()))

	var b BorradorDTO
	_ = json.Unmarshal(cuerpo, &b)

	if res.StatusCode != http.StatusCreated {
		t.Errorf("estado = %d, esperaba 201 con el borrador", res.StatusCode)
	}
	if b.Estado != string(domain.Fallida) {
		t.Errorf("estado = %q, esperaba fallida", b.Estado)
	}
	if b.ID == "" {
		t.Error("no devolvio el id: el dueno no podria volver a su borrador")
	}
}

// --- PUT /v1/importaciones/:id/nombre ---

func TestRenombrarCambiaElNombreYExigeToken(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	renombrar := func(token, nombre string) int {
		req := httpNuevaPeticion(t, http.MethodPut, "/v1/importaciones/"+b.ID+"/nombre",
			strings.NewReader(`{"nombre":`+strconv.Quote(nombre)+`}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, _ := e.pedir(t, req)
		return resp.StatusCode
	}

	if c := renombrar("", "Otro"); c != http.StatusUnauthorized {
		t.Fatalf("sin token = %d, esperaba 401", c)
	}
	if c := renombrar("token-de-otro", "Otro"); c != http.StatusNotFound {
		t.Fatalf("token ajeno = %d, esperaba 404", c)
	}
	// Un nombre en blanco dejaria al restaurante sin direccion web al publicar.
	if c := renombrar(b.Token, "   "); c != http.StatusBadRequest {
		t.Fatalf("nombre vacio = %d, esperaba 400", c)
	}
	if c := renombrar(b.Token, "  Cevicheria La Tribuna  "); c != http.StatusNoContent {
		t.Fatalf("renombrar = %d, esperaba 204", c)
	}

	_, cuerpo := e.pedir(t, peticionObtener(t, b.ID, b.Token))
	var imp ImportacionDTO
	if err := json.Unmarshal(cuerpo, &imp); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if imp.Restaurante.Nombre != "Cevicheria La Tribuna" {
		t.Errorf("nombre = %q, esperaba el nuevo ya recortado", imp.Restaurante.Nombre)
	}
}

// --- /v1/importaciones/:id/paginas ---

func peticionPagina(t *testing.T, impID, token string, contenido []byte, nombre string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	parte, err := w.CreateFormFile("pagina", nombre)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parte.Write(contenido); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httpNuevaPeticion(t, http.MethodPost, "/v1/importaciones/"+impID+"/paginas", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestAnadirUnaPaginaDevuelve202ConLaListaYExigeToken(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	resp, cuerpo := e.pedir(t, peticionPagina(t, b.ID, "", jpeg(), "hoja2.jpg"))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin token = %d, esperaba 401. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	// 202 y no 200: cuando esto responde, la pagina esta guardada pero sus
	// platos todavia no existen.
	resp, cuerpo = e.pedir(t, peticionPagina(t, b.ID, b.Token, jpeg(), "hoja2.jpg"))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("estado = %d, esperaba 202. cuerpo: %s", resp.StatusCode, cuerpo)
	}

	var dto PaginasDTO
	if err := json.Unmarshal(cuerpo, &dto); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if len(dto.Paginas) != 1 || dto.Paginas[0].URL == "" || dto.Paginas[0].Clave == "" {
		t.Fatalf("paginas = %+v, hacen falta clave y url", dto.Paginas)
	}
}

// El mismo agujero que en crear: el Content-Type del formulario lo pone el
// cliente y puede decir lo que quiera.
func TestElTipoDeUnaPaginaSeSniffeaYNoSeCreeAlCliente(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	req := peticionPagina(t, b.ID, b.Token, []byte("MZ\x90\x00ejecutable"), "carta.jpg")
	if resp, cuerpo := e.pedir(t, req); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("estado = %d: %s", resp.StatusCode, cuerpo)
	}
	if e.paginas.ultimoMIME == "image/jpeg" {
		t.Fatalf("el mime = %q: se creyo el nombre del archivo en vez de mirar los bytes",
			e.paginas.ultimoMIME)
	}
}

func TestReordenarPaginasExigeTokenYDevuelveElOrdenNuevo(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())
	e.paginas.claves = []string{"a.jpg", "b.jpg"}

	orden := func(token string, claves string) (int, []byte) {
		req := httpNuevaPeticion(t, http.MethodPut,
			"/v1/importaciones/"+b.ID+"/paginas", strings.NewReader(`{"claves":`+claves+`}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, cuerpo := e.pedir(t, req)
		return resp.StatusCode, cuerpo
	}

	if c, _ := orden("", `["b.jpg","a.jpg"]`); c != http.StatusUnauthorized {
		t.Fatalf("sin token = %d, esperaba 401", c)
	}

	c, cuerpo := orden(b.Token, `["b.jpg","a.jpg"]`)
	if c != http.StatusOK {
		t.Fatalf("estado = %d: %s", c, cuerpo)
	}
	var dto PaginasDTO
	if err := json.Unmarshal(cuerpo, &dto); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if len(dto.Paginas) != 2 || dto.Paginas[0].Clave != "b.jpg" {
		t.Fatalf("paginas = %+v", dto.Paginas)
	}
}

// --- PUT /v1/importaciones/:id/tipos ---

func TestGuardarTiposRechazaLoQueNoExiste(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	guardar := func(cuerpo string) int {
		req := httpNuevaPeticion(t, http.MethodPut,
			"/v1/importaciones/"+b.ID+"/tipos", strings.NewReader(cuerpo))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.Token)
		resp, _ := e.pedir(t, req)
		return resp.StatusCode
	}

	if c := guardar(`{"tipos":["cevicheria","polleria"]}`); c != http.StatusNoContent {
		t.Fatalf("dos tipos validos = %d, esperaba 204", c)
	}
	// Un tipo inventado no se descarta en silencio: el dueno creeria que quedo
	// guardado.
	if c := guardar(`{"tipos":["cevicheria","marciano"]}`); c != http.StatusBadRequest {
		t.Errorf("tipo inventado = %d, esperaba 400", c)
	}
	if c := guardar(`{"tipos":["cevicheria","cevicheria"]}`); c != http.StatusBadRequest {
		t.Errorf("tipo repetido = %d, esperaba 400", c)
	}
}

// El reparto es lo que sustituyo a la frase de ayuda escrita a mano: tiene que
// venir del servidor y hablar de LA carta, no de una cevicheria generica.
func TestElRepartoDeTiposCuentaLaCartaDeVerdad(t *testing.T) {
	precio := func(texto string, centimos dinero.Centimos) []domain.Precio {
		return []domain.Precio{{Texto: texto, Centimos: centimos}}
	}
	carta := domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "Ceviches", Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado", Precios: precio("s/ 30.00", 3000)},
			{Nombre: "Tiradito", Precios: precio("s/ 32.00", 3200)},
			{Nombre: "Jalea Mixta", Precios: precio("s/ 45.00", 4500)},
		}},
		{Nombre: "Brasas", Platos: []domain.Plato{
			{Nombre: "1/4 pollo a la brasa", Precios: precio("s/ 14.00", 1400)},
			{Nombre: "1/2 pollo a la brasa", Precios: precio("s/ 26.00", 2600)},
			{Nombre: "Pollo entero", Precios: precio("s/ 49.00", 4900)},
		}},
	}}

	e := montar(t, true, &lectorFalso{carta: carta})
	b := e.crear(t, "La Tribuna del Sur", jpeg())

	req := httpNuevaPeticion(t, http.MethodGet, "/v1/importaciones/"+b.ID+"/tipos", nil)
	req.Header.Set("Authorization", "Bearer "+b.Token)
	resp, cuerpo := e.pedir(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("estado = %d: %s", resp.StatusCode, cuerpo)
	}

	var dto TiposDTO
	if err := json.Unmarshal(cuerpo, &dto); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	if dto.Reparto.Total != 6 {
		t.Fatalf("total = %d, esperaba 6", dto.Reparto.Total)
	}
	if len(dto.Catalogo) != len(domain.TiposConocidos) {
		t.Errorf("catalogo = %d entradas", len(dto.Catalogo))
	}

	// El doble de estilo dice que el negocio es "generico", asi que ningun plato
	// se parece a nada y los dos tipos de la carta tienen que salir como que
	// faltan, con su nombre listo para la pantalla.
	if dto.Reparto.SinPistas != 6 {
		t.Errorf("sinPistas = %d, esperaba 6", dto.Reparto.SinPistas)
	}
	faltan := map[string]int{}
	for _, f := range dto.Reparto.Faltan {
		faltan[f.Clave] = f.Platos
		if f.Nombre == "" {
			t.Errorf("%q viene sin nombre para la pantalla", f.Clave)
		}
	}
	if faltan["cevicheria"] != 3 || faltan["polleria"] != 3 {
		t.Errorf("faltan = %+v, esperaba 3 y 3", dto.Reparto.Faltan)
	}
}

// --- POST /v1/importaciones/:id/reconocer ---

func TestReconocerExigeTokenYDevuelveLoQueCambio(t *testing.T) {
	e := montar(t, true, &lectorFalso{carta: cartaDeEjemplo()})
	b := e.crear(t, "Pollos Galponcito", jpeg())

	pedir := func(token string) (int, []byte) {
		req := httpNuevaPeticion(t, http.MethodPost, "/v1/importaciones/"+b.ID+"/reconocer", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, cuerpo := e.pedir(t, req)
		return resp.StatusCode, cuerpo
	}

	if c, _ := pedir(""); c != http.StatusUnauthorized {
		t.Fatalf("sin token = %d, esperaba 401", c)
	}
	if c, _ := pedir("token-de-otro"); c != http.StatusNotFound {
		t.Fatalf("token ajeno = %d, esperaba 404", c)
	}

	c, cuerpo := pedir(b.Token)
	if c != http.StatusOK {
		t.Fatalf("estado = %d: %s", c, cuerpo)
	}
	var dto ReconocidoDTO
	if err := json.Unmarshal(cuerpo, &dto); err != nil {
		t.Fatalf("json: %v (%s)", err, cuerpo)
	}
	// El total sale de la carta de verdad, no del doble: es lo que deja ver si
	// quedaron platos sin emparejar.
	if dto.Total != 2 {
		t.Errorf("total = %d, la carta de ejemplo tiene 2 platos", dto.Total)
	}
}
