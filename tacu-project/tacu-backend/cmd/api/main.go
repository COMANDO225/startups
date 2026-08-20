// api es el servicio de Tacu.
//
//	docker compose up -d
//	export TACU_BD_DSN="postgres://tacu:tacu@localhost:5433/tacu?sslmode=disable"
//	set -a; source .env; set +a
//	go run ./cmd/api
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/platform/config"
)

func main() {
	rutaConfig := flag.String("config", "config/config.yaml", "ruta de la configuracion")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := correr(*rutaConfig, log); err != nil {
		log.Error("no se pudo arrancar", "error", err)
		os.Exit(1)
	}
}

func correr(rutaConfig string, log *slog.Logger) error {
	cfg, err := config.Cargar(rutaConfig)
	if err != nil {
		return err
	}

	// El contexto del arranque se cancela al terminar de armar: es para abrir la
	// base y validar la IA, no para la vida del proceso.
	ctxArranque, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	app, err := core.Armar(ctxArranque, cfg, log)
	cancelar()
	if err != nil {
		return err
	}

	// La cola empieza a consumir jobs. Va antes de escuchar HTTP para que un job
	// pendiente de una ejecucion anterior se retome ya, sin esperar una peticion.
	//
	// EL CONTEXTO ES EL DE LA VIDA DEL PROCESO, no uno con timeout. River toma
	// este ctx como el tiempo de vida del cliente: cancelarlo APAGA la cola.
	//
	// La primera version le pasaba un WithTimeout de 15 s y lo cancelaba con
	// defer, creyendo que acotaba el arranque. La cola arrancaba y se detenia en
	// el mismo instante: el job quedaba en la tabla como "available" y no lo
	// tomaba nadie. Desde fuera se veia como una importacion eternamente
	// "leyendo", sin un solo error en el log.
	//
	// El apagado ordenado se hace con Stop(ctx), abajo, que es donde un timeout
	// SI tiene sentido.
	if err := app.Arrancar(context.Background()); err != nil {
		return fmt.Errorf("arrancando la cola: %w", err)
	}

	// Se escucha la senal ANTES de arrancar: si llega mientras el servidor esta
	// subiendo, no se pierde.
	paro := make(chan os.Signal, 1)
	signal.Notify(paro, os.Interrupt, syscall.SIGTERM)

	errores := make(chan error, 1)
	go func() {
		dir := fmt.Sprintf(":%d", cfg.Servidor.Puerto)
		// El tipo y no la raiz: con r2 la raiz no se usa, y ver "./datos/media"
		// en el arranque de un servidor que escribe en un bucket es la clase de
		// linea que hace perder media hora buscando en el sitio equivocado.
		almacen := cfg.Almacen.Tipo
		if almacen == "" {
			almacen = "disco"
		}
		log.Info("escuchando", "puerto", cfg.Servidor.Puerto,
			"lectura_sincrona", cfg.Servidor.LeerCartaSincrono,
			"almacen", almacen)
		errores <- app.Fiber.Listen(dir)
	}()

	select {
	case err := <-errores:
		return err
	case s := <-paro:
		log.Info("apagando", "senal", s.String())
	}

	// Margen para que las peticiones en vuelo terminen. Importa mas de lo
	// normal aqui: una lectura de carta en curso ya se pago, y cortarla a mitad
	// es tirar dinero que ya se gasto.
	ctxApagado, cancelarApagado := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelarApagado()
	app.Cerrar(ctxApagado)

	log.Info("apagado")
	return nil
}
