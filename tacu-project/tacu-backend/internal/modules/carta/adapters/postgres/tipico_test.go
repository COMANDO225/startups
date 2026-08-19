package postgres_test

import (
	"context"
	"strings"
	"testing"

	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
)

// El banco tiene que llegar al dominio con lo que decide la foto. Si un dia
// alguien vacia la tabla o se le cae un campo por el camino, esto lo dice antes
// que una foto rara.
func TestElBancoLlegaConSuCanonDescrito(t *testing.T) {
	r, _ := repo(t)

	banco, err := r.BancoDePlatos(context.Background())
	if err != nil {
		t.Fatalf("BancoDePlatos: %v", err)
	}
	if len(banco) < 100 {
		t.Fatalf("%d platos en el banco: la cosecha de la 00007 son 419", len(banco))
	}

	descritos := 0
	for _, p := range banco {
		if !p.Descrito() {
			continue
		}
		descritos++
		if len(p.Patrones) == 0 {
			t.Errorf("%q esta descrito y no tiene patrones: no lo va a emparejar nadie", p.Clave)
		}
		if strings.TrimSpace(p.Jamas) == "" {
			t.Errorf("%q no dice lo que NO es, que es la mitad que corrige al modelo", p.Clave)
		}
	}
	if descritos < 20 {
		t.Errorf("solo %d platos descritos; el canon de la 00008 son 24", descritos)
	}
}

// EL ARREGLO, de punta a punta y contra la base de verdad: una gaseosa deja de
// salir con camote y choclo al lado.
//
// Va aqui y no en el dominio porque lo que se prueba es el camino entero —
// emparejar al leer, guardar la clave, y que ReclamarFoto la traiga de vuelta
// plegada en la base—. En el dominio ya hay un test del pliegue; este comprueba
// que la clave sobrevive el viaje por Postgres.
func TestUnaGaseosaNoSaleConLaGuarnicionDeLaCevicheria(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	banco, err := r.BancoDePlatos(ctx)
	if err != nil {
		t.Fatalf("BancoDePlatos: %v", err)
	}

	carta := domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Carta",
		Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado", Precios: []domain.Precio{{Texto: "S/ 30", Centimos: 3000}}},
			{Nombre: "Gaseosa 1L.", Precios: []domain.Precio{{Texto: "S/ 10", Centimos: 1000}}},
		},
	}}}
	if n := carta.EmparejarConElBanco(banco); n != 2 {
		t.Fatalf("emparejo %d de 2 platos", n)
	}
	if err := r.GuardarCarta(ctx, impID, carta, nil, domain.Marcas{}); err != nil {
		t.Fatalf("GuardarCarta: %v", err)
	}

	// El negocio es una cevicheria: es su guarnicion la que antes se le pegaba a
	// todo, incluida la gaseosa.
	if err := r.GuardarTipos(ctx, impID, []domain.Tipo{domain.Cevicheria}); err != nil {
		t.Fatalf("GuardarTipos: %v", err)
	}

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatalf("Obtener: %v", err)
	}

	prompts := map[string]string{}
	for _, p := range imp.Carta.Platos() {
		if p.Tipico == "" {
			t.Fatalf("%q se guardo sin clave del banco", p.Nombre)
		}
		if err := r.MarcarFotoConEstado(ctx, p.ID, domain.FotoPendiente); err != nil {
			t.Fatal(err)
		}
		encargo, mio, err := r.ReclamarFoto(ctx, p.ID, impID)
		if err != nil || !mio {
			t.Fatalf("ReclamarFoto de %q: mio=%v err=%v", p.Nombre, mio, err)
		}
		prompts[p.Nombre] = strings.ToLower(
			app.PromptFoto(encargo.Plato, encargo.Tipos, encargo.Base))
	}

	gaseosa := prompts["Gaseosa 1L."]
	if strings.Contains(gaseosa, "sweet potato") {
		t.Error("la gaseosa sigue saliendo con camote al lado")
	}
	if !strings.Contains(gaseosa, "bottle") && !strings.Contains(gaseosa, "glass") {
		t.Error("una bebida tiene que salir en vaso o botella, no en plato")
	}

	// Y el ceviche SI lo lleva: el arreglo no puede consistir en quitarle la
	// guarnicion a todo el mundo.
	ceviche := prompts["Ceviche de Pescado"]
	if !strings.Contains(ceviche, "sweet potato") {
		t.Error("el ceviche se quedo sin camote")
	}
	if !strings.Contains(ceviche, "never cooked") {
		t.Error("al ceviche no le llego lo que NO es, que es lo que evita que salga cocido")
	}
}

