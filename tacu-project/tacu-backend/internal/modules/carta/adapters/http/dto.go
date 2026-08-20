// Package http expone la carta por HTTP.
package http

import (
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/imagen"
)

// Los DTO existen aparte del dominio a proposito. Si se serializara
// domain.Carta directo, el contrato con el frontend quedaria atado a la forma
// interna: renombrar un campo del dominio romperia la pantalla, y el dominio
// tendria que llevar tags json puestos para complacer a la API.
//
// Aqui ademas se resuelven dos cosas que el dominio no puede saber:
//   - la URL de una foto, porque el dominio guarda la clave y no sabe donde se
//     sirve
//   - el precio en soles ya formateado, para que el frontend no tenga que
//     reimplementar el formato de moneda peruano y equivocarse distinto

type BorradorDTO struct {
	ID     string `json:"id"`
	Estado string `json:"estado"`

	// Token viaja UNA sola vez, en la respuesta de crear. No aparece en ninguna
	// otra respuesta.
	Token string `json:"token,omitempty"`

	Restaurante RestauranteDTO `json:"restaurante"`
}

type RestauranteDTO struct {
	Nombre string `json:"nombre"`
	Slug   string `json:"slug,omitempty"`
}

type ImportacionDTO struct {
	ID          string         `json:"id"`
	Estado      string         `json:"estado"`
	Restaurante RestauranteDTO `json:"restaurante"`

	// Error solo viene con estado "fallida".
	Error string `json:"error,omitempty"`

	// Etapa es en que va la lectura, 0..6. Solo significa algo con estado
	// "leyendo": es lo que permite ensenar progreso de verdad en vez de una
	// animacion que no dice nada.
	Etapa int16 `json:"etapa"`

	Marcas MarcasDTO `json:"marcas"`
	Gasto  GastoDTO  `json:"gasto"`

	// Paginas son las hojas de la carta de papel, en el orden que el dueno les
	// dio.
	Paginas []PaginaDTO `json:"paginas"`

	// Categorias viene vacio mientras el estado es "leyendo": es la senal para
	// que la pantalla siga pintando esqueletos.
	Categorias []CategoriaDTO `json:"categorias"`

	// PuedePublicarse resume la regla en un booleano para que el frontend no
	// tenga que reimplementarla y desincronizarse con el servidor.
	PuedePublicarse bool `json:"puede_publicarse"`
}

type MarcasDTO struct {
	// Revisar son los que NO cuadran y bloquean publicar. Confirmar son los que
	// solo hay que mirar. Mezclarlos deja media carta en rojo y entrena al dueno
	// a aprobar sin leer.
	Revisar   int `json:"revisar"`
	Confirmar int `json:"confirmar"`
}

type GastoDTO struct {
	// En dolares y no en micros: es para mostrar. Los calculos van en enteros
	// del lado del servidor.
	GastadoUSD     float64 `json:"gastado_usd"`
	PresupuestoUSD float64 `json:"presupuesto_usd"`

	// Lo que cuesta una foto, para que la pantalla pueda decir cuantas quedan.
	// Viaja desde config.yaml en vez de copiarse al frontend: duplicado, el dia
	// que se cambie de modelo la pantalla mentiria sin que nada avise.
	PorFotoUSD float64 `json:"por_foto_usd"`
}

type CategoriaDTO struct {
	Nombre string     `json:"nombre"`
	Platos []PlatoDTO `json:"platos"`
}

type PlatoDTO struct {
	ID          string      `json:"id"`
	Nombre      string      `json:"nombre"`
	Descripcion string      `json:"descripcion,omitempty"`
	Precios     []PrecioDTO `json:"precios"`

	// Desde es el mas barato ya formateado: es lo que va en la tarjeta cuando el
	// plato tiene varias opciones.
	Desde string `json:"desde"`

	Revisar *RevisionDTO `json:"revisar,omitempty"`
	Foto    FotoDTO      `json:"foto"`

	// FotoAjuste viaja de vuelta para que el cuadro del lapiz nazca con lo que
	// el dueno ya habia escrito. Sin esto, abrirlo por segunda vez muestra un
	// campo vacio y parece que se perdio.
	FotoAjuste string `json:"foto_ajuste,omitempty"`

	// URLs, no claves: son para pintarlas. Borrar una concreta va por el
	// endpoint de referencias, que devuelve las dos listas.
	FotoReferencias []string `json:"foto_referencias,omitempty"`

	// Ausente: la ultima lectura ya no lo trajo. Sigue aqui con su foto hasta
	// que el dueno diga.
	Ausente bool `json:"ausente,omitempty"`

	// Hoja es la clave de la hoja de la que salio, vacia si no se sabe. La
	// pantalla la necesita para contar cuantos platos se lleva por delante
	// quitar una hoja, y asi decirlo ANTES de que el dueno lo confirme.
	Hoja string `json:"hoja,omitempty"`
}

type PrecioDTO struct {
	// Etiqueta vacia con mas de un precio es justo lo que el dueno tiene que
	// rellenar: la carta traia dos montos y no decia de que era cada uno.
	Etiqueta string `json:"etiqueta,omitempty"`

	// Soles es para mostrar; Centimos es para calcular. Se mandan los dos para
	// que el frontend no tenga que formatear moneda peruana por su cuenta.
	Soles    string `json:"soles"`
	Centimos int64  `json:"centimos"`

	// Impreso es el texto tal como esta en la carta. Se manda para que la
	// pantalla de revision pueda enfrentarlo con lo que entendio el modelo.
	Impreso string `json:"impreso"`

	Manuscrito bool `json:"manuscrito,omitempty"`
}

