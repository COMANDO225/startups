package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/almacen"
	"tacu-backend/internal/platform/config"
)

// El flujo entero de la etapa 1, con la IA de verdad: se sube galponcito.jpeg,
// se lee, y tienen que quedar 42 platos en la base con sus precios.
//
// GASTA DINERO (~$0.02 por corrida), asi que solo corre con TACU_E2E=1. Un
// `go test ./...` normal no puede cobrarle a nadie sin avisar.
//
//	docker compose up -d
//	set -a; source .env; set +a
//	TACU_E2E=1 go test ./internal/modules/carta/adapters/postgres/ -run E2E -v
func TestE2EImportarGalponcito(t *testing.T) {
	if os.Getenv("TACU_E2E") != "1" {
		t.Skip("gasta dinero: TACU_E2E=1 para correrlo")
	}

	r, _ := repo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	raiz := filepath.Join("..", "..", "..", "..", "..")
	foto, err := os.ReadFile(filepath.Join(raiz, "testdata", "cartas", "galponcito.jpeg"))
	if err != nil {
		t.Skipf("sin la carta de prueba: %v", err)
	}

	alm, err := almacen.NuevoDisco(t.TempDir(), "/media")
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Cargar(filepath.Join(raiz, "config", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ia, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.LeerCarta)
	if err != nil {
		t.Skipf("sin credenciales de IA: %v", err)
	}

	// --- 1. sube la carta ---
	importar := app.NuevoImportar(r, alm, dinero.USD(cfg.IA.PresupuestoPorImportacionUSD))
	b, err := importar.Ejecutar(ctx, "Pollos Galponcito", nil,
		[]app.Imagen{{Bytes: foto, MIME: "image/jpeg"}}, nil)
	if err != nil {
		t.Fatalf("Importar: %v", err)
	}
	if b.Token == "" {
		t.Fatal("no devolvio token: el dueno no podria volver a su borrador")
	}
	if b.Estado != domain.Leyendo {
		t.Fatalf("estado inicial = %q, esperaba leyendo", b.Estado)
	}

	// --- 2. la lee (esto es lo que hara el worker de River) ---
	inicio := time.Now()
	res, err := app.NuevoLeer(ia).Ejecutar(ctx, []ai.Imagen{{Bytes: foto, MIME: "image/jpeg"}})
	if err != nil {
		t.Fatalf("Leer: %v", err)
	}
	cruda, _ := json.Marshal(res.Carta)
	if err := r.GuardarCarta(ctx, b.ID, res.Carta, cruda, res.Marcas); err != nil {
		t.Fatalf("GuardarCarta: %v", err)
	}
	if err := r.AnotarGasto(ctx, &b.ID, string(ai.LeerCarta), res.Uso.Modelo,
		res.Uso.TokensEntrada, res.Uso.TokensSalida, res.Uso.Imagenes,
		dinero.USD(res.Uso.CostoUSD), res.Uso.Intentos); err != nil {
		t.Fatalf("AnotarGasto: %v", err)
	}

	// --- 3. lo que quedo en la base ---
	imp, err := r.Obtener(ctx, b.ID)
	if err != nil {
		t.Fatalf("Obtener: %v", err)
	}

	platos := imp.Carta.Platos()
	t.Logf("%d platos en %d categorias · %.1fs · %s · %s",
		len(platos), len(imp.Carta.Categorias), time.Since(inicio).Seconds(),
		imp.Gastado, res.Uso.Modelo)

	if imp.Estado != domain.Lista {
		t.Fatalf("estado = %q, esperaba lista", imp.Estado)
	}
	if len(platos) != 42 {
		t.Errorf("%d platos, esperaba 42 (la carta esta medida en cmd/cartabench)", len(platos))
	}

	// Ni un plato sin nombre, sin precio o sin id: los tres romperian la
	// pantalla, y los tres se detectan aqui y no cuando el dueno la abre.
	for _, p := range platos {
		if p.ID.String() == "" || p.Nombre == "" {
			t.Errorf("plato incompleto: %+v", p)
		}
		if len(p.Precios) == 0 {
			t.Errorf("el plato %q volvio sin precios", p.Nombre)
		}
		if p.Foto.Estado != domain.SinFoto {
			t.Errorf("el plato %q deberia nacer sin foto, esta en %q", p.Nombre, p.Foto.Estado)
		}
	}

	// Los 19 precios manuscritos de Galponcito van a confirmar, no a revisar:
	// es la distincion que evita dejar media carta en rojo.
	if imp.Marcas.Revisar != 0 {
		t.Errorf("%d platos marcados como que no cuadran, esperaba 0", imp.Marcas.Revisar)
		for _, p := range imp.Carta.ParaRevisar() {
			t.Logf("   no cuadra: %s (%s)", p.Nombre, p.Revisar.Explicacion())
		}
	}
	if imp.Marcas.Confirmar != 19 {
		t.Errorf("%d por confirmar, esperaba 19 (los stickers amarillos)", imp.Marcas.Confirmar)
	}

	// El costo tiene que estar en el rango medido, y muy por debajo del tope.
	if imp.Gastado < dinero.USD(0.005) || imp.Gastado > dinero.USD(0.10) {
		t.Errorf("costo = %s, fuera del rango medido (~$0.017)", imp.Gastado)
	}
	if imp.Gastado > imp.Presupuesto {
		t.Errorf("la sola lectura (%s) se comio el presupuesto (%s)", imp.Gastado, imp.Presupuesto)
	}

	porTarea, err := r.GastoPorTarea(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if porTarea[string(ai.LeerCarta)] != imp.Gastado {
		t.Errorf("el desglose (%s) no cuadra con el acumulado (%s)",
			porTarea[string(ai.LeerCarta)], imp.Gastado)
	}

	// La imagen original quedo guardada: sin ella no se puede reintentar la
	// lectura sin volver a pedirsela al dueno.
	if _, err := os.Stat(filepath.Join(alm.Raiz(), "cartas", b.ID.String(), "1.jpg")); err != nil {
		t.Errorf("la foto de la carta no se guardo: %v", err)
	}
}
