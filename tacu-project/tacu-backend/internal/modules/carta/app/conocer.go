package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// El modelo devuelve SIEMPRE la misma forma: por cada nombre que no emparejo,
// una entrada de banco completa. Si su clave ya existe, solo se enlaza y la
// descripcion se tira; si es nueva, entra al banco.
//
// Plana y sin partes opcionales a proposito: un schema con un objeto anidado
// "solo si no encaja en ninguna" es donde los modelos devuelven la mitad de las
// veces null y la otra mitad un objeto vacio.
const schemaConocer = `{
  "type": "object",
  "required": ["platos"],
  "additionalProperties": false,
  "properties": {
    "platos": {
      "type": "array",
      "description": "Una entrada por cada nombre que te pasaron, en el mismo orden.",
      "items": {
        "type": "object",
        "required": ["nombre_impreso", "clave", "nombre", "cocina", "curso", "aspecto", "recipiente", "guarnicion", "jamas", "patrones"],
        "additionalProperties": false,
        "properties": {
          "nombre_impreso": {"type": "string", "description": "El nombre tal como te lo pasaron, sin cambiar nada."},
          "clave": {"type": "string", "description": "Si el plato ES uno de los del banco, su clave exacta. Si no es ninguno, una clave nueva en minusculas y con guiones."},
          "nombre": {"type": "string", "description": "Como se llama el plato en general, no el nombre comercial de esa carta."},
          "cocina": {"type": "string", "enum": ["cevicheria", "polleria", "chifa", "criollo", "parrilla", "pizzeria", "sanguicheria", ""]},
          "curso": {"type": "string", "enum": ["entrada", "fondo", "sopa", "postre", "bebida", "guarnicion", "otro"]},
          "aspecto": {"type": "string", "description": "EN INGLES. Como se VE el plato ya servido: componentes visibles, color, textura, como esta dispuesto. Nada de ingredientes crudos ni de como se cocina."},
          "recipiente": {"type": "string", "description": "EN INGLES. En que se sirve: plato llano, bol hondo, vaso, bandeja."},
          "guarnicion": {"type": "string", "description": "EN INGLES. Lo que lo acompana POR SER ESTE plato, en el mismo plato o al lado. Vacio si va solo."},
          "jamas": {"type": "string", "description": "EN INGLES. Lo que el plato NO es, para corregir al generador: 'nunca cocido', 'nunca empanizado', 'no es una sopa'."},
          "patrones": {
            "type": "array",
            "description": "Trozos de nombre impreso que identifican este plato, en minusculas y con guiones. Cortos: 'ceviche', no 'ceviche-de-pescado-fresco'.",
            "items": {"type": "string"}
          }
        }
      }
    }
  }
}`

const sistemaConocer = `Sabes como se ve, ya servido en la mesa, cualquier plato de
un restaurante peruano.

Tu trabajo NO es cocinar ni dar recetas: es describir lo que la camara veria.`

const promptConocer = `Te paso nombres de platos sacados de la carta de un restaurante
peruano, y la lista de platos que ya conozco.

Te paso ademas platos que YA estan en el banco pero de los que solo se el nombre y
los ingredientes: de esos NO inventes clave nueva, devuelve su clave tal cual y
describelos con los ingredientes que te doy como contexto.

Por cada nombre, decide:

1. Si es uno de los que YA CONOZCO —aunque la carta lo llame distinto—, devuelve su
   clave exacta. "Ceviche de Conchas Negras" es un ceviche. "Chaufa de Pollo" es un
   arroz chaufa. Esto es lo que hay que hacer casi siempre.

2. Solo si NO es ninguno de ellos, invéntale una clave nueva y describelo.

Un nombre comercial casi nunca es un plato nuevo: "Combo Tribuna", "El Especial de la
Casa" o "Promocion 2" son platos conocidos con nombre de marketing. Si por el nombre no
puedes saber que lleva, elige el plato conocido mas probable de esa carta antes que
inventar uno.

COMO SE DESCRIBE PARA UNA FOTO:

- aspecto: lo que se VE. "cubos de pescado blanco crudo en un charco de leche de tigre
  turbia, con tiras de cebolla morada encima". No "pescado, limon, cebolla": eso es la
  receta, no la foto.
- jamas: lo que corrige al generador cuando su estadistica tira para otro lado. Es el
  campo que evita que un ceviche salga cocido o un pollo a la brasa salga rostizado de
  supermercado. Nunca lo dejes vacio.
- aspecto, recipiente, guarnicion y jamas van EN INGLES. El resto en espanol.`

