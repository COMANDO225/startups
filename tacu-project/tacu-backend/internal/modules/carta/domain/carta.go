// Package domain modela la carta de un restaurante.
package domain

import (
	"strings"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
)

// MotivoRevision dice por que un plato necesita ojo humano antes de publicarse.
// Vacio significa que paso todas las verificaciones.
type MotivoRevision string

const (
	SinRevision        MotivoRevision = ""
	PrecioIlegible     MotivoRevision = "precio_ilegible"
	PrecioDiscordante  MotivoRevision = "precio_discordante"
	PrecioAusente      MotivoRevision = "precio_ausente"
	NombreVacio        MotivoRevision = "nombre_vacio"
	PrecioManuscrito   MotivoRevision = "precio_manuscrito"
	VariantesSinNombre MotivoRevision = "variantes_sin_nombre"
)

// Nivel separa "esto no deberia publicarse" de "esto conviene que lo mires".
//
// Sin la distincion, Galponcito marca 19 de 42 platos y las 19 marcas dicen que
// todo cuadra: es una carta con la mitad en amarillo donde ningun aviso senala
// nada roto. Eso entrena al dueno a aprobar sin leer, que es exactamente cuando
// se cuela el precio que si estaba mal.
type Nivel int

const (
	NivelNinguno   Nivel = iota
	NivelConfirmar       // nada esta roto, pero el dueno lo tiene que mirar
	NivelRevisar         // hay algo que no cuadra: no se publica asi
)

func (m MotivoRevision) Nivel() Nivel {
	switch m {
	case SinRevision:
		return NivelNinguno
	case PrecioManuscrito:
		// Los dos testigos coinciden. Nada indica error: es una correccion a
		// mano, y el dueno la confirma de un vistazo.
		return NivelConfirmar
	}
	return NivelRevisar
}

func (m MotivoRevision) Explicacion() string {
	switch m {
	case PrecioIlegible:
		return "no se pudo leer el precio impreso"
	case PrecioDiscordante:
		return "el precio leido no coincide con el texto impreso"
	case PrecioAusente:
		return "el plato no tiene precio"
	case NombreVacio:
		return "el plato no tiene nombre"
	case PrecioManuscrito:
		return "el precio esta corregido a mano sobre el impreso"
	case VariantesSinNombre:
		return "tiene varios precios y la carta no dice de que es cada uno"
	}
	return ""
}

// Procedencia dice de donde salio el precio.
type Procedencia string

const (
	Impreso    Procedencia = "impreso"
	Manuscrito Procedencia = "manuscrito"
	SinPrecio  Procedencia = "ninguno"
)

// Precio es UNA de las formas de pedir un plato.
//
// Texto y Centimos existen por separado A PROPOSITO. El modelo de vision
// devuelve ambos: el texto EXACTO como esta impreso, y su interpretacion
// numerica. Nosotros parseamos el texto por nuestra cuenta y comparamos.
//
// Hace falta porque los VLM aciertan ~67% en valores y, peor, CORRIGEN en
// silencio lo que perciben como errores de formato: producen salidas
// "linguisticamente plausibles pero factualmente inconsistentes con la imagen".
// El JSON Schema valida la FORMA, no los valores: el JSON sale perfecto y el
// precio esta mal, sin ninguna senal.
type Precio struct {
	// Etiqueta es como la carta llama a esta opcion: "personal", "fuente",
	// "1/2 doc.". Vacia cuando la carta pone varios precios sin decir de que
	// son — pasa, y no se adivina: lo llena el dueno, que si lo sabe.
	Etiqueta string `json:"etiqueta,omitempty"`

	Texto    string          `json:"texto"`
	Centimos dinero.Centimos `json:"centimos"`

	// Procedencia distingue el precio impreso del corregido a mano. Un sticker
	// pegado encima es el precio VIGENTE; el impreso de abajo es el viejo.
	// Publicar el viejo le hace perder plata al restaurante.
	Procedencia Procedencia `json:"procedencia,omitempty"`

	// AnuladoTexto es el precio impreso que el sticker tapa, cuando se alcanza
	// a ver. Sirve para mostrarle al dueno "antes decia X, ahora Y" en la
	// pantalla de confirmacion.
	AnuladoTexto string `json:"anulado_texto,omitempty"`
}

