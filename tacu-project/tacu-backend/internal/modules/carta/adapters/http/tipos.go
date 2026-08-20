package http

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/modules/carta/domain"
)

// TiposDTO es lo que la pantalla necesita para explicar la eleccion SIN
// inventarse la frase.
//
// El reparto se calcula sobre la carta de verdad y con las mismas pistas que
// deciden la foto, asi que lo que dice la pantalla es literalmente lo que va a
// pasar al generar. Antes esto era un texto fijo en el frontend con un nombre
// metido dentro, y decia lo mismo con dos platos que con doscientos.
type TiposDTO struct {
	// Elegidos, en orden: el primero es el principal.
	Elegidos []string  `json:"elegidos"`
	Catalogo []TipoDTO `json:"catalogo"`

	// Reparto viene vacio mientras la carta se lee: todavia no hay platos que
	// repartir.
	Reparto RepartoDTO `json:"reparto"`
}

type RepartoDTO struct {
	Total int `json:"total"`

	// PorTipo son los elegidos con cuantos platos toma cada uno, en el orden
	// del dueno.
	PorTipo []ConteoDTO `json:"por_tipo"`

	// SinPistas son los platos que no se parecen a ninguno de los elegidos y
	// por tanto salen con la guarnicion del primero.
	SinPistas int `json:"sin_pistas"`

	// Faltan son los tipos NO elegidos que serian el mejor tipo de varios
	// platos. Es el aviso que evita el error caro: una cevicheria-polleria con
	// solo cevicheria marcada saca los pollos con camote y choclo al lado.
	Faltan []ConteoDTO `json:"faltan"`
}

type ConteoDTO struct {
	Clave  string `json:"clave"`
	Nombre string `json:"nombre"`
	Platos int    `json:"platos"`
}

// obtenerTipos devuelve los tipos elegidos, el catalogo y que le pasa a la carta
// con esa eleccion.
func (h *Handler) obtenerTipos(c fiber.Ctx) error {
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

	elegidos, _, err := h.negocio.Base(c.Context(), impID, "")
	if err != nil {
		return traducirError(c, err)
	}

	salida := TiposDTO{
		Elegidos: make([]string, 0, len(elegidos)),
		Catalogo: make([]TipoDTO, 0, len(domain.TiposConocidos)),
	}
	for _, t := range elegidos {
		salida.Elegidos = append(salida.Elegidos, string(t))
	}
	for _, t := range domain.TiposConocidos {
		salida.Catalogo = append(salida.Catalogo, TipoDTO{
			Clave: string(t), Nombre: t.Nombre(), Descripcion: t.Descripcion(),
		})
	}

	r := domain.RepartirCarta(imp.Carta, elegidos)
	salida.Reparto = RepartoDTO{
		Total:     r.Total,
		SinPistas: r.SinPistas,
		PorTipo:   conteos(r.PorTipo),
		Faltan:    conteos(r.Faltan),
	}
	return c.JSON(salida)
}

// conteos traduce y de paso pone el nombre para la pantalla: el frontend no
// tiene por que saber como se llama "sanguicheria" en castellano con tilde.
func conteos(cs []domain.ConteoDeTipo) []ConteoDTO {
	fuera := make([]ConteoDTO, 0, len(cs))
	for _, c := range cs {
		fuera = append(fuera, ConteoDTO{
			Clave:  string(c.Tipo),
			Nombre: c.Tipo.Nombre(),
			Platos: c.Platos,
		})
	}
	return fuera
}

// catalogoDeTipos es el catalogo pelado, SIN restaurante detras.
//
// Existe porque el primer paso —nombre y tipo de negocio— pinta esta lista
// antes de que haya nada creado a lo que pedirle permiso. Lo que no puede dar
// es el reparto: eso se calcula sobre una carta, y aqui todavia no hay.
func (h *Handler) catalogoDeTipos(c fiber.Ctx) error {
	salida := make([]TipoDTO, 0, len(domain.TiposConocidos))
	for _, t := range domain.TiposConocidos {
		salida = append(salida, TipoDTO{
			Clave: string(t), Nombre: t.Nombre(), Descripcion: t.Descripcion(),
		})
	}
	return c.JSON(fiber.Map{"catalogo": salida})
}
