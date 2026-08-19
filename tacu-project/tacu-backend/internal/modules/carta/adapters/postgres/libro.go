package postgres

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/platform/ai"
)

// El Libro persiste lo que costo cada llamada a la IA.
//
// Hasta ahora la unica implementacion era LibroNulo, un no-op: el costo se
// calculaba bien y se tiraba. Con esto, "cuanto costo importar la carta de este
// restaurante" pasa a ser una consulta.

type claveCtx struct{}

// ConImportacion marca el contexto con la importacion a la que cargarle el
// gasto.
//
// Va por contexto y no por parametro porque ai.Libro.Anotar(ctx, Uso) no recibe
// id, y cambiar esa firma obligaria a tocar el paquete ai —que tiene 88% de
// cobertura y es el activo mas caro del repo— para algo que solo le importa a
// este adaptador. El ctx ya viaja entero desde quien llama hasta Anotar.
func ConImportacion(ctx context.Context, importacionID id.ID) context.Context {
	return context.WithValue(ctx, claveCtx{}, importacionID)
}

func importacionDe(ctx context.Context) *uuid.UUID {
	v, ok := ctx.Value(claveCtx{}).(id.ID)
	if !ok {
		return nil
	}
	return &v
}

type Libro struct {
	repo *Repo
	log  *slog.Logger
}

func NuevoLibro(repo *Repo, log *slog.Logger) *Libro {
	return &Libro{repo: repo, log: log}
}

// Anotar registra una llamada.
//
// NO devuelve error, y es deliberado: la llamada a la IA ya se pago cuando esto
// corre. Un fallo al registrarla no puede tumbar una respuesta que el usuario ya
// tiene y que el proveedor ya cobro. Se loguea y se sigue.
//
// Sin importacion en el contexto —los CLIs de laboratorio— se anota igual con
// importacion_id NULL: ese gasto es real aunque no pertenezca a ninguna carta.
func (l *Libro) Anotar(ctx context.Context, u ai.Uso) {
	err := l.repo.AnotarGasto(ctx, importacionDe(ctx),
		string(u.Tarea), u.Modelo,
		u.TokensEntrada, u.TokensSalida, u.Imagenes,
		dinero.USD(u.CostoUSD), u.Intentos)
	if err != nil {
		l.log.Error("no se pudo anotar el gasto de IA",
			"tarea", u.Tarea, "modelo", u.Modelo, "costo_usd", u.CostoUSD, "error", err)
	}
}
