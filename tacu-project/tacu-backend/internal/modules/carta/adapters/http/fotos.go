package http

import (
	"context"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// FotosDeImportacion es la lista FLACA para el poll del minuto que dura generar
// las fotos.
//
// No devuelve el catalogo entero a proposito: son ~7 KB para 60 platos en vez de
// la carta completa, y sobre todo permite que el frontend PARCHEE cada tarjeta
// por su id en vez de repintar el catalogo. Sin eso, el scroll salta cada 2
// segundos mientras el dueno esta corrigiendo precios.
type FotosDeImportacion struct {
	// Estado de la importacion, para que el frontend sepa cuando dejar de
	// consultar sin tener que pedir el catalogo aparte.
	Estado string `json:"estado"`

	// Etapa de la lectura, 0..6. El poll de fotos es el que corre mientras la
	// carta se lee, asi que es por aqui por donde llega el progreso.
	Etapa int16 `json:"etapa"`

	// Fotos es un MAPA por id de plato, no una lista: cada tarjeta lee solo su
	// entrada.
	Fotos map[string]FotoDTO `json:"fotos"`

	// Pendientes es cuantas faltan. Cuando llega a cero el frontend apaga el
	// poll solo.
	Pendientes int `json:"pendientes"`

	Gasto GastoDTO `json:"gasto"`
}

// GeneradorDeFotos encola la generacion. Interfaz para poder probar el borde sin
// cola ni base.
type GeneradorDeFotos interface {
	// Generar encola las fotos que faltan. Con soloEstos vacio, todas las que
	// se puedan. Devuelve cuantas se encolaron.
	Generar(ctx context.Context, importacionID id.ID, soloEstos []id.ID, corregir bool) (int, error)

	// SubirPropia guarda la foto que subio el dueno.
	SubirPropia(ctx context.Context, platoID id.ID, bytes []byte, mime string) error

	// Quitar deja el plato con el recuadro gris otra vez.
	Quitar(ctx context.Context, platoID id.ID) error

	// AjustarFoto guarda la correccion del dueno. No regenera.
	AjustarFoto(ctx context.Context, platoID id.ID, ajuste string) error
}

func (h *Handler) montarFotos(r fiber.Router) {
	r.Post("/importaciones/:id/fotos", h.generarFotos)
	r.Get("/importaciones/:id/fotos", h.estadoDeFotos)
	r.Post("/platos/:id/foto", h.subirFoto)
	r.Delete("/platos/:id/foto", h.quitarFoto)
	r.Patch("/platos/:id/foto-ajuste", h.ajustarFoto)
}

type peticionGenerar struct {
	// Platos vacio con Todos en true = generar todas las que falten.
	Platos []string `json:"platos"`
	Todos  bool     `json:"todos"`

	// Corregir edita la foto actual con el ajuste guardado, en vez de hacer una
	// nueva. Solo con platos concretos.
	Corregir bool `json:"corregir,omitempty"`
}

// generarFotos encola la generacion y responde YA. Las fotos van llegando cada
// una por su cuenta.
func (h *Handler) generarFotos(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	var pet peticionGenerar
	if err := c.Bind().JSON(&pet); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es JSON valido", err.Error())
	}
	if !pet.Todos && len(pet.Platos) == 0 {
		return problema(c, http.StatusBadRequest,
			"hay que indicar que platos, o pedir todos", "")
	}

	soloEstos := make([]id.ID, 0, len(pet.Platos))
	for _, s := range pet.Platos {
		pid, ok := parsearID(s)
		if !ok {
			return problema(c, http.StatusBadRequest, "hay un identificador de plato invalido", s)
		}
		soloEstos = append(soloEstos, pid)
	}

	encoladas, err := h.fotos.Generar(c.Context(), impID, soloEstos, pet.Corregir)
	if err != nil {
		h.log.Error("no se pudieron encolar las fotos", "importacion", impID, "error", err)
		return traducirError(c, err)
	}

	return c.Status(http.StatusAccepted).JSON(fiber.Map{"encoladas": encoladas})
}

