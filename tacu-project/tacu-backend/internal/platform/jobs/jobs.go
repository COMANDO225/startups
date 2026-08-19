// Package jobs monta la cola de trabajo sobre River.
//
// La cola existe por una razon concreta: leer una carta tarda ~10 s y generar 60
// fotos ~1 minuto, y las dos cosas cuestan dinero. Si el proceso muere a mitad
// —un despliegue, un OOM— lo que falta no lo retoma nadie, porque la peticion
// HTTP que lo lanzo murio hace rato.
//
// De River se usan tres cosas y ninguna es opcional: UniqueOpts para no encolar
// dos veces el mismo trabajo, el backoff con reintentos para el 429 de rutina de
// Gemini, y JobSnooze para posponer sin quemar un intento. Escribir eso a mano
// es escribir una cola.
//
// PERO LA COLA ES REEMPLAZABLE POR CONSTRUCCION: el estado de verdad vive en las
// columnas de la base (importacion.estado, plato.foto_estado), no en River. "Que
// falta por hacer" es una consulta SQL, asi que cualquier planificador serviria
// —hasta un ticker de 30 segundos— y cambiarlo no tocaria el dominio.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Colas. Estan SEPARADAS a proposito y no es cosmetico.
//
// Con una sola cola, los 60 jobs de foto del restaurante A dejan al restaurante
// B esperando un minuto por su extraccion — y esos 10 segundos de la extraccion
// son el momento que vende el producto. Un dueno que sube su carta y ve
// esqueletos durante un minuto porque OTRO restaurante esta generando fotos se
// va antes de llegar a la parte buena.
const (
	ColaLecturas = river.QueueDefault
	ColaFotos    = "fotos"
)

type Config struct {
	// MaxFotos son las generaciones simultaneas contra el proveedor. Sale de
	// ia.max_concurrencia_por_proveedor.
	MaxFotos int

	// TiempoMaxJob acota cuanto puede durar un job. El default de River son 60
	// segundos y NO alcanza: una extraccion con failover a 3.5-flash son 10 s
	// mas 33 s, y ese timeout cancela el contexto del job sin avisar.
	TiempoMaxJob time.Duration
}

// Nuevo arma el cliente con sus workers.
func Nuevo(pool *pgxpool.Pool, workers *river.Workers, cfg Config, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	if cfg.MaxFotos < 1 {
		cfg.MaxFotos = 1
	}
	if cfg.TiempoMaxJob <= 0 {
		cfg.TiempoMaxJob = 3 * time.Minute
	}

	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			ColaLecturas: {MaxWorkers: 4},
			ColaFotos:    {MaxWorkers: cfg.MaxFotos},
		},
		Workers:    workers,
		JobTimeout: cfg.TiempoMaxJob,
		Logger:     log,

		// Cada cuanto se buscan jobs pendientes que ningun aviso desperto. Es la
		// red de seguridad de LISTEN/NOTIFY, no el camino normal: un job
		// encolado arranca en milisegundos, no en un segundo.
		FetchPollInterval: time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("creando el cliente de la cola: %w", err)
	}
	return c, nil
}

// Migrar aplica las tablas propias de River.
//
// Van en su propia migracion y no en sql/migrations/ porque las gestiona la
// libreria: mezclarlas con las nuestras significa que actualizar River obligue a
// escribir a mano una migracion que la libreria ya sabe generar.
func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	migrador, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("creando el migrador de la cola: %w", err)
	}
	if _, err := migrador.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("migrando la cola: %w", err)
	}
	return nil
}