// Plato es un item de la carta.
//
// Precios es una lista porque un mismo plato se vende en mas de un tamano y la
// carta lo escribe en UNA fila con dos montos:
//
//	Ceviche + Arroz c/ Mariscos + Chicharron Mixto    S/ 45   S/ 80
//
// Partir eso en dos platos con el mismo nombre le deja al cliente dos filas
// identicas a distinto precio y ninguna forma de elegir. Es un plato con dos
// formas de pedirlo, y en la web son dos pestanas.
type Plato struct {
	// ID vacio mientras el plato solo existe en la respuesta del modelo. Se
	// asigna al guardarlo, y a partir de ahi es lo que permite editarlo y
	// colgarle una foto sin depender de su posicion — que cambia en cuanto
	// OrganizarCarta reordena la carta.
	ID id.ID `json:"id,omitempty"`

	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion,omitempty"`

	// Categoria viaja con el plato, duplicada respecto de Carta.Categorias, y
	// hace falta: la foto la necesita. "Ceviche + Arroz c/ Mariscos + Chicharron
	// Mixto" no dice en ningun sitio que sea un trio de tres compartimentos —lo
	// dice la cabecera de su seccion— y el worker recibe un Plato suelto, no la
	// carta entera. Ver domain.FormatoDePlato.
	Categoria string `json:"categoria,omitempty"`

	// Tipico es la clave del banco de platos con la que se emparejo este nombre
	// impreso, y TipicoOrigen quien lo decidio (regla|ia|dueno). Se resuelve al
	// leer la carta y se guarda: emparejar en cada foto repetiria el trabajo y
	// dejaria que el resultado cambiara solo al ampliar el banco.
	Tipico       string `json:"tipico,omitempty"`
	TipicoOrigen string `json:"tipico_origen,omitempty"`

	// Ausente marca el plato que estaba y la ultima lectura ya no trajo. No se
	// borra solo: puede tener una foto pagada detras y la lectura pudo
	// equivocarse, asi que lo confirma el dueno.
	Ausente bool `json:"ausente,omitempty"`

	// HojaLeida es en cual de las imagenes dijo el modelo que esta escrito este
	// plato, 1..N. Es lo unico que el modelo puede saber: no conoce nuestras
	// claves de almacen.
	HojaLeida int `json:"hoja,omitempty"`

	// Hoja es la CLAVE de esa hoja en el almacen. La resuelve el worker a
	// partir de HojaLeida, y no viaja en la carta cruda porque no es parte de
	// lo que contesto el modelo.
	//
	// Sirve para acotar una relectura a una hoja y para poder quitar una hoja
	// con sus platos sin tocar los de las demas.
	Hoja string `json:"-"`

	Precios []Precio `json:"precios"`

	Revisar MotivoRevision `json:"revisar,omitempty"`

	Foto Foto `json:"foto"`

	// FotoAjuste es lo que el dueno escribio para corregir la foto de ESTE
	// plato: "va con mas cancha", "el nuestro lleva yuca frita, no papa".
	//
	// Se SUMA al conocimiento compartido, nunca lo reemplaza. Un cuadro de texto
	// que sustituyera el prompt entero le dejaria borrar sin querer la receta
	// fotografica —encuadre, luz, fondo, lo prohibido— y con ella volverian los
	// fallos que costaron cinco rondas arreglar. Ver app.PromptFoto.
	FotoAjuste string `json:"foto_ajuste,omitempty"`

	// FotoReferencias son las fotos de ejemplo que subio el dueno para ESTE
	// plato: "asi se ve el nuestro". Son CLAVES del almacen, nunca URLs ni
	// bytes; quien genera las carga.
	//
	// Con una referencia el texto deja de hacer falta: la foto dice mas que
	// cualquier descripcion que el dueno pueda escribir. Se admiten las dos
	// cosas a la vez.
	FotoReferencias []string `json:"foto_referencias,omitempty"`
}

// EstadoFoto es en que punto esta la foto de un plato. Los seis son visualmente
// distintos en la pantalla y todos hacen falta.
type EstadoFoto string

