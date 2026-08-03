package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"facturacion-service/internal/features/comprobante/infra/worker"
	"facturacion-service/internal/shared/adapters/database"
	"facturacion-service/internal/shared/config"
	"facturacion-service/internal/shared/logger"
	"facturacion-service/internal/shared/server"
	"facturacion-service/internal/shared/validator"
	"facturacion-service/pkg/crypto"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type App struct {
	Config *config.Config
	Log    *logger.Logger
	DB     *database.Pool
	Server *server.Server
	River  *river.Client[pgx.Tx]

	// riverWorking pasa a true cuando se registran Queues y Workers.
	// En insert-only el cliente no se arranca.
	riverWorking bool
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("cargando configuracion: %w", err)
	}

	log, err := logger.New(cfg.Logger.Level, cfg.Logger.Format)
	if err != nil {
		return nil, fmt.Errorf("inicializando logger: %w", err)
	}

	log.Infow("iniciando",
		"app", cfg.App.Name,
		"env", cfg.App.Environment,
		"version", cfg.App.Version,
	)

	db, err := database.NewPool(ctx, cfg.Database.DSN,
		cfg.Database.MaxConns, cfg.Database.MinConns,
		cfg.Database.MaxLifetime, cfg.Database.MaxIdleTime,
	)
	if err != nil {
		return nil, fmt.Errorf("conectando a postgres: %w", err)
	}

	srv := server.New(server.Config{
		AppName:      cfg.App.Name,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		BodyLimit:    cfg.Server.BodyLimit,
		TrustProxy:   cfg.Server.TrustProxy,
		ProxyHeader:  cfg.Server.ProxyHeader,
	}, log, validator.New())

	app := &App{Config: cfg, Log: log, DB: db, Server: srv}

	if err := migrateAll(ctx, db.Pool, cfg.Database.DSN); err != nil {
		return nil, err
	}

	cipher, err := crypto.NewCipher(cfg.Crypto.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("inicializando cifrado: %w", err)
	}

	// El bundle se pasa por referencia: los workers se registran despues de
	// crear el cliente, en el cableado de cada feature, y antes de Start.
	workers := river.NewWorkers()

	periodicos := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(5*time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return worker.RescatarArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		),
		// Las boletas se acumulan durante el dia; el barrido corre seguido para
		// no dejar ninguna sin resumen mas de un rato.
		river.NewPeriodicJob(
			river.PeriodicInterval(15*time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return worker.ResumenDiarioArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		),
	}

	riverClient, err := river.NewClient(riverpgxv5.New(db.Pool), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers:      workers,
		PeriodicJobs: periodicos,

		// River cancela el contexto del job a los 60s por defecto, sin importar
		// el timeout del cliente HTTP: la consulta de ticket a SUNAT tarda mas
		// y moria cancelada antes de poder reprogramarse.
		JobTimeout: config.JobTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("creando cliente river: %w", err)
	}
	app.River = riverClient
	app.riverWorking = true

	deps := app.wireComprobante(cipher, workers)

	app.registerMiddleware()
	app.registerRoutes(deps)

	return app, nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if a.riverWorking {
		if err := a.River.Start(ctx); err != nil {
			return fmt.Errorf("arrancando river: %w", err)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		a.Log.Infow("escuchando", "puerto", a.Config.Server.Port)
		errCh <- a.Server.Listen(a.Config.Server.Port)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.Log.Infow("apagando")
	}

	return a.Shutdown()
}

func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.River.Stop(ctx); err != nil {
		a.Log.Errorw("deteniendo river", "error", err)
	}
	if err := a.Server.Shutdown(); err != nil {
		a.Log.Errorw("deteniendo servidor", "error", err)
	}

	a.DB.Close()

	// Sync sobre stderr devuelve EINVAL en Linux: no es un fallo real.
	_ = a.Log.Sync()
	return nil
}
