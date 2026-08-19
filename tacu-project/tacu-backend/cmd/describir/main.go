// describir rellena en lote las filas del banco de platos que solo tienen
// nombre e ingredientes.
//
// La cosecha de la migracion 00007 trajo 419 platos peruanos con sus
// ingredientes y ni una linea de como se VEN, que es lo unico que decide la
// foto. El sistema las va rellenando solo segun aparecen en cartas reales; esto
// hace de una vez las que quedan.
//
// GASTA DINERO: es una llamada de texto por lote. Medido, el banco entero son
// centimos, pero no se lanza "para ver".
//
//	go run ./cmd/describir -n 5              # una prueba corta
//	go run ./cmd/describir -n 0              # todas las que falten
//	go run ./cmd/describir -lote 30 -n 0
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
	"tacu-backend/internal/platform/db"
)

func main() {
	cuantos := flag.Int("n", 5, "cuantos platos describir en total; 0 = todos los que falten")
	lote := flag.Int("lote", 20, "cuantos van en cada llamada al modelo")
	rutaConfig := flag.String("config", "config/config.yaml", "ruta de la configuracion")
	flag.Parse()

	if err := correr(*rutaConfig, *cuantos, *lote); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func correr(rutaConfig string, cuantos, lote int) error {
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancelar()

	cfg, err := config.Cargar(rutaConfig)
	if err != nil {
		return err
	}
	if cfg.BD.DSN == "" {
		return fmt.Errorf("falta TACU_BD_DSN")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	pool, err := db.Abrir(ctx, cfg.BD.DSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := postgres.NuevoRepo(pool)

	// LibroNulo: esto no es de un restaurante, asi que su gasto no se le carga a
	// ninguna importacion. Se imprime aqui y ya.
	ia, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.ConocerPlatos)
	if err != nil {
		return err
	}
	conocer := app.NuevoConocer(ia, repo)

	descritos, gasto := 0, 0.0
	for cuantos == 0 || descritos < cuantos {
		pedir := lote
		if cuantos > 0 && cuantos-descritos < pedir {
			pedir = cuantos - descritos
		}

		pendientes, err := repo.PendientesDeDescribir(ctx, pedir)
		if err != nil {
			return err
		}
		if len(pendientes) == 0 {
			fmt.Println("no queda nada por describir")
			break
		}

		n, uso, err := conocer.Describir(ctx, pendientes)
		gasto += uso.CostoUSD
		if err != nil {
			return err
		}

		descritos += n
		fmt.Printf("%3d/%d  %-28s  $%.4f acumulado\n",
			descritos, cuantos, pendientes[0].Clave+"...", gasto)

		// Si un lote entero se cae al suelo, no hay que seguir pagando lotes: la
		// siguiente consulta devolveria los mismos y esto giraria para siempre.
		if n == 0 {
			return fmt.Errorf("el lote de %d no dejo ninguno usable; se para aqui", len(pendientes))
		}
	}

	fmt.Printf("\ndescritos: %d    costo: $%.4f\n", descritos, gasto)
	return nil
}
