package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// Lector lee una carta a partir de sus imagenes. Se declara aqui como interfaz
// para que el handler pueda probarse sin gastar dinero en la IA.
type Lector interface {
	Ejecutar(ctx context.Context, imagenes []ai.Imagen) (*app.Resultado, error)
}

// Guardador es lo que el handler necesita de la persistencia.
type Guardador interface {
	GuardarCarta(ctx context.Context, importacionID id.ID,
		carta domain.Carta, cruda []byte, marcas domain.Marcas) error
	MarcarFallida(ctx context.Context, importacionID id.ID, motivo string) error
	Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error)
	TokenHash(ctx context.Context, importacionID id.ID) ([]byte, error)
	AnotarGastoDeLectura(ctx context.Context, importacionID id.ID, uso ai.Uso) error

	// ImportacionDePlato resuelve de quien es un plato: los endpoints de foto
	// reciben un plato, pero el token que autoriza es de la importacion.
	ImportacionDePlato(ctx context.Context, platoID id.ID) (id.ID, error)

	GuardarNombre(ctx context.Context, importacionID id.ID, nombre string) error

	// BancoDePlatos es el catalogo de platos tipicos. Hace falta en el camino
	// SINCRONO: sin el, los datos de desarrollo nacen sin saber que es cada
	// plato y sus fotos salen distintas de las de produccion.
	BancoDePlatos(ctx context.Context) ([]domain.PlatoTipico, error)
}

// Encolador pone a leer la carta en segundo plano. Se declara aqui como
// interfaz para que el handler se pueda probar sin base de datos ni cola.
type Encolador interface {
	EncolarLectura(ctx context.Context, importacionID id.ID) error
}

type Handler struct {
	importar    *app.Importar
	lector      Lector
	encolador   Encolador
	fotos       GeneradorDeFotos
	editar      EditorDePlato
	estilo      Estilista
	negocio     Negocio
	referencias GestorDeReferencias
	paginas     GestorDePaginas
	portada     GestorDePortada
	logo        GestorDeLogo
	reconocedor Reconocedor
	publicador  Publicador
	repo        Guardador
	url         URLDeClave
	log         *slog.Logger

	// sincrono hace que POST espere los ~10 s de la lectura en vez de encolarla.
	//
	// Se queda como interruptor y no se borra: sin cola levantada —un test, una
	// maquina sin Postgres, depurar un prompt— es la forma de ejercitar el flujo
	// entero en una sola peticion. En produccion va en false.
	sincrono bool

	maxSubidaBytes int64

	// porFotoUSD es lo que cuesta una foto. Solo se muestra.
	porFotoUSD float64
}

func NuevoHandler(importar *app.Importar, lector Lector, encolador Encolador,
	fotos GeneradorDeFotos, editar EditorDePlato, negocio Negocio, estilo Estilista,
	referencias GestorDeReferencias, paginas GestorDePaginas, portada GestorDePortada, logo GestorDeLogo,
	reconocedor Reconocedor,
	publicador Publicador, repo Guardador, url URLDeClave,
	log *slog.Logger, sincrono bool, maxSubidaMB int, porFotoUSD float64) *Handler {
	return &Handler{
		importar: importar, lector: lector, encolador: encolador,
		fotos: fotos, editar: editar, negocio: negocio, estilo: estilo, referencias: referencias,
		paginas: paginas, portada: portada, logo: logo, reconocedor: reconocedor, publicador: publicador,
		repo: repo, url: url, log: log,
		sincrono: sincrono, maxSubidaBytes: int64(maxSubidaMB) * 1024 * 1024,
		porFotoUSD: porFotoUSD,
	}
}