type RevisionDTO struct {
	Motivo string `json:"motivo"`

	// Explicacion es la frase que se le ensena al dueno, no el codigo interno.
	Explicacion string `json:"explicacion"`

	// Bloquea distingue "no se puede publicar asi" de "miralo": son dos colores
	// distintos en la pantalla.
	Bloquea bool `json:"bloquea"`
}

type FotoDTO struct {
	Estado string `json:"estado"`
	Origen string `json:"origen,omitempty"`

	// URL vacia = todavia no hay foto, la tarjeta pinta el recuadro gris.
	//
	// Van las TRES y no solo la grande con una regla de sufijo en el frontend:
	// como se llama cada variante lo sabe imagen.ConVariante y nadie mas. Con la
	// regla repetida en TypeScript, el dia que cambie el sufijo el catalogo
	// pediria claves que nadie escribio, y fallaria en silencio como una imagen
	// rota.
	URL string `json:"url,omitempty"`

	// Media es para las tarjetas del editor (~200 px) y Pequena para el
	// catalogo publico (80 px). Medido: la pequena pesa 8.9 KB donde la grande
	// pesa 46, y la original que serviamos antes pesaba 594.
	URLMedia   string `json:"url_media,omitempty"`
	URLPequena string `json:"url_pequena,omitempty"`
}

type ErrorDTO struct {
	Error   string `json:"error"`
	Detalle string `json:"detalle,omitempty"`
}

// --- traduccion ---

// URLDeClave la provee quien sirve las imagenes.
type URLDeClave func(clave string) string

func aFotoDTO(f domain.Foto, url URLDeClave) FotoDTO {
	dto := FotoDTO{Estado: string(f.Estado), Origen: string(f.Origen)}
	if f.Clave == "" {
		return dto
	}
	dto.URL = url(f.Clave)
	dto.URLMedia = url(imagen.ConVariante(f.Clave, imagen.Media))
	dto.URLPequena = url(imagen.ConVariante(f.Clave, imagen.Pequena))
	return dto
}

func aImportacionDTO(imp domain.Importacion, url URLDeClave, porFoto float64) ImportacionDTO {
	dto := ImportacionDTO{
		ID:     imp.ID.String(),
		Estado: string(imp.Estado),
		Etapa:  imp.Etapa,
		Restaurante: RestauranteDTO{
			Nombre: imp.Restaurante.Nombre,
			Slug:   imp.Restaurante.Slug,
		},
		Error:  imp.Error,
		Marcas: MarcasDTO{Revisar: imp.Marcas.Revisar, Confirmar: imp.Marcas.Confirmar},
		Gasto: GastoDTO{
			GastadoUSD:     imp.Gastado.Dolares(),
			PresupuestoUSD: imp.Presupuesto.Dolares(),
			PorFotoUSD:     porFoto,
		},
		// Nunca nil: una lista vacia y una ausente se recorren igual en el
		// frontend, y un null obliga a comprobar antes de mapear.
		Categorias:      make([]CategoriaDTO, 0, len(imp.Carta.Categorias)),
		Paginas:         make([]PaginaDTO, 0, len(imp.Imagenes)),
		PuedePublicarse: imp.PuedePublicarse(),
	}

	for _, clave := range imp.Imagenes {
		dto.Paginas = append(dto.Paginas, PaginaDTO{Clave: clave, URL: url(clave)})
	}

	for _, cat := range imp.Carta.Categorias {
		c := CategoriaDTO{Nombre: cat.Nombre, Platos: make([]PlatoDTO, 0, len(cat.Platos))}
		for _, p := range cat.Platos {
			c.Platos = append(c.Platos, aPlatoDTO(p, url))
		}
		dto.Categorias = append(dto.Categorias, c)
	}
	return dto
}

func aPlatoDTO(p domain.Plato, url URLDeClave) PlatoDTO {
	dto := PlatoDTO{
		ID:              p.ID.String(),
		Nombre:          p.Nombre,
		Descripcion:     p.Descripcion,
		Precios:         make([]PrecioDTO, 0, len(p.Precios)),
		Desde:           p.Desde().String(),
		FotoAjuste:      p.FotoAjuste,
		FotoReferencias: urls(p.FotoReferencias, url),
		Ausente:         p.Ausente,
		Hoja:            p.Hoja,
		Foto:            aFotoDTO(p.Foto, url),
	}

	for _, pr := range p.Precios {
		dto.Precios = append(dto.Precios, PrecioDTO{
			Etiqueta:   pr.Etiqueta,
			Soles:      pr.Centimos.String(),
			Centimos:   int64(pr.Centimos),
			Impreso:    pr.Texto,
			Manuscrito: pr.Procedencia == domain.Manuscrito,
		})
	}

	if p.Revisar != domain.SinRevision {
		dto.Revisar = &RevisionDTO{
			Motivo:      string(p.Revisar),
			Explicacion: p.Revisar.Explicacion(),
			Bloquea:     p.Revisar.Nivel() == domain.NivelRevisar,
		}
	}
	return dto
}

func parsearID(s string) (id.ID, bool) { return id.Parsear(s) }
