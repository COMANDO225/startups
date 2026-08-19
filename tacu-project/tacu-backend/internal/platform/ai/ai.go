// Package ai es la puerta unica hacia los proveedores de modelos.
//
// La aplicacion NUNCA elige un modelo: elige una TAREA. El config dice que
// modelos atienden esa tarea y en que orden. Cambiar de modelo, de proveedor o
// de orden es editar YAML, no tocar codigo.
package ai

import (
	"context"
	"errors"
	"fmt"
)

// Capacidad es lo que un proveedor sabe hacer.
type Capacidad string

const (
	Vision        Capacidad = "vision"         // imagen(es) -> struct tipado
	Texto         Capacidad = "texto"          // texto -> struct tipado
	GenerarImagen Capacidad = "generar_imagen" // prompt -> imagen
)

// Imagen es una imagen de entrada o de salida.
type Imagen struct {
	Bytes []byte
	MIME  string // image/jpeg, image/png, image/webp, application/pdf

	// Densidad es cuantos tokens se le dedican a esta imagen.
	//
	// En Gemini 3 el presupuesto de tokens de una imagen es FIJO por item, no
	// proporcional a su tamano: baja 280, media 560, alta 1120, maxima 2240.
	// Una foto de 1600x1200 y un recorte de 400x300 cuestan lo mismo.
	//
	// Por eso recortar una carta en secciones NO ahorra: 8 recortes cuestan 8x
	// los tokens de imagen, no 1/8. Lo que el recorte compra es DENSIDAD
	// (1600x1200 a 1120 tokens son 1714 px por token; un recorte de seccion a
	// los mismos 1120 tokens son 107, o sea 16x mas resolucion efectiva).
	//
	// MEDIDO en galponcito.jpeg con gemini-3.7-flash (cmd/cartabench):
	//
	//	densidad   tok.entrada   tok.salida   exactitud     costo
	//	baja             865        ~2850        97.6%     $0.0114
	//	alta            1663        ~2850     2 de 3 al 100%   $0.0128
	//	maxima          2759        ~2870     2 de 3 al 100%   $0.0128
	//
	// Subir de alta a maxima NO cambio nada: mismos aciertos, mismos fallos.
	// La densidad ya no era el cuello de botella en esta carta.
	//
	// Y el costo casi no se mueve porque la entrada es ~1/3 de los tokens y
	// vale 5x menos que la salida: 1096 tokens de imagen de mas son $0.0008.
	// Quien quiera abaratar la extraccion tiene que reducir la SALIDA, no la
	// entrada. Ahi esta el 85% de la factura.
	Densidad Densidad
}

// Densidad controla cuantos tokens se le dedican a una imagen.
type Densidad string

const (
	DensidadPorDefecto Densidad = ""       // lo que decida el proveedor
	DensidadBaja       Densidad = "baja"   // ~280 tokens
	DensidadMedia      Densidad = "media"  // ~560
	DensidadAlta       Densidad = "alta"   // ~1120
	DensidadMaxima     Densidad = "maxima" // ~2240
)

// Peticion es lo que la aplicacion pide.
type Peticion struct {
	Tarea      Tarea
	Sistema    string // instrucciones de rol
	Prompt     string // la instruccion concreta
	Imagenes   []Imagen
	SchemaJSON []byte // JSON Schema de la respuesta esperada
	MaxTokens  int

	// Referencias son imagenes de estilo para generacion. gpt-image-2 admite
	// hasta 16 y las procesa siempre en alta fidelidad; es lo que hace que 60
	// fotos de platos parezcan del mismo restaurante.
	Referencias []Imagen

	// Tamano, Proporcion y Calidad solo aplican a generacion de imagenes.
	//
	// Cada proveedor los expresa distinto y NO se traducen aqui: OpenAI pide
	// "1536x1024" y una calidad; Gemini pide "1K"/"2K"/"4K" y una proporcion
	// aparte. Traducir uno al otro obliga a inventar equivalencias que ninguno
	// de los dos garantiza.
	Tamano     string // openai: "1536x1024" · gemini: "1K" | "2K" | "4K"
	Proporcion string // gemini: "1:1" | "4:3" | "16:9" ... (openai lo ignora)
	Calidad    string // openai: low | medium | high (gemini lo ignora)
}