func (h *Handler) Montar(r fiber.Router) {
	r.Post("/importaciones", h.crear)
	r.Post("/importaciones/:id/carta", h.subirCarta)
	r.Post("/importaciones/:id/carta/releer", h.releerCarta)
	r.Get("/importaciones/:id", h.obtener)

	// El catalogo de tipos SIN token: la primera pantalla lo necesita para
	// dejar elegir antes de que exista restaurante al que pedirle permiso.
	// Es una lista fija de ocho palabras, no hay nada que proteger.
	r.Get("/tipos", h.catalogoDeTipos)
	r.Put("/importaciones/:id/nombre", h.guardarNombre)
	r.Patch("/platos/:id", h.editarPlato)
	r.Delete("/platos/:id", h.quitarPlato)
	r.Post("/platos/:id/recuperar", h.recuperarPlato)
	h.montarFotos(r)
	h.montarEstilo(r)
	h.montarReferencias(r)
	h.montarPaginas(r)
	h.montarPortada(r)
	h.montarLogo(r)
	h.montarReconocer(r)
	h.montarPublicar(r)
}

// crear recibe la carta y devuelve el borrador.
func (h *Handler) crear(c fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return problema(c, http.StatusBadRequest, "la peticion no es un formulario multipart", err.Error())
	}

	nombre := strings.TrimSpace(primero(form.Value["nombre"]))
	imagenes, err := h.leerArchivos(form.File["fotos"])
	if err != nil {
		return traducirError(c, err)
	}

	// Los tipos que no existen se descartan en vez de rechazar la creacion: es
	// la primera pantalla del dueno y perder el nombre que acaba de escribir
	// por una clave mal puesta sale mucho mas caro que ignorarla.
	tipos := domain.TiposValidos(form.Value["tipos"])

	b, err := h.importar.Ejecutar(c.Context(), nombre, tipos, imagenes, ipDe(c))
	if err != nil {
		return traducirError(c, err)
	}

	respuesta := BorradorDTO{
		ID:          b.ID.String(),
		Estado:      string(b.Estado),
		Token:       b.Token,
		Restaurante: RestauranteDTO{Nombre: nombre},
	}

	// Sin hojas no hay nada que leer ni que encolar: el restaurante queda
	// creado y guardado, que es justo lo que pide el primer paso.
	if b.Estado == domain.Nueva {
		return c.Status(http.StatusCreated).JSON(respuesta)
	}

	if !h.sincrono {
		// El camino normal: se encola y se responde en milisegundos. El
		// frontend pinta esqueletos y consulta el estado hasta que pase a
		// "lista".
		//
		// Si encolar falla, la importacion YA existe con sus imagenes
		// guardadas. Se marca fallida en vez de devolver un 500 a secas: un
		// borrador que existe y dice por que fallo se puede reintentar; uno
		// que existe y parece colgado para siempre, no.
		if err := h.encolador.EncolarLectura(c.Context(), b.ID); err != nil {
			h.log.Error("no se pudo encolar la lectura", "importacion", b.ID, "error", err)
			if e := h.repo.MarcarFallida(c.Context(), b.ID, "no se pudo encolar la lectura"); e != nil {
				h.log.Error("y tampoco marcarla fallida", "error", e)
			}
			respuesta.Estado = string(domain.Fallida)
			return c.Status(http.StatusCreated).JSON(respuesta)
		}
		return c.Status(http.StatusAccepted).JSON(respuesta)
	}

	// Sincrono: se lee aqui mismo. El contexto NO es el de la peticion.
	//
	// c.Context() en Fiber v3 devuelve context.Background() si nadie llamo a
	// SetContext, y no se cancela nunca — asi que no sirve para acotar esto. Y
	// aunque se cancelara, tampoco convendria: la llamada a la IA ya se pago
	// cuando el cliente cierra la pestana, y tirar el resultado es tirar dinero.
	fondo, cancelar := context.WithTimeout(context.WithoutCancel(c.Context()), tiempoDeLectura)
	defer cancelar()

	if err := h.leerYGuardar(fondo, b.ID, imagenes); err != nil {
		h.log.Error("no se pudo leer la carta", "importacion", b.ID, "error", err)
		// La importacion existe y quedo marcada como fallida: se responde 201
		// igual, con el estado, para que la pantalla lo muestre en vez de
		// perder el borrador.
		respuesta.Estado = string(domain.Fallida)
		return c.Status(http.StatusCreated).JSON(respuesta)
	}

	respuesta.Estado = string(domain.Lista)
	return c.Status(http.StatusCreated).JSON(respuesta)
}

