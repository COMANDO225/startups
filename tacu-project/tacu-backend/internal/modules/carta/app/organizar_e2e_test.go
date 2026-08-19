package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"tacu-backend/internal/core"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/platform/ai"
	"tacu-backend/internal/platform/config"
)

// La carta sale de la extraccion en orden de LECTURA y hay que publicarla en
// orden de VENTA. Galponcito empieza por las guarniciones y deja los combos en el
// puesto 5 de 6, cuando la carta fisica les da media hoja en recuadros grandes.
//
// MEDIDO el 14-ago-2026:
//
//	ANTES   Porciones y Chaufas · EN MESA · PARA LLEVAR · BEBIDAS · COMBOS · Otros Platos
//	DESPUES Combos · En mesa · Para llevar · Otros platos · Porciones y chaufas · Bebidas
//	$0.0011 con gemini-3.7-flash
//
// De paso arreglo el grito: "EN MESA" -> "En mesa".
//
// GASTA DINERO: solo corre con TACU_E2E=1.
func TestE2EOrganizarGalponcito(t *testing.T) {
	if os.Getenv("TACU_E2E") != "1" {
		t.Skip("gasta dinero: TACU_E2E=1 para correrlo")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	foto, err := os.ReadFile("../../../../testdata/cartas/galponcito.jpeg")
	if err != nil {
		t.Skip(err)
	}
	cfg, err := config.Cargar("../../../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ia, err := core.ArmarIA(ctx, cfg.IA, ai.LibroNulo{}, log, ai.LeerCarta, ai.OrganizarCarta)
	if err != nil {
		t.Skip(err)
	}

	res, err := app.NuevoLeer(ia).Ejecutar(ctx, []ai.Imagen{{Bytes: foto, MIME: "image/jpeg"}})
	if err != nil {
		t.Fatal(err)
	}
	antes := []string{}
	for _, c := range res.Carta.Categorias {
		antes = append(antes, c.Nombre)
	}

	nueva, uso, err := app.NuevoOrganizar(ia).Ejecutar(ctx, res.Carta)
	if err != nil {
		t.Fatalf("Organizar: %v", err)
	}
	despues := []string{}
	for _, c := range nueva.Categorias {
		despues = append(despues, c.Nombre)
	}

	a, _ := json.Marshal(antes)
	d, _ := json.Marshal(despues)
	t.Logf("ANTES   %s", a)
	t.Logf("DESPUES %s", d)
	t.Logf("costo   $%.5f  %s", uso.CostoUSD, uso.Modelo)

	if len(nueva.Platos()) != len(res.Carta.Platos()) {
		t.Fatalf("se perdieron platos: %d -> %d", len(res.Carta.Platos()), len(nueva.Platos()))
	}
	if len(nueva.Categorias) != len(res.Carta.Categorias) {
		t.Fatalf("se perdieron categorias: %d -> %d", len(res.Carta.Categorias), len(nueva.Categorias))
	}

	// Lo que de verdad se busca: los combos arriba y las bebidas abajo.
	if despues[0] != "Combos" {
		t.Errorf("la primera categoria es %q; los combos son lo que sube el ticket", despues[0])
	}
	if despues[len(despues)-1] != "Bebidas" {
		t.Errorf("la ultima es %q, esperaba Bebidas", despues[len(despues)-1])
	}
}
