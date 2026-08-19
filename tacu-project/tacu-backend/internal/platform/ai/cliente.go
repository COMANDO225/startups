package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Libro registra cada llamada. La implementacion real persiste en Postgres y
// exporta a Prometheus; en herramientas de linea de comandos basta LibroNulo.
type Libro interface {
	Anotar(ctx context.Context, u Uso)
}

type LibroNulo struct{}

func (LibroNulo) Anotar(context.Context, Uso) {}

// Cliente resuelve una TAREA: busca su cadena de modelos en el config y los
// prueba en orden hasta que uno responda.
type Cliente struct {
	proveedores map[string]Proveedor
	cadenas     Cadenas
	precios     Precios
	libro       Libro
	log         *slog.Logger
}

func NuevoCliente(cadenas Cadenas, precios Precios, libro Libro, log *slog.Logger) *Cliente {
	if libro == nil {
		libro = LibroNulo{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Cliente{
		proveedores: map[string]Proveedor{},
		cadenas:     cadenas,
		precios:     precios,
		libro:       libro,
		log:         log,
	}
}

func (c *Cliente) Registrar(p Proveedor) *Cliente {
	c.proveedores[p.Nombre()] = p
	return c
}

// Validar comprueba al arrancar que la configuracion sea coherente con los
// proveedores realmente registrados. Un modelo mal escrito en el YAML debe
// matar el arranque, no descubrirse cuando un dueno sube su carta.
func (c *Cliente) Validar() error {
	capacidades := make(map[string]func(Capacidad) bool, len(c.proveedores))
	for nombre, prov := range c.proveedores {
		capacidades[nombre] = prov.Soporta
	}
	return c.cadenas.Validar(capacidades)
}

// Ejecutar prueba los modelos de la cadena de la tarea, en orden.
//
// Un error terminal corta la cadena: si la peticion esta mal formada o el
// contenido fue rechazado por moderacion, probar en otro modelo solo gasta
// dinero. Un error transitorio (429, 503, timeout) pasa al siguiente.
//
// El Uso se anota SIEMPRE, incluso cuando la cadena entera falla: los intentos
// fallidos tambien se pagan, y sin registrarlos el costo real queda invisible.
func (c *Cliente) Ejecutar(ctx context.Context, p Peticion) (Respuesta, error) {
	cadena, err := c.cadenas.Para(p.Tarea)
	if err != nil {
		return Respuesta{}, err
	}

	var fallos []error

	for i, modelo := range cadena {
		if err := ctx.Err(); err != nil {
			return Respuesta{}, err
		}

		prov, ok := c.proveedores[modelo.Proveedor]
		if !ok {
			fallos = append(fallos, fmt.Errorf("%w: %s", ErrSinProveedor, modelo.Proveedor))
			continue
		}
		if !prov.Soporta(p.Tarea.Capacidad()) {
			fallos = append(fallos, fmt.Errorf("%s no soporta %s", modelo, p.Tarea.Capacidad()))
			continue
		}

		resp, err := prov.Ejecutar(ctx, modelo.Nombre, p)

		resp.Uso.Modelo = modelo.String()
		resp.Uso.Tarea = p.Tarea
		resp.Uso.Intentos = i + 1
		resp.Uso.CostoUSD = c.precios.Costo(modelo, resp.Uso)

		if err == nil {
			c.libro.Anotar(ctx, resp.Uso)
			return resp, nil
		}

		// Lo consumido se anota aunque haya fallado: tambien se paga.
		if resp.Uso.TokensEntrada > 0 || resp.Uso.TokensSalida > 0 || resp.Uso.Imagenes > 0 {
			c.libro.Anotar(ctx, resp.Uso)
		}

		fallos = append(fallos, err)

		if EsTerminal(err) {
			c.log.Warn("error terminal, se corta la cadena",
				"tarea", string(p.Tarea), "modelo", modelo.String(), "error", err)
			return Respuesta{}, errors.Join(fallos...)
		}

		c.log.Warn("modelo fallo, se prueba el siguiente",
			"tarea", string(p.Tarea), "modelo", modelo.String(),
			"restantes", len(cadena)-i-1, "error", err)
	}

	return Respuesta{}, fmt.Errorf("la cadena completa fallo para %s: %w",
		p.Tarea, errors.Join(fallos...))
}