// subirCarta recibe las hojas de un restaurante que ya existe y las manda a
// leer. Es la segunda mitad de lo que antes hacia el POST de creacion.
func (h *Handler) subirCarta(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	form, err := c.MultipartForm()
	if err != nil {
		return problema(c, http.StatusBadRequest, "la peticion no es un formulario multipart", err.Error())
	}
	imagenes, err := h.leerArchivos(form.File["fotos"])
	if err != nil {
		return traducirError(c, err)
	}

	if err := h.importar.SubirCarta(c.Context(), impID, imagenes); err != nil {
		return traducirError(c, err)
	}

	// Sin cola levantada se lee aqui mismo, igual que en la creacion. Sin esta
	// rama, el modo sincrono —que es como se depura un prompt sin Postgres ni
	// River— dejaria la carta en 'leyendo' para siempre.
	if h.sincrono {
		fondo, cancelar := context.WithTimeout(context.WithoutCancel(c.Context()), tiempoDeLectura)
		defer cancelar()

		if err := h.leerYGuardar(fondo, impID, imagenes); err != nil {
			h.log.Error("no se pudo leer la carta", "importacion", impID, "error", err)
			return c.JSON(fiber.Map{"estado": string(domain.Fallida)})
		}
		return c.JSON(fiber.Map{"estado": string(domain.Lista)})
	}

	// Mismo trato que en la creacion: si encolar falla, las hojas YA estan
	// guardadas, asi que se marca fallida en vez de devolver un 500 pelado.
	if err := h.encolador.EncolarLectura(c.Context(), impID); err != nil {
		h.log.Error("no se pudo encolar la lectura", "importacion", impID, "error", err)
		if e := h.repo.MarcarFallida(c.Context(), impID, "no se pudo encolar la lectura"); e != nil {
			h.log.Error("y tampoco marcarla fallida", "error", e)
		}
		return c.JSON(fiber.Map{"estado": string(domain.Fallida)})
	}
	return c.Status(http.StatusAccepted).JSON(fiber.Map{"estado": string(domain.Leyendo)})
}

// releerCarta vuelve a leer las hojas que ya estan guardadas, desde cero.
//
// CUESTA una lectura y BORRA los platos actuales con sus fotos. La pantalla
// avisa de las dos cosas antes de llamar aqui; el backend no puede saber si el
// dueno lo entendio, asi que lo unico que hace es no dejar que pase por
// accidente: solo desde 'lista', y solo por este endpoint.
func (h *Handler) releerCarta(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	if err := h.importar.Releer(c.Context(), impID); err != nil {
		return traducirError(c, err)
	}

	if err := h.encolador.EncolarLectura(c.Context(), impID); err != nil {
		h.log.Error("no se pudo encolar la relectura", "importacion", impID, "error", err)
		if e := h.repo.MarcarFallida(c.Context(), impID, "no se pudo encolar la lectura"); e != nil {
			h.log.Error("y tampoco marcarla fallida", "error", e)
		}
		return c.JSON(fiber.Map{"estado": string(domain.Fallida)})
	}
	return c.Status(http.StatusAccepted).JSON(fiber.Map{"estado": string(domain.Leyendo)})
}