// Uso es lo que costo una llamada. Se registra SIEMPRE: el costo por carta
// importada es el unit economics del producto, y sin medirlo no se le puede
// poner precio a un plan.
type Uso struct {
	Tarea         Tarea
	Modelo        string // "gemini/gemini-3.7-flash"
	TokensEntrada int
	TokensSalida  int
	Imagenes      int // imagenes generadas, para modelos que cobran por imagen
	CostoUSD      float64
	Intentos      int
}

func (u Uso) String() string {
	if u.Imagenes > 0 {
		return fmt.Sprintf("%s [%s]: %d imagenes = $%.5f", u.Tarea, u.Modelo, u.Imagenes, u.CostoUSD)
	}
	return fmt.Sprintf("%s [%s]: %d entrada + %d salida = $%.5f (intentos: %d)",
		u.Tarea, u.Modelo, u.TokensEntrada, u.TokensSalida, u.CostoUSD, u.Intentos)
}

// Respuesta es el resultado crudo mas su costo.
type Respuesta struct {
	JSON     []byte   // para tareas que devuelven struct
	Imagenes []Imagen // para tareas de generacion
	Uso      Uso

	// PromptReescrito es lo que el proveedor REALMENTE uso, cuando lo informa.
	// gpt-image-2 reescribe el prompt antes de generar; sin esto, depurar una
	// foto mala es adivinar si el problema es lo que escribimos o lo que el
	// proveedor entendio.
	PromptReescrito string
}

// Proveedor es lo que implementa cada backend concreto.
//
// Recibe el nombre del modelo en cada llamada: un mismo proveedor atiende varias
// tareas con modelos distintos, y quien elige es el config.
type Proveedor interface {
	Nombre() string
	Soporta(Capacidad) bool
	Ejecutar(ctx context.Context, modelo string, p Peticion) (Respuesta, error)
}

// --- errores ---

// ErrProveedor distingue lo que vale la pena reintentar en otro modelo de lo que
// no tiene arreglo en ninguno.
type ErrProveedor struct {
	Proveedor string
	Codigo    string
	Mensaje   string
	Terminal  bool
	Causa     error
}

func (e *ErrProveedor) Error() string {
	return fmt.Sprintf("%s [%s]: %s", e.Proveedor, e.Codigo, e.Mensaje)
}
func (e *ErrProveedor) Unwrap() error { return e.Causa }

// Transitorio: vale la pena intentar con el siguiente modelo de la cadena.
func Transitorio(proveedor, codigo, mensaje string, causa error) *ErrProveedor {
	return &ErrProveedor{Proveedor: proveedor, Codigo: codigo, Mensaje: mensaje, Causa: causa}
}

// Terminal: el problema es la peticion, no el modelo. Reintentar solo gasta.
func Terminal(proveedor, codigo, mensaje string, causa error) *ErrProveedor {
	return &ErrProveedor{Proveedor: proveedor, Codigo: codigo, Mensaje: mensaje, Terminal: true, Causa: causa}
}

func EsTerminal(err error) bool {
	var e *ErrProveedor
	return errors.As(err, &e) && e.Terminal
}

var (
	ErrSinProveedor = errors.New("proveedor no registrado")
	ErrPresupuesto  = errors.New("presupuesto de la operacion agotado")
)

// --- precios ---

// Precio de un modelo. Por tokens, por imagen, o ambos.
type Precio struct {
	EntradaPorMillon float64
	SalidaPorMillon  float64
	PorImagen        float64
}

// Precios es la tabla completa, indexada por "proveedor/modelo". Vive en config
// para que un cambio de tarifa no sea un despliegue.
type Precios map[string]Precio

func (t Precios) Costo(m Modelo, u Uso) float64 {
	p, ok := t[m.String()]
	if !ok {
		return 0
	}
	return float64(u.TokensEntrada)/1e6*p.EntradaPorMillon +
		float64(u.TokensSalida)/1e6*p.SalidaPorMillon +
		float64(u.Imagenes)*p.PorImagen
}
