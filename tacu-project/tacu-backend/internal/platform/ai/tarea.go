package ai

import (
	"errors"
	"fmt"
)

// Tarea es lo que la aplicacion pide. NUNCA pide un modelo.
//
// Cada tarea tiene requisitos distintos: leer una carta necesita el modelo que
// mejor asocia precio con plato; verificar si una foto corresponde a un plato es
// una decision binaria que resuelve el mas barato. Usar el mismo modelo para
// ambas significa o pagar de mas en lo facil, o rendir de menos en lo dificil.
type Tarea string

const (
	LeerCarta      Tarea = "leer_carta"
	LeerFachada    Tarea = "leer_fachada"
	VerificarFoto  Tarea = "verificar_foto"
	ExpandirPrompt Tarea = "expandir_prompt"
	OrganizarCarta Tarea = "organizar_carta"
	ConocerPlatos  Tarea = "conocer_platos"
	GenerarFoto    Tarea = "generar_foto"
	GenerarLogo    Tarea = "generar_logo"
)

// Capacidad devuelve que sabe hacer falta para atender esta tarea.
func (t Tarea) Capacidad() Capacidad {
	switch t {
	case LeerCarta, LeerFachada, VerificarFoto:
		return Vision
	case ExpandirPrompt, OrganizarCarta, ConocerPlatos:
		return Texto
	case GenerarFoto, GenerarLogo:
		return GenerarImagen
	}
	return ""
}

// Modelo identifica un modelo concreto de un proveedor concreto: "gemini/gemini-3.7-flash".
type Modelo struct {
	Proveedor string
	Nombre    string
}

func (m Modelo) String() string { return m.Proveedor + "/" + m.Nombre }

// ParsearModelo lee "proveedor/modelo" tal como viene del config.
func ParsearModelo(s string) (Modelo, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' && i > 0 && i < len(s)-1 {
			return Modelo{Proveedor: s[:i], Nombre: s[i+1:]}, nil
		}
	}
	return Modelo{}, fmt.Errorf("modelo %q: se espera el formato proveedor/modelo", s)
}

// Cadenas mapea cada tarea a los modelos que la atienden, en orden de
// preferencia. Vive en config para que cambiar de modelo, de proveedor o de
// orden no sea un cambio de codigo.
type Cadenas map[Tarea][]Modelo

// Para devuelve la cadena de una tarea.
func (c Cadenas) Para(t Tarea) ([]Modelo, error) {
	cadena, ok := c[t]
	if !ok || len(cadena) == 0 {
		return nil, fmt.Errorf("la tarea %q no tiene modelos configurados", t)
	}
	return cadena, nil
}

// Validar comprueba al arrancar que toda tarea conocida tenga cadena, que todo
// modelo referenciado tenga proveedor registrado, y que ese proveedor SEPA HACER
// la tarea.
//
// Lo ultimo no es teorico: el YAML declaraba gemini como respaldo de
// generar_foto cuando el proveedor de gemini solo soportaba vision y texto. La
// configuracion arrancaba sin quejarse y prometia un respaldo que no existia; se
// habria descubierto el dia que OpenAI fallara, que es justo el dia en que el
// respaldo importa.
//
// Es deliberadamente estricto: un modelo mal escrito en el YAML tiene que matar
// el arranque, no descubrirse en produccion cuando un dueno sube su carta.
func (c Cadenas) Validar(capacidades map[string]func(Capacidad) bool) error {
	todas := []Tarea{LeerCarta, LeerFachada, VerificarFoto, ExpandirPrompt,
		OrganizarCarta, GenerarFoto, GenerarLogo}

	// Se acumulan TODOS los problemas: arreglar la configuracion de a un error
	// por arranque es una perdida de tiempo, y cada intento cuesta levantar el
	// servicio entero.
	var problemas []error

	for _, t := range todas {
		cadena, ok := c[t]
		if !ok || len(cadena) == 0 {
			problemas = append(problemas, fmt.Errorf("la tarea %q no tiene modelos configurados", t))
			continue
		}
		for _, m := range cadena {
			soporta, registrado := capacidades[m.Proveedor]
			if !registrado {
				problemas = append(problemas,
					fmt.Errorf("la tarea %q usa el proveedor %q, que no esta registrado", t, m.Proveedor))
				continue
			}
			if !soporta(t.Capacidad()) {
				problemas = append(problemas,
					fmt.Errorf("la tarea %q necesita %q y el proveedor %q no lo soporta (modelo %s)",
						t, t.Capacidad(), m.Proveedor, m))
			}
		}
	}
	return errors.Join(problemas...)
}