// maxAprendidosPorCarta acota lo que UNA carta puede meter en el banco.
//
// El banco lo comparten todos los restaurantes: una carta con nombres raros
// —"Promocion 1", "Promocion 2"...— podria meter treinta entradas inventadas y
// ensuciarlo para siempre. Diez es mas de lo que una carta real necesita: La
// Tribuna, con 74 platos, deja 3 sin emparejar.
const maxAprendidosPorCarta = 10

// RepoConocer es lo que hace falta para que el banco crezca y para arrastrar lo
// aprendido a una carta que ya estaba leida.
type RepoConocer interface {
	GuardarPlatosTipicos(ctx context.Context, platos []domain.PlatoTipico) error
	BancoDePlatos(ctx context.Context) ([]domain.PlatoTipico, error)
	Obtener(ctx context.Context, importacionID id.ID) (domain.Importacion, error)
	ActualizarTipicos(ctx context.Context, platos []domain.Plato) error
}

// Conocer amplia el banco de platos con lo que la carta trae y el banco todavia
// no sabe.
//
// Es una tarea APARTE de la extraccion y no un campo mas en su schema: asi se
// puede volver a correr sobre una carta ya leida cuando el banco crezca, sin
// pagar otra vez por leer las fotos de la carta. Y no toca un camino medido.
type Conocer struct {
	ia   *ai.Cliente
	repo RepoConocer
}

func NuevoConocer(ia *ai.Cliente, repo RepoConocer) *Conocer {
	return &Conocer{ia: ia, repo: repo}
}

