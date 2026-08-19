package postgres_test

import (
	"context"
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

// EL BUG QUE BLOQUEABA PUBLICAR: el dueno escribia "Personal"/"Familiar" y el
// aviso seguia ahi porque el texto vivia solo en la memoria de React.
//
// Las dos mitades importan: el plato deja de estar marcado Y el recuento baja.
func TestPonerleNombreALosPreciosApagaLaMarcaYBajaElRecuento(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	// Un trio real: dos precios, ninguna etiqueta.
	trio := domain.Plato{
		Nombre: "Ceviche + Arroz c/ Mariscos + Chicharrón Mixto",
		Precios: []domain.Precio{
			{Texto: "S/ 45", Centimos: 4500},
			{Texto: "S/ 80", Centimos: 8000},
		},
	}
	limpio := domain.Plato{
		Nombre:  "Ceviche de Pescado",
		Precios: []domain.Precio{{Texto: "S/ 30", Centimos: 3000}},
	}

	carta := domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "Tríos", Platos: []domain.Plato{trio}},
		{Nombre: "Ceviches", Platos: []domain.Plato{limpio}},
	}}
	marcas := carta.Verificar()
	if marcas.Revisar != 1 {
		t.Fatalf("de partida deberia haber 1 plato marcado, hay %d", marcas.Revisar)
	}
	if err := r.GuardarCarta(ctx, impID, carta, nil, marcas); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	var trioID = imp.Carta.Categorias[0].Platos[0].ID

	plato, nuevas, err := r.EditarPlato(ctx, trioID, []string{"Personal", "Familiar"})
	if err != nil {
		t.Fatal(err)
	}

	if plato.Revisar != domain.SinRevision {
		t.Errorf("el plato sigue marcado con %q despues de ponerle nombre a los precios", plato.Revisar)
	}
	if nuevas.Revisar != 0 {
		t.Errorf("el recuento sigue en %d: la barra de publicar no se desbloquearia", nuevas.Revisar)
	}

	// Y tiene que estar en la base, no solo en la respuesta.
	imp, _ = r.Obtener(ctx, impID)
	guardado := imp.Carta.Categorias[0].Platos[0]
	if guardado.Precios[0].Etiqueta != "Personal" || guardado.Precios[1].Etiqueta != "Familiar" {
		t.Errorf("las etiquetas no se guardaron: %+v", guardado.Precios)
	}
	if imp.Marcas.Revisar != 0 {
		t.Errorf("la importacion sigue con %d por revisar", imp.Marcas.Revisar)
	}
	if !imp.PuedePublicarse() {
		t.Error("la carta deberia poder publicarse ya")
	}
}

// Con un solo precio nombrado el cliente sigue sin saber cual pedir.
func TestUnaSolaEtiquetaNoAlcanza(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	carta := domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Tríos",
		Platos: []domain.Plato{{
			Nombre: "Trio Marino",
			Precios: []domain.Precio{
				{Texto: "S/ 45", Centimos: 4500},
				{Texto: "S/ 80", Centimos: 8000},
			},
		}},
	}}}
	if err := r.GuardarCarta(ctx, impID, carta, nil, carta.Verificar()); err != nil {
		t.Fatal(err)
	}
	imp, _ := r.Obtener(ctx, impID)

	plato, marcas, err := r.EditarPlato(ctx, imp.Carta.Platos()[0].ID, []string{"Personal"})
	if err != nil {
		t.Fatal(err)
	}
	if plato.Revisar != domain.VariantesSinNombre {
		t.Errorf("con un solo precio nombrado el plato deberia seguir marcado, dio %q", plato.Revisar)
	}
	if marcas.Revisar != 1 {
		t.Errorf("el recuento deberia seguir en 1, dio %d", marcas.Revisar)
	}
}
