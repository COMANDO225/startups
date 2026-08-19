// Package app contiene los casos de uso de la carta.
package app

import (
	"context"
	"encoding/json"
	"fmt"

	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// schemaCarta es el contrato de salida del modelo.
//
// "texto" y "centimos" NO son redundantes: son los dos testigos que se cruzan
// despues. El schema garantiza la FORMA de la respuesta, no que los valores
// sean correctos — por eso hace falta la verificacion en Go.
const schemaCarta = `{
  "type": "object",
  "required": ["categorias"],
  "additionalProperties": false,
  "properties": {
    "categorias": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["nombre", "platos"],
        "additionalProperties": false,
        "properties": {
          "nombre": {
            "type": "string",
            "description": "Nombre de la seccion tal como aparece: Entradas, Segundos, Bebidas, Criollos..."
          },
          "platos": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["nombre", "precios", "hoja"],
              "additionalProperties": false,
              "properties": {
                "nombre": {
                  "type": "string",
                  "description": "Nombre del plato, transcrito literalmente"
                },
                "descripcion": {
                  "type": "string",
                  "description": "Lo que el plato INCLUYE o su descripcion, tal como lo lista la carta. Para un combo, la lista completa de lo que trae: 'Pollo + papas + ensalada + gaseosa 1.5 L'. Para una promocion, sus condiciones: 'Martes y Jueves'. Vacio solo si la carta no dice nada."
                },
                "hoja": {
                  "type": "integer",
                  "minimum": 1,
                  "description": "En cual de las imagenes que recibiste esta escrito este plato. La primera es 1. Si una seccion continua en la siguiente hoja, cada plato lleva el numero de la hoja donde esta ESCRITO, no el de la hoja donde empieza su titulo."
                },
                "precios": {
                  "type": "array",
                  "minItems": 1,
                  "description": "Las formas de pedir este plato. Casi siempre una sola. Varias cuando la MISMA fila muestra mas de un monto (tamanos distintos).",
                  "items": {
                    "type": "object",
                    "required": ["texto", "centimos", "procedencia"],
                    "additionalProperties": false,
                    "properties": {
                      "etiqueta": {
                        "type": "string",
                        "description": "Como llama la carta a esta opcion: 'personal', 'fuente', 'jarra', '1/2 doc.'. SOLO si esta impreso. Si la carta pone dos montos sin decir de que son, cadena vacia. NO adivines 'personal' ni 'familiar'."
                      },
                      "texto": {
                        "type": "string",
                        "description": "El precio EXACTAMENTE como esta impreso, caracter por caracter, incluyendo simbolo de moneda, puntos, comas y espacios. Ejemplos: 'S/ 12.50', '25', 'S/45'. Si el plato no tiene precio impreso, cadena vacia. NO normalizar, NO corregir, NO completar decimales."
                      },
                      "centimos": {
                        "type": "integer",
                        "description": "El mismo precio convertido a centimos de sol. 'S/ 12.50' -> 1250. '25' -> 2500. Si no hay precio impreso, 0."
                      },
                      "procedencia": {
                        "type": "string",
                        "enum": ["impreso", "manuscrito", "ninguno"],
                        "description": "De donde salio el precio. 'manuscrito' si viene de un sticker, etiqueta o correccion escrita a mano PEGADA ENCIMA del precio original. 'impreso' si es parte del diseno de la carta. 'ninguno' si el plato no tiene precio."
                      },
                      "anulado_texto": {
                        "type": "string",
                        "description": "Si un precio manuscrito tapa a uno impreso y alcanzas a ver el de abajo, escribe aqui el impreso ANULADO. Cadena vacia si no hay o no se ve. NUNCA pongas este valor en 'texto'."
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

const sistemaCarta = `Transcribes cartas de restaurantes peruanos a datos estructurados.

Tu unica funcion es TRANSCRIBIR lo que ves. No interpretas, no mejoras, no
corriges y no completas.`

const promptCarta = `Transcribe esta carta de restaurante.

REGLAS SOBRE LOS PRECIOS — son lo mas importante de esta tarea:

1. En "texto" copia el precio EXACTAMENTE como esta impreso, caracter por
   caracter. Si dice "25", escribe "25", NO "S/ 25.00". Si dice "S/12,50",
   escribe "S/12,50". Incluye el simbolo de moneda solo si esta impreso.

2. NO corrijas lo que te parezca un error de formato. Si un precio se ve raro,
   incompleto o inconsistente con los demas, transcribelo TAL CUAL. Un precio
   que parece equivocado es informacion valiosa; uno que tu "arreglaste" es un
   dato falso que nadie va a poder detectar despues.

3. PRECIOS CORREGIDOS A MANO. Si hay un sticker, etiqueta o numero escrito a
   mano PEGADO ENCIMA de un precio impreso, el manuscrito ES EL PRECIO VIGENTE:
   va en "texto" con procedencia "manuscrito". El impreso que quedo debajo va
   en "anulado_texto", nunca en "texto".
   Publicar el precio viejo le hace perder plata al restaurante.

4. Si no puedes leer un precio con seguridad, deja "texto" vacio y "centimos"
   en 0. NO adivines. Es preferible un hueco a un numero inventado.

5. "centimos" es tu conversion de ese texto a centimos. Si el texto esta
   vacio, va 0.

6. UN PLATO CON VARIOS MONTOS EN LA MISMA FILA ES UN SOLO PLATO. Si la carta
   escribe

       Ceviche + Arroz c/ Mariscos + Chicharron Mixto    S/ 45   S/ 80

   eso es UN plato con DOS precios, no dos platos con el mismo nombre. Ponlos
   como dos entradas de "precios". Duplicar el plato le deja al cliente dos
   filas identicas y ninguna forma de elegir.

   La "etiqueta" de cada precio SOLO se llena si la carta la imprime
   ("personal", "fuente", "jarra", "vaso", "1/2 doc."). Si la carta pone los
   dos montos sin decir de que son, dejala VACIA. No adivines "personal" ni
   "familiar": nosotros le preguntamos al dueno, que si lo sabe. Una etiqueta
   inventada es peor que ninguna porque nadie la va a poder desmentir.

SOBRE EL RESTO:
- Respeta las secciones de la carta como categorias.
- Transcribe los nombres de los platos literalmente, sin corregir ortografia, y
  con lo que la carta les ponga entre parentesis al lado: "Limonada (jarra)",
  "Tequenos de Queso (1/2 doc.)", "Infusiones (te, anis o manzanilla)". Eso es
  parte del nombre, no una descripcion aparte. La regla 6 y las etiquetas son
  para OTRA cosa: una sola fila con VARIOS montos.
- EL NOMBRE ES EL TITULO DESTACADO, no la lista de lo que trae. Si un aviso dice
  "2 X 1" en letra grande y debajo, en letra chica, "2 pollos + papas +
  ensalada", el nombre es "2 X 1" y lo de abajo es la descripcion. El titulo es
  como el restaurante llama a esa promocion y es lo que el cliente reconoce.
- LOS COMBOS SIN SU CONTENIDO NO SIRVEN. Nadie compra "COMBO 1": compra lo que
  trae. Si la carta lista lo que incluye un combo o un plato, ESO va en
  "descripcion", completo. Lo mismo con las condiciones de una promocion
  ("Martes y Jueves"). No lo inventes, pero no lo dejes fuera si esta impreso.
- Si un plato aparece en DOS FILAS distintas de la carta a distinto precio (por
  ejemplo en "En mesa" y en "Para llevar"), esos si son dos platos distintos.
  Lo de la regla 6 aplica solo cuando los montos comparten fila.
- Ignora telefonos, direcciones, horarios y publicidad.`

// Leer convierte una o varias imagenes de una carta en una Carta verificada.
type Leer struct {
	ia *ai.Cliente
}

func NuevoLeer(ia *ai.Cliente) *Leer { return &Leer{ia: ia} }

// Resultado es la carta mas lo que costo obtenerla y cuantos platos quedaron
// marcados, separados por nivel: lo que no cuadra y lo que solo hay que mirar.
type Resultado struct {
	Carta  domain.Carta
	Uso    ai.Uso
	Marcas domain.Marcas
}

func (uc *Leer) Ejecutar(ctx context.Context, imagenes []ai.Imagen) (*Resultado, error) {
	if len(imagenes) == 0 {
		return nil, fmt.Errorf("no se recibio ninguna imagen")
	}

	resp, err := uc.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:      ai.LeerCarta,
		Sistema:    sistemaCarta,
		Prompt:     promptCarta,
		Imagenes:   imagenes,
		SchemaJSON: []byte(schemaCarta),
		MaxTokens:  32000,
	})
	if err != nil {
		return nil, fmt.Errorf("leyendo la carta: %w", err)
	}

	var carta domain.Carta
	if err := json.Unmarshal(resp.JSON, &carta); err != nil {
		return nil, fmt.Errorf("la respuesta del modelo no encaja en el schema: %w", err)
	}

	// El cruce que atrapa lo que el schema no puede: el JSON valida perfecto y
	// el precio puede seguir estando mal.
	marcados := carta.Verificar()

	return &Resultado{Carta: carta, Uso: resp.Uso, Marcas: marcados}, nil
}