// Ejecutar mira los platos sin emparejar, pregunta por ellos y guarda lo que
// resulte ser nuevo. Devuelve el banco con lo aprendido dentro.
//
// NO devuelve error cuando el modelo falla: devuelve el banco tal como estaba.
// Igual que organizar, esto es una mejora — una carta cuyos platos no se
// reconocen se publica igual, con las fotos que se hacian antes de que el banco
// existiera, y tumbar una lectura ya pagada por esto seria absurdo.
func (uc *Conocer) Ejecutar(ctx context.Context, c domain.Carta,
	banco []domain.PlatoTipico) ([]domain.PlatoTipico, map[string]string, ai.Uso, error) {
	desconocidos, aMedias := loQueFalta(c, banco)
	if len(desconocidos) == 0 && len(aMedias) == 0 {
		return banco, nil, ai.Uso{}, nil
	}

	resp, err := uc.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:   ai.ConocerPlatos,
		Sistema: sistemaConocer,
		Prompt: promptConocer + conocidos(banco) +
			porConocer(desconocidos) + porDescribir(aMedias),
		SchemaJSON: []byte(schemaConocer),
		MaxTokens:  8000,
	})
	if err != nil {
		return banco, nil, resp.Uso, fmt.Errorf("conociendo platos: %w", err)
	}

	var salida struct {
		Platos []struct {
			NombreImpreso string   `json:"nombre_impreso"`
			Clave         string   `json:"clave"`
			Nombre        string   `json:"nombre"`
			Cocina        string   `json:"cocina"`
			Curso         string   `json:"curso"`
			Aspecto       string   `json:"aspecto"`
			Recipiente    string   `json:"recipiente"`
			Guarnicion    string   `json:"guarnicion"`
			Jamas         string   `json:"jamas"`
			Patrones      []string `json:"patrones"`
		} `json:"platos"`
	}
	if err := json.Unmarshal(resp.JSON, &salida); err != nil {
		return banco, nil, resp.Uso, fmt.Errorf("la respuesta no encaja en el schema: %w", err)
	}

	// El schema valida la FORMA, no los valores: la clave puede venir con tildes
	// o con espacios, el curso puede ser cualquier cosa y la descripcion puede
	// venir vacia. Lo que no pase por aqui no entra al banco.
	porClave := make(map[string]domain.PlatoTipico, len(banco))
	for _, p := range banco {
		porClave[p.Clave] = p
	}

	preguntados := make([]string, 0, len(desconocidos))
	for _, d := range desconocidos {
		preguntados = append(preguntados, domain.Slug(d.Nombre))
	}

	// Los ENLACES son la otra mitad del trabajo y la que mas se nota: cuando el
	// modelo dice que "Combo Tribuna" es un chaufa, eso no lo va a descubrir
	// ningun patron. Sin guardarlos, la respuesta se tiraba entera.
	enlaces := map[string]string{}

	var nuevos []domain.PlatoTipico
	for _, p := range salida.Platos {
		clave := domain.Slug(p.Clave)
		if clave == "" {
			continue
		}
		if impreso := domain.Slug(p.NombreImpreso); impreso != "" {
			enlaces[impreso] = clave
		}

		viejo, existe := porClave[clave]
		if existe && viejo.Descrito() {
			// Enlazar con uno ya descrito es el caso bueno y el mas comun: la
			// descripcion que venga se tira, que para eso el canon esta escrito
			// y medido a mano.
			continue
		}

		nuevo := domain.PlatoTipico{
			Clave:      clave,
			Nombre:     strings.TrimSpace(p.Nombre),
			Cocina:     strings.TrimSpace(p.Cocina),
			Curso:      strings.TrimSpace(p.Curso),
			Aspecto:    strings.TrimSpace(p.Aspecto),
			Recipiente: strings.TrimSpace(p.Recipiente),
			Guarnicion: strings.TrimSpace(p.Guarnicion),
			Jamas:      strings.TrimSpace(p.Jamas),
			Patrones:   patronesLimpios(p.Patrones, clave),
		}
		if existe {
			// De una fila cosechada solo se rellena lo que le falta: el nombre,
			// la cocina y el curso vienen de la cosecha y no los decide el
			// modelo. La consulta ademas solo actualiza filas con aspecto vacio.
			nuevo.Nombre, nuevo.Cocina, nuevo.Curso = viejo.Nombre, viejo.Cocina, viejo.Curso
			nuevo.Patrones = patronesLimpios(append(p.Patrones, viejo.Patrones...), clave)
		} else {
			nuevo.Patrones = patronesDeFiar(nuevo.Patrones, banco, preguntados)
			if len(nuevo.Patrones) == 0 {
				// Todos sus patrones los cubre ya algo descrito: es un duplicado
				// del canon con otro nombre. En la primera corrida real el
				// modelo creo "ceviche-mixto" teniendo "ceviche" delante.
				continue
			}
		}
		if !entradaUsable(nuevo) {
			continue
		}
		porClave[clave] = nuevo
		nuevos = append(nuevos, nuevo)
	}

	if len(nuevos) > maxAprendidosPorCarta {
		// Se corta y se dice. Un recorte silencioso se lee como "el banco ya lo
		// sabia todo", que es justo lo contrario de lo que paso.
		nuevos = nuevos[:maxAprendidosPorCarta]
	}
	if len(nuevos) == 0 {
		return banco, enlaces, resp.Uso, nil
	}
	if err := uc.repo.GuardarPlatosTipicos(ctx, nuevos); err != nil {
		return banco, enlaces, resp.Uso, fmt.Errorf("guardando lo aprendido: %w", err)
	}
	return append(banco, nuevos...), enlaces, resp.Uso, nil
}

