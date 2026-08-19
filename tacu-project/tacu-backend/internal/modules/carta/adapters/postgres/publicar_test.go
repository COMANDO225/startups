package postgres_test

import (
	"context"
	"testing"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// slugUnico da una base distinta en cada corrida.
//
// El slug es UNICO en toda la base y estos tests no la limpian: con un literal,
// pasaban solos y fallaban en la suite entera porque una corrida anterior ya
// habia dejado ese nombre tomado. Lo que se prueba es el SUFIJO, y para eso da
// igual cual sea la base.
//
// Se usa la COLA del uuid, no la cabeza: en un UUIDv7 los primeros caracteres
// son el TIMESTAMP y solo cambian cada ~65 s, asi que dos tests de la misma
// corrida sacaban la misma base y el primero ya salia con sufijo.
func slugUnico() string {
	s := id.Nuevo().String()
	return "carta-" + s[len(s)-12:]
}

// carta lista arma una importacion sin nada marcado, o sea publicable.
func cartaLimpia() domain.Carta {
	return domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Ceviches",
		Platos: []domain.Plato{{
			Nombre:  "Ceviche de Pescado",
			Precios: []domain.Precio{{Texto: "S/ 30", Centimos: 3000}},
		}},
	}}}
}

func TestPublicarDejaLaCartaVisiblePorSuSlug(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	carta := cartaLimpia()
	if err := r.GuardarCarta(ctx, impID, carta, nil, carta.Verificar()); err != nil {
		t.Fatal(err)
	}

	base := slugUnico()
	slug, err := r.Publicar(ctx, impID, base)
	if err != nil {
		t.Fatal(err)
	}
	if slug != base {
		t.Fatalf("slug %q, esperaba %q sin sufijo", slug, base)
	}

	// La carta publica se lee SIN token y trae los platos.
	publica, err := r.CartaPublica(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	if publica.Estado != domain.Publicada {
		t.Errorf("estado %q, esperaba publicada", publica.Estado)
	}
	if n := len(publica.Carta.Platos()); n != 1 {
		t.Fatalf("la carta publica trae %d platos", n)
	}

	// Y la importacion queda marcada, que es lo que apaga el boton en pantalla.
	imp, _ := r.Obtener(ctx, impID)
	if imp.Estado != domain.Publicada {
		t.Errorf("la importacion quedo en %q", imp.Estado)
	}
}

// DOS RESTAURANTES CON EL MISMO NOMBRE.
//
// "Pollos Galponcito" hay uno en cada barrio de Lima. Sin desambiguar, el
// segundo se estrella contra el UNIQUE del slug, o peor: le roba la URL al
// primero y sus clientes acaban viendo otra carta.
func TestDosRestaurantesConElMismoNombreNoSePisanLaURL(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()

	base := slugUnico()
	publicar := func() string {
		impID := borrador(t, r)
		carta := cartaLimpia()
		if err := r.GuardarCarta(ctx, impID, carta, nil, carta.Verificar()); err != nil {
			t.Fatal(err)
		}
		slug, err := r.Publicar(ctx, impID, base)
		if err != nil {
			t.Fatal(err)
		}
		return slug
	}

	primero, segundo := publicar(), publicar()
	if primero != base {
		t.Errorf("el primero deberia quedarse el slug limpio, dio %q", primero)
	}
	if segundo == primero {
		t.Fatal("el segundo restaurante le robo la URL al primero")
	}
	if segundo != base+"-2" {
		t.Errorf("el segundo dio %q, esperaba %q", segundo, base+"-2")
	}

	// Y cada URL lleva a SU carta.
	for _, s := range []string{primero, segundo} {
		if _, err := r.CartaPublica(ctx, s); err != nil {
			t.Errorf("la carta de %q no se puede leer: %v", s, err)
		}
	}
}

// Republicar el MISMO restaurante no le cambia la URL: el dueno ya la repartio.
func TestRepublicarConservaLaURL(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	carta := cartaLimpia()
	if err := r.GuardarCarta(ctx, impID, carta, nil, carta.Verificar()); err != nil {
		t.Fatal(err)
	}

	base := slugUnico()
	primero, err := r.Publicar(ctx, impID, base)
	if err != nil {
		t.Fatal(err)
	}
	segundo, err := r.Publicar(ctx, impID, base)
	if err != nil {
		t.Fatal(err)
	}
	if primero != segundo {
		t.Fatalf("republicar cambio la URL de %q a %q", primero, segundo)
	}
}