// guardarNombre renombra el restaurante. El slug publicado no se mueve hasta la
// siguiente publicacion: la direccion que el dueno ya repartio sigue viva.
func (h *Handler) guardarNombre(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	var cuerpo struct {
		Nombre string `json:"nombre"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	nombre := strings.TrimSpace(cuerpo.Nombre)
	if nombre == "" {
		return traducirError(c, app.ErrNombreVacio)
	}
	if err := h.repo.GuardarNombre(c.Context(), impID, nombre); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// leerYGuardar es lo que va a hacer el worker de River en la etapa 4, tal cual.
func (h *Handler) leerYGuardar(ctx context.Context, importacionID id.ID, imagenes []app.Imagen) error {
	res, err := h.lector.Ejecutar(ctx, app.ImagenesParaIA(imagenes))
	if err != nil {
		// El motivo se guarda: sin el, el dueno ve "fallida" y nadie sabe por que.
		if e := h.repo.MarcarFallida(ctx, importacionID, err.Error()); e != nil {
			h.log.Error("no se pudo marcar la importacion como fallida", "error", e)
		}
		return err
	}

	// El gasto se anota SIEMPRE, incluso si guardar la carta falla despues: la
	// llamada ya se pago y no anotarla es perder plata de vista.
	if err := h.repo.AnotarGastoDeLectura(ctx, importacionID, res.Uso); err != nil {
		h.log.Error("no se pudo anotar el gasto", "importacion", importacionID, "error", err)
	}

	if banco, err := h.repo.BancoDePlatos(ctx); err != nil {
		h.log.Warn("no se pudo leer el banco de platos", "importacion", importacionID, "error", err)
	} else {
		res.Carta.EmparejarConElBanco(banco)
	}

	cruda, err := marshalCarta(res.Carta)
	if err != nil {
		return err
	}
	return h.repo.GuardarCarta(ctx, importacionID, res.Carta, cruda, res.Marcas)
}

// obtener devuelve el catalogo. Es lo que el frontend consulta en bucle mientras
// el estado sea "leyendo".
func (h *Handler) obtener(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		// 400 y no 404: "esto no es un id" es distinto de "este id no existe", y
		// confundirlos manda a buscar un bug donde no lo hay.
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}

	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	imp, err := h.repo.Obtener(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}
	return c.JSON(aImportacionDTO(imp, h.url, h.porFotoUSD))
}

// Los errores de autorizacion. Son valores y no respuestas escritas: quien
// decide NO escribe, y quien escribe NO decide.
//
// ASI ES COMO ESTO FILTRABA EL CATALOGO. La primera version hacia que autorizar
// devolviera problema(c, 401, ...) directamente, y problema() termina en
// c.Status(401).JSON(...), que en Fiber devuelve nil cuando escribe bien. O sea
// que autorizar devolvia nil —"adelante"— justo cuando estaba rechazando. El
// handler seguia, y el c.JSON(catalogo) del final sobrescribia el cuerpo dejando
// el 401 puesto.
//
// Lo peor fue como casi se me escapa: lo probe con curl, vi 401 y 404, y lo di
// por bueno. Estaba mirando SOLO el codigo de estado. El cuerpo traia la carta
// entera. Es el mismo error que con el recorrido de rutas: comprobar la senal
// facil en vez del efecto.
var (
	errSinToken      = errors.New("falta el token del borrador")
	errTokenInvalido = errors.New("el token no corresponde a este borrador")
)

// autorizar comprueba el token del dueno y NO escribe nada.
//
// Sin registro, el token es lo unico que separa un borrador de otro. Se compara
// en tiempo constante dentro de app.TokenValido.
func (h *Handler) autorizar(ctx context.Context, cabecera string, impID id.ID) error {
	token := strings.TrimPrefix(cabecera, "Bearer ")
	if token == "" {
		return errSinToken
	}

	hash, err := h.repo.TokenHash(ctx, impID)
	if err != nil {
		return err
	}

	if !app.TokenValido(token, hash) {
		return errTokenInvalido
	}
	return nil
}

// leerArchivos pasa el multipart a memoria, comprobando el tipo real.
// Cero archivos NO es error aqui: crear el restaurante sin carta es el primer
// paso del flujo. Quien si lo exige es SubirCarta, que es donde no tener hojas
// significa que no hay nada que leer.
func (h *Handler) leerArchivos(archivos []*multipart.FileHeader) ([]app.Imagen, error) {
	imagenes := make([]app.Imagen, 0, len(archivos))
	for _, a := range archivos {
		if a.Size > h.maxSubidaBytes {
			return nil, fmt.Errorf("%w: %s pesa %d MB", app.ErrImagenInvalida,
				a.Filename, a.Size/(1024*1024))
		}

		f, err := a.Open()
		if err != nil {
			return nil, fmt.Errorf("abriendo %s: %w", a.Filename, err)
		}
		bytes, err := io.ReadAll(io.LimitReader(f, h.maxSubidaBytes))
		// El error de cerrar un archivo que solo se leyo no aporta nada: lo que
		// importa es el error de ReadAll, que ya se comprueba abajo.
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("leyendo %s: %w", a.Filename, err)
		}

		// El tipo se SNIFFEA, no se lee de la cabecera: el Content-Type del
		// formulario lo pone el cliente y puede decir lo que quiera. Un .exe
		// declarado como image/jpeg no puede llegar al almacen.
		mime := http.DetectContentType(bytes)
		if i := strings.Index(mime, ";"); i > 0 {
			mime = mime[:i]
		}

		imagenes = append(imagenes, app.Imagen{Bytes: bytes, MIME: mime})
	}
	return imagenes, nil
}

func ipDe(c fiber.Ctx) *netip.Addr {
	a, err := netip.ParseAddr(c.IP())
	if err != nil {
		return nil
	}
	return &a
}

func primero(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// traducirError convierte los errores conocidos en su codigo HTTP. Lo que no
// reconoce es un 500 con el detalle en el log, nunca en la respuesta.
func traducirError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errSinToken):
		return problema(c, http.StatusUnauthorized, err.Error(), "")

	// 404 y no 403 para un token equivocado: responder "existe pero no es tuyo"
	// confirma que ese id existe, y con ids inadivinables eso es justo lo unico
	// que hay que proteger. Por eso comparte respuesta con "no existe".
	case errors.Is(err, errTokenInvalido), errors.Is(err, postgres.ErrNoExiste):
		return problema(c, http.StatusNotFound, "no existe", "")

	// 402 y no 500: no hay nada roto, se acabo el presupuesto de esta carta y
	// la pantalla tiene que poder decirlo con esas palabras.
	case errors.Is(err, app.ErrSinPresupuesto):
		return problema(c, http.StatusPaymentRequired, err.Error(), "")

	case errors.Is(err, app.ErrNoSePuedePublicar),
		errors.Is(err, app.ErrCartaYaSubida),
		errors.Is(err, app.ErrNoSePuedeReleer),
		errors.Is(err, app.ErrDemasiadasReferencias),
		errors.Is(err, app.ErrDemasiadasPaginas),
		errors.Is(err, app.ErrCartaOcupada):
		return problema(c, http.StatusConflict, err.Error(), "")

	case errors.Is(err, app.ErrNombreVacio),
		errors.Is(err, app.ErrSinImagenes),
		errors.Is(err, app.ErrDemasiadas),
		errors.Is(err, app.ErrImagenInvalida),
		errors.Is(err, app.ErrAjusteLargo),
		errors.Is(err, app.ErrTextoLargo),
		errors.Is(err, app.ErrRanuraDesconocida),
		errors.Is(err, app.ErrEtiquetaLarga),
		errors.Is(err, app.ErrPrecioInvalido),
		errors.Is(err, app.ErrImagenAjena),
		errors.Is(err, app.ErrSinLetrero),
		errors.Is(err, app.ErrPaginaDesconocida):
		return problema(c, http.StatusBadRequest, err.Error(), "")
	}
	return problema(c, http.StatusInternalServerError, "algo salio mal", "")
}

func problema(c fiber.Ctx, codigo int, mensaje, detalle string) error {
	return c.Status(codigo).JSON(ErrorDTO{Error: mensaje, Detalle: detalle})
}

// tiempoDeLectura acota la lectura sincrona. Diez segundos es lo medido; con el
// respaldo a 3.5-flash puede llegar a 43, y el margen cubre una carta de dos
// fotos.
const tiempoDeLectura = 2 * time.Minute

func marshalCarta(c domain.Carta) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serializando la carta cruda: %w", err)
	}
	return b, nil
}