// Describir rellena en LOTE filas del banco que solo tienen nombre e
// ingredientes.
//
// Es el mismo trabajo que hace Ejecutar con los platos "a medias" de una carta,
// pero sin carta: sirve para las 413 que trajo la cosecha, que si no se
// rellenarian solo segun fueran apareciendo en cartas reales.
//
// Devuelve cuantas quedaron descritas. El schema valida la forma y estas mismas
// reglas validan los valores: una fila que no sirva para una foto no se guarda.
func (uc *Conocer) Describir(ctx context.Context, pendientes []domain.PlatoTipico) (int, ai.Uso, error) {
	if len(pendientes) == 0 {
		return 0, ai.Uso{}, nil
	}

	resp, err := uc.ia.Ejecutar(ctx, ai.Peticion{
		Tarea:      ai.ConocerPlatos,
		Sistema:    sistemaConocer,
		Prompt:     promptConocer + porDescribir(pendientes),
		SchemaJSON: []byte(schemaConocer),
		MaxTokens:  16000,
	})
	if err != nil {
		return 0, resp.Uso, fmt.Errorf("describiendo platos: %w", err)
	}

	var salida struct {
		Platos []struct {
			Clave      string   `json:"clave"`
			Aspecto    string   `json:"aspecto"`
			Recipiente string   `json:"recipiente"`
			Guarnicion string   `json:"guarnicion"`
			Jamas      string   `json:"jamas"`
			Patrones   []string `json:"patrones"`
		} `json:"platos"`
	}
	if err := json.Unmarshal(resp.JSON, &salida); err != nil {
		return 0, resp.Uso, fmt.Errorf("la respuesta no encaja en el schema: %w", err)
	}

	// Solo se rellenan las que se pidieron: si el modelo devuelve una clave que
	// nadie le paso, se inventa una entrada y eso no entra al banco por aqui.
	porClave := make(map[string]domain.PlatoTipico, len(pendientes))
	for _, p := range pendientes {
		porClave[p.Clave] = p
	}

	var listos []domain.PlatoTipico
	for _, p := range salida.Platos {
		viejo, pedido := porClave[domain.Slug(p.Clave)]
		if !pedido {
			continue
		}
		nuevo := viejo
		nuevo.Aspecto = strings.TrimSpace(p.Aspecto)
		nuevo.Recipiente = strings.TrimSpace(p.Recipiente)
		nuevo.Guarnicion = strings.TrimSpace(p.Guarnicion)
		nuevo.Jamas = strings.TrimSpace(p.Jamas)
		nuevo.Patrones = patronesLimpios(append(p.Patrones, viejo.Patrones...), viejo.Clave)
		if !entradaUsable(nuevo) {
			continue
		}
		listos = append(listos, nuevo)
	}

	if len(listos) == 0 {
		return 0, resp.Uso, nil
	}
	return len(listos), resp.Uso, uc.repo.GuardarPlatosTipicos(ctx, listos)
}

// Aplicar es el flujo entero: emparejar con el banco, preguntar por los huecos y
// volver a emparejar con lo aprendido.
//
// El segundo emparejado no sobra: sin el, lo que se acaba de aprender le sirve a
// la carta SIGUIENTE pero no a la que lo enseno.
//
// Lo usan igual la lectura inicial, la pagina anadida y el reconocer a mano, y
// por eso vive aqui y no repetido en los tres.
func (uc *Conocer) Aplicar(ctx context.Context, c *domain.Carta,
	banco []domain.PlatoTipico) (emparejados, aprendidos int, err error) {
	emparejados = c.EmparejarConElBanco(banco)

	crecido, enlaces, _, err := uc.Ejecutar(ctx, *c, banco)
	if err != nil {
		return emparejados, 0, err
	}
	if aprendidos = len(crecido) - len(banco); aprendidos > 0 {
		emparejados = c.EmparejarConElBanco(crecido)
	}
	emparejados += aplicarEnlaces(c, enlaces, crecido)
	return emparejados, aprendidos, nil
}

