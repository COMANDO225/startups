// Package core es la raiz de composicion: el unico lugar que conoce a todos los
// demas paquetes y los conecta.
package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/ai/proveedores/gemini"
	"tacu-backend/internal/platform/ai/proveedores/openai"
	"tacu-backend/internal/platform/config"
)

// ArmarIA construye el cliente de IA a partir de la configuracion.
//
// Los proveedores se registran solo si tienen API key. Uno sin key no es un
// error: es un proveedor que hoy no esta disponible, y la cadena de cada tarea
// existe justamente para eso. Lo que SI es error es que una tarea se quede sin
// ningun modelo alcanzable — eso mata el arranque, porque descubrirlo cuando un
// dueno sube su carta es tardisimo.
//
// requeridas acota que tareas tienen que estar operativas. El servicio las
// declara todas; una herramienta que solo lee cartas declara solo esa y no
// necesita las credenciales de generacion de imagenes. Vacio = todas.
func ArmarIA(ctx context.Context, cfg config.IA, libro ai.Libro, log *slog.Logger, requeridas ...ai.Tarea) (*ai.Cliente, error) {
	cadenas, err := interpretarCadenas(cfg.Tareas)
	if err != nil {
		return nil, err
	}

	cliente := ai.NuevoCliente(cadenas, interpretarPrecios(cfg.Precios), libro, log)

	disponibles := map[string]bool{}

	for nombre, pc := range cfg.Proveedores {
		clave := strings.TrimSpace(pc.APIKey)
		if clave == "" {
			log.Warn("proveedor sin API key: se omite", "proveedor", nombre)
			continue
		}

		prov, err := construir(ctx, nombre, clave)
		if err != nil {
			return nil, fmt.Errorf("proveedor %s: %w", nombre, err)
		}

		cliente.Registrar(prov)
		disponibles[nombre] = true
		log.Info("proveedor de IA registrado", "proveedor", nombre)
	}

	if err := verificarAlcanzables(cadenas, disponibles, requeridas, log); err != nil {
		return nil, err
	}

	return cliente, nil
}

func construir(ctx context.Context, nombre, clave string) (ai.Proveedor, error) {
	switch nombre {
	case "gemini":
		return gemini.Nuevo(ctx, clave)
	case "openai":
		return openai.Nuevo(clave)
	}
	return nil, fmt.Errorf("proveedor desconocido: %q", nombre)
}

func interpretarCadenas(tareas map[string][]string) (ai.Cadenas, error) {
	cadenas := make(ai.Cadenas, len(tareas))
	var problemas []error

	for nombre, refs := range tareas {
		modelos := make([]ai.Modelo, 0, len(refs))
		for _, ref := range refs {
			m, err := ai.ParsearModelo(ref)
			if err != nil {
				problemas = append(problemas, fmt.Errorf("tarea %s: %w", nombre, err))
				continue
			}
			modelos = append(modelos, m)
		}
		cadenas[ai.Tarea(nombre)] = modelos
	}

	return cadenas, errors.Join(problemas...)
}

func interpretarPrecios(precios map[string]config.Precio) ai.Precios {
	out := make(ai.Precios, len(precios))
	for ref, p := range precios {
		out[ref] = ai.Precio{
			EntradaPorMillon: p.EntradaPorMillon,
			SalidaPorMillon:  p.SalidaPorMillon,
			PorImagen:        p.PorImagen,
		}
	}
	return out
}

// verificarAlcanzables distingue dos situaciones que se parecen y no son lo
// mismo:
//
//   - una tarea perdio un respaldo -> aviso, sigue funcionando degradada
//   - una tarea se quedo sin NINGUN modelo -> el arranque falla
func verificarAlcanzables(cadenas ai.Cadenas, disponibles map[string]bool, requeridas []ai.Tarea, log *slog.Logger) error {
	var muertas []error

	exigida := func(t ai.Tarea) bool {
		if len(requeridas) == 0 {
			return true
		}
		for _, r := range requeridas {
			if r == t {
				return true
			}
		}
		return false
	}

	for tarea, cadena := range cadenas {
		if !exigida(tarea) {
			continue
		}

		alcanzables := 0
		for _, m := range cadena {
			if disponibles[m.Proveedor] {
				alcanzables++
			}
		}

		switch {
		case alcanzables == 0:
			muertas = append(muertas, fmt.Errorf(
				"la tarea %q no tiene ningun modelo disponible: falta la API key de %s",
				tarea, proveedoresDe(cadena)))
		case alcanzables < len(cadena):
			log.Warn("tarea sin respaldo: si el proveedor falla, la tarea falla",
				"tarea", string(tarea), "disponibles", alcanzables, "configurados", len(cadena))
		}
	}

	return errors.Join(muertas...)
}

func proveedoresDe(cadena []ai.Modelo) string {
	vistos := map[string]bool{}
	var nombres []string
	for _, m := range cadena {
		if !vistos[m.Proveedor] {
			vistos[m.Proveedor] = true
			nombres = append(nombres, m.Proveedor)
		}
	}
	return strings.Join(nombres, " o ")
}