const (
	SinFoto        EstadoFoto = "vacia"           // el recuadro gris con "subir imagen"
	FotoPendiente  EstadoFoto = "pendiente"       // encolada, el shimmer
	FotoGenerando  EstadoFoto = "generando"       // el worker la tiene
	FotoLista      EstadoFoto = "lista"           //
	FotoConError   EstadoFoto = "error"           // con su boton de reintentar
	FotoSinCredito EstadoFoto = "sin_presupuesto" // se acabo el presupuesto de esta carta
)

// OrigenFoto distingue la que genero la IA de la que subio el dueno. Importa
// porque "generar todas" NUNCA pisa una foto propia.
type OrigenFoto string

const (
	FotoDeNadie OrigenFoto = ""
	FotoDeIA    OrigenFoto = "ia"
	FotoPropia  OrigenFoto = "propia"
)

// Foto es el estado de la imagen de un plato.
type Foto struct {
	Estado EstadoFoto `json:"estado"`
	Origen OrigenFoto `json:"origen,omitempty"`

	// Clave es la ruta en el almacen, NUNCA una URL. La URL la arma quien sirve
	// la pagina; guardarla aqui convertiria mudarse a otro almacen en un UPDATE
	// masivo sobre datos vivos.
	Clave string `json:"-"`

	Intentos int `json:"-"`
}

// Generable dice si tiene sentido pedirle una foto a la IA para este plato.
//
// Una foto que subio el dueno no se pisa nunca: es mejor que cualquier cosa que
// generemos, y sobrescribirla seria destruir trabajo suyo.
func (p Plato) Generable() bool {
	if p.Foto.Origen == FotoPropia {
		return false
	}
	return p.Foto.Estado == SinFoto || p.Foto.Estado == FotoConError
}

// NecesitaRevision indica si este plato tiene algo que NO cuadra.
func (p Plato) NecesitaRevision() bool { return p.Revisar.Nivel() == NivelRevisar }

// NecesitaConfirmacion indica que no hay nada roto pero el dueno lo tiene que
// mirar antes de publicar.
func (p Plato) NecesitaConfirmacion() bool { return p.Revisar.Nivel() == NivelConfirmar }

// Desde devuelve el precio mas barato, que es el que se muestra en la tarjeta
// cuando el plato tiene varias opciones. Cero si no tiene ninguno.
func (p Plato) Desde() dinero.Centimos {
	if len(p.Precios) == 0 {
		return 0
	}
	min := p.Precios[0].Centimos
	for _, pr := range p.Precios[1:] {
		if pr.Centimos < min {
			min = pr.Centimos
		}
	}
	return min
}

type Categoria struct {
	Nombre string  `json:"nombre"`
	Platos []Plato `json:"platos"`
}

type Carta struct {
	Categorias []Categoria `json:"categorias"`
}

// Marcas es cuantos platos quedaron en cada nivel.
type Marcas struct {
	Revisar   int
	Confirmar int
}

func (m Marcas) Total() int { return m.Revisar + m.Confirmar }

// Verificar cruza lo que dijo el modelo contra lo que nosotros leemos del texto
// impreso, y marca cada plato que no cuadre.
//
// NO corrige ni descarta: marca. La decision es del dueno, en pantalla.
func (c *Carta) Verificar() Marcas {
	var m Marcas
	for i := range c.Categorias {
		for j := range c.Categorias[i].Platos {
			p := &c.Categorias[i].Platos[j]
			p.Revisar = verificarPlato(*p)
			switch p.Revisar.Nivel() {
			case NivelRevisar:
				m.Revisar++
			case NivelConfirmar:
				m.Confirmar++
			}
		}
	}
	return m
}

// Verificar revisa UN plato y actualiza su marca.
//
// Existe aparte de Carta.Verificar porque el dueno corrige de a un plato: le
// pone nombre a los dos precios de un trio y el aviso rojo tiene que apagarse
// solo, sin volver a verificar los otros 73 ni volver a leer la carta.
func (p *Plato) Verificar() { p.Revisar = verificarPlato(*p) }