// aplicarEnlaces pone la clave que dijo el modelo SOLO en los platos que siguen
// sin ninguna.
//
// Solo en esos a proposito: un patron del canon es determinista y esta medido, y
// una respuesta del modelo es una opinion. La opinion vale donde no hay nada, no
// para pisar lo que ya se sabe. Y lo que corrigio el dueno no lo toca nadie.
func aplicarEnlaces(c *domain.Carta, enlaces map[string]string, banco []domain.PlatoTipico) int {
	if len(enlaces) == 0 {
		return 0
	}
	existe := make(map[string]bool, len(banco))
	for _, t := range banco {
		existe[t.Clave] = true
	}

	enlazados := 0
	for i := range c.Categorias {
		for j := range c.Categorias[i].Platos {
			p := &c.Categorias[i].Platos[j]
			if p.Tipico != "" || p.TipicoOrigen == domain.TipicoDelDueno {
				continue
			}
			clave, ok := enlaces[domain.Slug(p.Nombre)]
			if !ok || !existe[clave] {
				continue
			}
			p.Tipico, p.TipicoOrigen = clave, domain.TipicoPorIA
			enlazados++
		}
	}
	return enlazados
}

// Reconocer vuelve a pasar por el banco una carta que YA estaba guardada.
//
// Hace falta porque el banco crece: una carta leida antes de que existiera —o
// antes de que alguien describiera su plato— se quedaria con sus fotos viejas
// para siempre. Es la unica via para que lo aprendido llegue hacia atras.
//
// No reescribe la carta: solo toca la clave de cada plato, asi que ningun id
// cambia y las fotos ya generadas siguen colgando de su plato.
func (uc *Conocer) Reconocer(ctx context.Context, importacionID id.ID) (emparejados, aprendidos int, err error) {
	imp, err := uc.repo.Obtener(ctx, importacionID)
	if err != nil {
		return 0, 0, err
	}
	if imp.Estado == domain.Leyendo {
		return 0, 0, fmt.Errorf("%w: %s", ErrCartaOcupada, imp.Estado)
	}

	banco, err := uc.repo.BancoDePlatos(ctx)
	if err != nil {
		return 0, 0, err
	}

	carta := imp.Carta
	emparejados, aprendidos, err = uc.Aplicar(ctx, &carta, banco)
	if err != nil {
		// Ampliar el banco es una mejora; lo emparejado con lo que ya habia se
		// guarda igual.
		emparejados = carta.EmparejarConElBanco(banco)
	}

	return emparejados, aprendidos, uc.repo.ActualizarTipicos(ctx, carta.Platos())
}

// entradaUsable descarta lo que no sirve para una foto. Un plato sin aspecto o
// sin patrones es peor que no tenerlo: ocupa una clave en el banco y no aporta
// nada al prompt.
func entradaUsable(p domain.PlatoTipico) bool {
	switch p.Curso {
	case domain.CursoEntrada, domain.CursoFondo, domain.CursoSopa,
		domain.CursoPostre, domain.CursoBebida, domain.CursoGuarnicion, "otro":
	default:
		return false
	}
	return p.Nombre != "" && p.Aspecto != "" && p.Jamas != "" && len(p.Patrones) > 0
}

// patronesDeFiar quita los patrones que no hay que creerse.
//
// Dos filtros, los dos aprendidos de la primera corrida contra una carta real:
//
//  1. El que YA empareja con algo descrito sobra: el canon esta escrito y medido
//     a mano y no lo mejora una entrada nueva con otro nombre.
//  2. El que no aparece en ninguno de los nombres que preguntamos es
//     especulacion. Asi es como "surtido" —un jugo de esa carta— acabo colgando
//     de un ceviche, para todos los restaurantes.
func patronesDeFiar(patrones []string, banco []domain.PlatoTipico, preguntados []string) []string {
	fuera := make([]string, 0, len(patrones))
	for _, p := range patrones {
		if t, ok := domain.EmparejarTipico(p, banco); ok && t.Descrito() {
			continue
		}
		usado := false
		for _, n := range preguntados {
			if strings.Contains("-"+n+"-", "-"+p+"-") {
				usado = true
				break
			}
		}
		if usado {
			fuera = append(fuera, p)
		}
	}
	return fuera
}

