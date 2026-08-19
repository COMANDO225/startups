package postgres_test

import (
	"context"
	"testing"
)

// AgregarPlatos es lo contrario de GuardarCarta: aquella empieza borrando, y con
// los platos se irian las fotos generadas y las etiquetas corregidas. Esto SUMA.
func TestGuardarImagenesReescribeLasPaginas(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	claves := []string{"cartas/x/2.jpg", "cartas/x/1.jpg"}
	if err := r.GuardarImagenes(ctx, impID, claves); err != nil {
		t.Fatalf("GuardarImagenes: %v", err)
	}

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatalf("Obtener: %v", err)
	}
	if len(imp.Imagenes) != 2 || imp.Imagenes[0] != "cartas/x/2.jpg" {
		t.Fatalf("imagenes = %v, el orden es el que puso el dueno", imp.Imagenes)
	}
}
