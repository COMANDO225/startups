package domain

import (
	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
)

// Estado de una importacion. Cada uno tiene que tener quien lo saque de ahi, o
// es un estado sin salida — que es un bug, no un estado.
//
//	nueva     -> leyendo             lo mueve el dueno al subir sus hojas
//	leyendo   -> lista | fallida     lo mueve el job de lectura
//	lista     -> publicada           lo mueve el dueno desde la pantalla
//	fallida   -> (final)             se sube la carta de nuevo, es otra importacion
//	publicada -> (final)             re-publicar mueve el puntero del restaurante
type Estado string

const (
	// Nueva es el restaurante creado y sin carta todavia: ya tiene nombre, tipo
	// de negocio y token, o sea que lo que el dueno escribio en el primer paso
	// esta guardado antes de pedirle una sola foto.
	Nueva     Estado = "nueva"
	Leyendo   Estado = "leyendo"
	Lista     Estado = "lista"
	Publicada Estado = "publicada"
	Fallida   Estado = "fallida"
)

// Importacion es una subida de carta: la unidad de trabajo y la unidad de
// presupuesto.
type Importacion struct {
	ID          id.ID
	Restaurante Restaurante
	Estado      Estado

	// Etapa solo tiene sentido con Estado == Leyendo.
	Etapa int16

	// Marcas es cuantos platos quedaron en cada nivel tras la verificacion.
	// Se guarda calculado para que la pantalla no tenga que recorrer la carta
	// entera solo para pintar un contador.
	Marcas Marcas

	// Presupuesto es el tope; Reservado es lo comprometido ANTES de gastar y es
	// lo que compara el guard; Gastado es lo que costo DESPUES. Reservado y
	// Gastado no son redundantes: entre uno y otro esta la llamada en vuelo.
	Presupuesto dinero.MicrosUSD
	Reservado   dinero.MicrosUSD
	Gastado     dinero.MicrosUSD

	// Imagenes son las CLAVES de las paginas de la carta, en el orden en que el
	// dueno las ordeno. La URL la arma quien las sirve.
	Imagenes []string

	// Error vacio significa que no hubo. Solo tiene sentido con Estado Fallida.
	Error string

	// Carta se llena al leerla; en Leyendo viene vacia.
	Carta Carta
}

// Restaurante es de quien es la carta.
type Restaurante struct {
	ID     id.ID
	Nombre string

	// Slug vacio = todavia no se publico. Es lo que va en la URL publica.
	Slug string
}

// PuedePublicarse dice si la carta esta lista para salir a la calle.
//
// Bloquea NivelRevisar y NO NivelConfirmar, que es toda la razon de que existan
// dos niveles: un precio que no cuadra no puede publicarse, y un precio
// manuscrito que SI cuadra solo hay que mirarlo. Tratarlos igual deja media
// carta en rojo y entrena al dueno a aprobar sin leer.
// Las etapas de la lectura, EN EL ORDEN EN QUE EL WORKER LAS EMITE. El numero es
// el contrato con la pantalla; los textos son de la pantalla y no viven aqui.
//
// El orden es el contrato entero: la pantalla marca como hechas todas las
// anteriores al numero que recibe. Cuando clasificar salia despues de guardar y
// su constante estaba antes, la lista escribia 0,1,2,4,5,3 — o sea que saltaba
// una etapa hacia adelante, la daba por hecha sin haberla hecho, y luego volvia
// atras. Si se mueve una llamada en el worker, se mueve su constante aqui.
const (
	EtapaArchivos int16 = iota
	EtapaLeyendo
	EtapaCruzandoPrecios
	EtapaOrdenando
	EtapaConociendo
	EtapaClasificando
	EtapaGuardando
	EtapaTerminada
)

// Lista blanca y no lista negra: con "todo menos leyendo y fallida", el estado
// nuevo que se anadiera manana quedaria publicable sin que nadie lo decidiera.
// Fue exactamente lo que iba a pasar con 'nueva', que es un restaurante sin una
// sola hoja de carta.
func (i Importacion) PuedePublicarse() bool {
	if i.Estado != Lista && i.Estado != Publicada {
		return false
	}
	return len(i.Carta.ParaRevisar()) == 0
}

// PresupuestoDisponible es lo que queda por comprometer.
func (i Importacion) PresupuestoDisponible() dinero.MicrosUSD {
	if i.Reservado >= i.Presupuesto {
		return 0
	}
	return i.Presupuesto - i.Reservado
}