// patronesLimpios normaliza lo que devolvio el modelo y garantiza que la propia
// clave sirva de patron: sin eso, una entrada nueva podria no emparejar ni con
// el plato que la origino.
func patronesLimpios(patrones []string, clave string) []string {
	vistos := map[string]bool{}
	fuera := []string{}
	for _, p := range append(patrones, clave) {
		s := domain.Slug(p)
		if s == "" || vistos[s] {
			continue
		}
		vistos[s] = true
		fuera = append(fuera, s)
	}
	return fuera
}

// loQueFalta separa los dos huecos del banco que esta carta destapa: los nombres
// que no emparejaron con nada, y las entradas que SI emparejaron pero que nadie
// ha descrito todavia — las 413 que trajo la cosecha con su lista de
// ingredientes y sin una sola linea de como se ven.
func loQueFalta(c domain.Carta, banco []domain.PlatoTipico) (desconocidos []platoSuelto, aMedias []domain.PlatoTipico) {
	porClave := make(map[string]domain.PlatoTipico, len(banco))
	for _, p := range banco {
		porClave[p.Clave] = p
	}

	vistos := map[string]bool{}
	for _, cat := range c.Categorias {
		for _, p := range cat.Platos {
			if p.Tipico == "" {
				if !vistos[p.Nombre] {
					vistos[p.Nombre] = true
					desconocidos = append(desconocidos, platoSuelto{Nombre: p.Nombre, Seccion: cat.Nombre})
				}
				continue
			}
			if t, ok := porClave[p.Tipico]; ok && !t.Descrito() && !vistos[t.Clave] {
				vistos[t.Clave] = true
				aMedias = append(aMedias, t)
			}
		}
	}
	return desconocidos, aMedias
}

func conocidos(banco []domain.PlatoTipico) string {
	var b strings.Builder
	b.WriteString("\n\nLOS QUE YA CONOZCO (clave: nombre):\n")
	for _, p := range banco {
		b.WriteString(p.Clave)
		b.WriteString(": ")
		b.WriteString(p.Nombre)
		b.WriteByte('\n')
	}
	return b.String()
}

// platoSuelto es un nombre impreso con la seccion en la que salio.
//
// La SECCION es el contexto que faltaba y costo caro no mandar: en la primera
// corrida real el modelo vio "Surtido" a secas y decidio que era un ceviche
// surtido. Estaba en la seccion de jugos.
type platoSuelto struct {
	Nombre  string
	Seccion string
}

func porConocer(platos []platoSuelto) string {
	if len(platos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nLOS NOMBRES DE ESTA CARTA, con la seccion en la que salen.\n")
	b.WriteString("La seccion manda: un nombre suelto en la seccion de bebidas es una bebida.\n")
	for _, p := range platos {
		b.WriteString("- ")
		b.WriteString(p.Nombre)
		if p.Seccion != "" {
			b.WriteString("   [seccion: ")
			b.WriteString(p.Seccion)
			b.WriteString("]")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// porDescribir manda la clave con sus ingredientes: es el contexto que convierte
// "Patarashca" en algo que se puede fotografiar.
func porDescribir(platos []domain.PlatoTipico) string {
	if len(platos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nESTOS YA ESTAN EN EL BANCO Y SOLO FALTA SABER COMO SE VEN.\n")
	b.WriteString("Devuelve su clave TAL CUAL, no inventes una nueva:\n")
	for _, p := range platos {
		b.WriteString(p.Clave)
		b.WriteString(": ")
		b.WriteString(p.Nombre)
		if len(p.Ingredientes) > 0 {
			b.WriteString(" — lleva: ")
			b.WriteString(strings.Join(p.Ingredientes, ", "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
