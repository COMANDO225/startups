package main

import (
	"context"
	"fmt"
	"os"

	"facturacion-service/internal/core"
)

func main() {
	app, err := core.New(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error de arranque: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error en ejecucion: %v\n", err)
		os.Exit(1)
	}
}