// Reconocer es lo que arrastra hacia atras lo que el banco aprendio despues. La
// invariante que no se negocia: NINGUN id de plato cambia, porque de esos ids
// cuelgan las fotos ya pagadas.
func TestActualizarTipicosNoCambiaNingunIdDePlato(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	// Se guarda SIN emparejar, que es como quedaron las cartas leidas antes de
	// que el banco existiera.
	carta := domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Carta",
		Platos: []domain.Plato{
			{Nombre: "Ceviche de Pescado", Precios: []domain.Precio{{Texto: "S/ 30", Centimos: 3000}}},
			{Nombre: "Gaseosa 1L.", Precios: []domain.Precio{{Texto: "S/ 10", Centimos: 1000}}},
		},
	}}}
	if err := r.GuardarCarta(ctx, impID, carta, nil, domain.Marcas{}); err != nil {
		t.Fatalf("GuardarCarta: %v", err)
	}

	antes, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range antes.Carta.Platos() {
		if p.Tipico != "" {
			t.Fatalf("%q nacio emparejado; el test no prueba nada", p.Nombre)
		}
		ids[p.ID.String()] = true
	}

	banco, err := r.BancoDePlatos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reconocida := antes.Carta
	if n := reconocida.EmparejarConElBanco(banco); n != 2 {
		t.Fatalf("emparejo %d de 2", n)
	}
	if err := r.ActualizarTipicos(ctx, reconocida.Platos()); err != nil {
		t.Fatalf("ActualizarTipicos: %v", err)
	}

	despues, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range despues.Carta.Platos() {
		if !ids[p.ID.String()] {
			t.Errorf("%q cambio de id: las fotos que colgaban de el se quedaron huerfanas", p.Nombre)
		}
		if p.Tipico == "" {
			t.Errorf("%q sigue sin clave del banco", p.Nombre)
		}
	}
}

// Lo que el dueno corrigio a mano no lo pisa una ampliacion del banco.
func TestReconocerNoPisaLaCorreccionDelDueno(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	carta := domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Carta",
		Platos: []domain.Plato{{
			Nombre:       "Ceviche de Pescado",
			Precios:      []domain.Precio{{Texto: "S/ 30", Centimos: 3000}},
			Tipico:       "jalea",
			TipicoOrigen: domain.TipicoDelDueno,
		}},
	}}}
	if err := r.GuardarCarta(ctx, impID, carta, nil, domain.Marcas{}); err != nil {
		t.Fatalf("GuardarCarta: %v", err)
	}

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	banco, err := r.BancoDePlatos(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Ni el emparejado en memoria...
	reconocida := imp.Carta
	reconocida.EmparejarConElBanco(banco)
	if p := reconocida.Platos()[0]; p.Tipico != "jalea" {
		t.Errorf("el emparejado piso la correccion del dueno: %q", p.Tipico)
	}

	// ...ni el UPDATE, que tiene su propia guarda por si alguien llama directo.
	forzado := imp.Carta.Platos()
	forzado[0].Tipico, forzado[0].TipicoOrigen = "ceviche", domain.TipicoPorRegla
	if err := r.ActualizarTipicos(ctx, forzado); err != nil {
		t.Fatal(err)
	}
	final, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	if p := final.Carta.Platos()[0]; p.Tipico != "jalea" {
		t.Errorf("el UPDATE piso la correccion del dueno: %q", p.Tipico)
	}
}