func verificarPlato(p Plato) MotivoRevision {
	if strings.TrimSpace(p.Nombre) == "" {
		return NombreVacio
	}
	if len(p.Precios) == 0 {
		return PrecioAusente
	}

	// El motivo mas grave gana: primero lo que huele a dato falso, al final lo
	// que solo pide confirmacion.
	peor := SinRevision
	for _, pr := range p.Precios {
		if m := verificarPrecio(pr); gravedad(m) > gravedad(peor) {
			peor = m
		}
	}
	if peor != SinRevision {
		return peor
	}

	// Varios precios sin etiqueta: cada uno cuadra por separado, pero juntos no
	// se pueden publicar. El cliente veria dos montos sin saber cual pedir, y
	// el cruce de precios no puede verlo porque mira un precio a la vez.
	if len(p.Precios) > 1 && algunaSinEtiqueta(p.Precios) {
		return VariantesSinNombre
	}

	return SinRevision
}

func verificarPrecio(pr Precio) MotivoRevision {
	if strings.TrimSpace(pr.Texto) == "" {
		// Sin texto impreso no hay contra que contrastar. Si ademas el modelo no
		// dio numero, el plato simplemente no tiene precio en la carta (pasa:
		// "precio segun mercado"). Si dio numero, se lo invento.
		if pr.Centimos == 0 {
			return PrecioAusente
		}
		return PrecioDiscordante
	}

	leido, err := dinero.Parsear(pr.Texto)
	if err != nil {
		// El texto existe pero es ambiguo ("12 a 15", "desde 20"). No podemos
		// afirmar que el numero del modelo este mal, pero tampoco confirmarlo.
		return PrecioIlegible
	}
	if leido != pr.Centimos {
		return PrecioDiscordante
	}

	// Un precio manuscrito es una correccion pegada encima del impreso: es el
	// vigente, pero tambien el de lectura mas fragil (letra a mano, sticker
	// torcido, el viejo asomando por debajo). Va a confirmacion siempre.
	if pr.Procedencia == Manuscrito {
		return PrecioManuscrito
	}
	return SinRevision
}

// gravedad ordena los motivos: lo que puede ser un dato falso pesa mas que lo
// que solo hay que confirmar.
func gravedad(m MotivoRevision) int {
	switch m {
	case PrecioDiscordante:
		return 4
	case PrecioIlegible:
		return 3
	case PrecioAusente:
		return 2
	case PrecioManuscrito:
		return 1
	}
	return 0
}

// algunaSinEtiqueta dice si queda algun precio sin nombre.
//
// ALGUNA, no todas. La primera version pedia que estuvieran las dos vacias para
// marcar, y con eso un plato con "Personal S/ 45" y un "S/ 80" pelado se daba
// por bueno: el cliente ve dos montos y solo sabe que es uno. Ademas la pantalla
// ya marcaba con "alguna", asi que el aviso rojo del navegador y el recuento del
// servidor discrepaban, y publicar se desbloqueaba con la carta a medio nombrar.
func algunaSinEtiqueta(precios []Precio) bool {
	for _, pr := range precios {
		if strings.TrimSpace(pr.Etiqueta) == "" {
			return true
		}
	}
	return false
}

// Platos devuelve todos los platos de todas las categorias, en orden.
func (c Carta) Platos() []Plato {
	var todos []Plato
	for _, cat := range c.Categorias {
		todos = append(todos, cat.Platos...)
	}
	return todos
}

// ParaRevisar devuelve los platos con algo que no cuadra. Son los que hay que
// mirar de verdad, y la lista tiene que ser corta para que se lea.
func (c Carta) ParaRevisar() []Plato {
	return filtrar(c.Platos(), Plato.NecesitaRevision)
}

// ParaConfirmar devuelve los que no tienen nada roto pero conviene mirar.
func (c Carta) ParaConfirmar() []Plato {
	return filtrar(c.Platos(), Plato.NecesitaConfirmacion)
}

func filtrar(platos []Plato, cumple func(Plato) bool) []Plato {
	var out []Plato
	for _, p := range platos {
		if cumple(p) {
			out = append(out, p)
		}
	}
	return out
}