// estadoDeFotos es el poll flaco.
func (h *Handler) estadoDeFotos(c fiber.Ctx) error {
	impID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	imp, err := h.repo.Obtener(c.Context(), impID)
	if err != nil {
		return traducirError(c, err)
	}

	salida := FotosDeImportacion{
		Estado: string(imp.Estado),
		Etapa:  imp.Etapa,
		Fotos:  make(map[string]FotoDTO),
		Gasto: GastoDTO{
			PorFotoUSD:     h.porFotoUSD,
			GastadoUSD:     imp.Gastado.Dolares(),
			PresupuestoUSD: imp.Presupuesto.Dolares(),
		},
	}

	for _, p := range imp.Carta.Platos() {
		salida.Fotos[p.ID.String()] = FotoDTO{
			Estado: string(p.Foto.Estado),
			Origen: string(p.Foto.Origen),
			URL:    h.url(p.Foto.Clave),
		}
		if p.Foto.Estado == domain.FotoPendiente || p.Foto.Estado == domain.FotoGenerando {
			salida.Pendientes++
		}
	}
	return c.JSON(salida)
}

// subirFoto guarda la que trae el dueno. Siempre gana a la generada.
func (h *Handler) subirFoto(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}

	// El token es de la IMPORTACION, y el parametro aqui es un plato. Se busca a
	// que importacion pertenece antes de comprobar nada.
	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	cabecera, err := c.FormFile("foto")
	if err != nil {
		return problema(c, http.StatusBadRequest, "falta el archivo 'foto'", err.Error())
	}
	if cabecera.Size > h.maxSubidaBytes {
		return problema(c, http.StatusBadRequest, "la imagen pesa demasiado", "")
	}

	f, err := cabecera.Open()
	if err != nil {
		return problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}
	bytes, err := io.ReadAll(io.LimitReader(f, h.maxSubidaBytes))
	_ = f.Close()
	if err != nil {
		return problema(c, http.StatusBadRequest, "no se pudo leer el archivo", "")
	}

	// El tipo se SNIFFEA. La cabecera del formulario la pone el cliente y puede
	// decir lo que quiera: es el mismo motivo por el que un .exe renombrado a
	// .jpg no entra por el endpoint de crear.
	mime := http.DetectContentType(bytes)
	if !esImagenSoportada(mime) {
		return problema(c, http.StatusBadRequest, "el archivo no es una imagen soportada", mime)
	}

	if err := h.fotos.SubirPropia(c.Context(), platoID, bytes, mime); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// quitarFoto devuelve el plato al recuadro gris.
func (h *Handler) quitarFoto(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}

	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	if err := h.fotos.Quitar(c.Context(), platoID); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// ajustarFoto guarda lo que el dueno escribio para corregir la foto del plato.
//
// NO regenera. Guardar y regenerar son dos llamadas a proposito: el dueno
// reescribe el texto las veces que quiera sin gastar, y cuando le convence usa
// el boton de regenerar que ya existe. Una sola ruta que hiciera ambas cosas
// cobraria en cada tecleo arrepentido.
func (h *Handler) ajustarFoto(c fiber.Ctx) error {
	platoID, ok := parsearID(c.Params("id"))
	if !ok {
		return problema(c, http.StatusBadRequest, "el identificador no es valido", "")
	}

	var cuerpo struct {
		Ajuste string `json:"ajuste"`
	}
	if err := c.Bind().JSON(&cuerpo); err != nil {
		return problema(c, http.StatusBadRequest, "el cuerpo no es un JSON valido", "")
	}

	impID, err := h.repo.ImportacionDePlato(c.Context(), platoID)
	if err != nil {
		return traducirError(c, err)
	}
	if err := h.autorizar(c.Context(), c.Get("Authorization"), impID); err != nil {
		return traducirError(c, err)
	}

	if err := h.fotos.AjustarFoto(c.Context(), platoID, cuerpo.Ajuste); err != nil {
		return traducirError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// esImagenSoportada acepta solo lo que el catalogo sabe mostrar. Un PDF vale
// como carta de entrada, pero no como foto de un plato.
func esImagenSoportada(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}
