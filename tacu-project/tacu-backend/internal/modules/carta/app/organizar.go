package app

import (
	"context"
	"encoding/json"
	"fmt"

	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// El modelo recibe SOLO el resumen de categorias y devuelve SOLO una permutacion
// de indices con un nombre para cada una. Nunca ve un plato, nunca ve un precio.
//
// Esa restriccion es lo que hace la operacion segura: no es que confiemos en que
// el modelo no toque nada, es que no le damos nada que tocar. Lo unico que puede
// devolver mal es el ORDEN, y eso lo atrapa domain.Reordenar.
const schemaOrden = `{
  "type": "object",
  "required": ["orden"],
  "additionalProperties": false,
  "properties": {
    "orden": {
      "type": "array",
      "description": "Las categorias en el orden en que deben salir en la carta web. Tiene que incluir TODAS exactamente una vez.",
      "items": {
        "type": "object",
        "required": ["indice", "nombre"],
        "additionalProperties": false,
        "properties": {
          "indice": {
            "type": "integer",
            "description": "El indice de la categoria tal como te la pasaron. Cada indice aparece UNA sola vez."
          },
          "nombre": {
            "type": "string",
            "description": "Como debe titularse esa categoria en la web. Corrige mayusculas gritadas y abreviaturas, pero NO inventes uno nuevo si el de la carta ya es bueno."
          }
        }
      }
    }
  }
}`

const sistemaOrganizar = `Ordenas la carta de un restaurante peruano para que venda.

No transcribes, no corriges precios y no tocas platos: solo decides en que orden
salen las secciones y como se titulan.`

const promptOrganizar = `Te paso las secciones de la carta de un restaurante, con
cuantos platos tiene cada una y tres ejemplos.

Devuelvelas ORDENADAS como deben aparecer en la carta web de ese restaurante, y con
el titulo que debe llevar cada una.

COMO SE ORDENA UNA CARTA QUE VENDE:

1. Arriba lo que el restaurante quiere vender y lo que deja mas margen: los combos,
   las promociones, las fuentes o platos para compartir. En una polleria el cliente
   ya sabe que hay pollo; lo que decide el ticket es si se lleva el combo.

2. Despues los platos principales, que son la razon por la que el cliente entro.

3. Al final lo que se pide DE ANADIDURA, nunca primero: guarniciones, porciones
   sueltas, extras. Una carta que empieza por "Porcion de papas" esta enterrando su
   propio producto.

4. Las bebidas van ultimas. Se piden igual y ocupar el primer scroll con ellas es
   gastar el mejor sitio de la pantalla.

SOBRE LOS TITULOS:

- Si el titulo de la carta ya es claro, DEJALO. No renombres por renombrar.
- Arregla el grito: "PARA LLEVAR" pasa a "Para llevar". Las mayusculas completas se
  leen peor en pantalla y no aportan nada.
- Si una seccion no tenia titulo en la carta y le pusieron uno generico, mejoralo
  con lo que digan sus platos.
- Nunca inventes un titulo que prometa algo que la seccion no tiene.

REGLAS QUE NO SE ROMPEN:

- Devuelve TODAS las secciones, cada una EXACTAMENTE UNA VEZ. Si omites una,
  desaparecen todos sus platos de la carta publicada. Si repites un indice, esa
  seccion sale dos veces.
- Los indices son los que te pasaron. No los renumeres.`

// Organizar decide el orden de las categorias de una carta.
type Organizar struct {
	ia *ai.Cliente
}

func NuevoOrganizar(ia *ai.Cliente) *Organizar { return &Organizar{ia: ia} }

// Ejecutar devuelve la carta reordenada.
//
// NO devuelve error cuando el modelo falla o responde algo incoherente: devuelve
// la carta ORIGINAL y el motivo. Organizar es una mejora, no un requisito — una
// carta en orden de lectura se publica igual, y tumbar una importacion de $0.017
// porque el reordenado de $0.002 fallo seria absurdo.
//
// El Uso se devuelve siempre, incluso al fallar: la llamada ya se pago.
func (uc *Organizar) Ejecutar(ctx context.Context, c domain.Carta) (domain.Carta, ai.Uso, error) {
	// Con una sola categoria no hay nada que ordenar y no vale gastar una
	// llamada. Con ninguna, menos.
	if len(c.Categorias) < 2 {
		return c, ai.Uso{}, nil
	}

	resumen, err := json.Marshal(c.Resumir())
	if err != nil {
		return c, ai.Uso{}, fmt.Errorf("resumiendo la carta: %w", err)
	}

	resp, err := uc.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:      ai.OrganizarCarta,
		Sistema:    sistemaOrganizar,
		Prompt:     promptOrganizar + "\n\nLAS SECCIONES:\n" + string(resumen),
		SchemaJSON: []byte(schemaOrden),
		MaxTokens:  4000,
	})
	if err != nil {
		return c, resp.Uso, fmt.Errorf("organizando la carta: %w", err)
	}

	var salida struct {
		Orden []struct {
			Indice int    `json:"indice"`
			Nombre string `json:"nombre"`
		} `json:"orden"`
	}
	if err := json.Unmarshal(resp.JSON, &salida); err != nil {
		return c, resp.Uso, fmt.Errorf("la respuesta no encaja en el schema: %w", err)
	}

	orden := make([]int, len(salida.Orden))
	nombres := make([]string, len(salida.Orden))
	for i, o := range salida.Orden {
		orden[i], nombres[i] = o.Indice, o.Nombre
	}

	// Aqui es donde se atrapa una respuesta que borraria una categoria entera.
	nueva, err := c.Reordenar(orden, nombres)
	if err != nil {
		return c, resp.Uso, fmt.Errorf("el orden que devolvio el modelo no sirve: %w", err)
	}
	return nueva, resp.Uso, nil
}
