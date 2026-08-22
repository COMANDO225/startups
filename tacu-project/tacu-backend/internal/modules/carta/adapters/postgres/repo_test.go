package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/adapters/postgres"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/db"
)

const dsnPorDefecto = "postgres://tacu:tacu@localhost:5433/tacu?sslmode=disable"

// banco es el repo con su pool al lado, para que borrador() pueda borrar lo que
// crea. Va embebido, asi que todo lo que se llamaba sobre *postgres.Repo se
// sigue llamando igual y ningun test cambia.
type banco struct {
	*postgres.Repo
	pool *pgxpool.Pool
}

func repo(t *testing.T) (*banco, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TACU_BD_DSN")
	if dsn == "" {
		dsn = dsnPorDefecto
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.Abrir(ctx, dsn, 4)
	if err != nil {
		t.Skipf("sin Postgres en %s (docker compose up -d): %v", dsn, err)
	}
	t.Cleanup(pool.Close)
	return &banco{Repo: postgres.NuevoRepo(pool), pool: pool}, pool
}

// limpiarAlTerminar borra el restaurante de una importacion cuando acaba el test.
//
// Hace falta porque estos tests hablan con la base de DESARROLLO: el DSN por
// defecto de aqui es el MISMO que usa el servidor, y sin esto cada corrida del
// paquete se dejaba 26 restaurantes dentro. Medido antes de arreglarlo: 1836 de
// los 1850 que habia eran de estos tests, o sea que la base de desarrollo era
// 99% basura y cada vez se parecia menos a algo real.
//
// El cleanup se registra DESPUES del pool.Close de repo(), y por eso corre
// antes: t.Cleanup es LIFO.
func limpiarAlTerminar(t *testing.T, pool *pgxpool.Pool, importacionID id.ID) {
	t.Helper()
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(),
			"DELETE FROM restaurante WHERE id = (SELECT restaurante_id FROM importacion WHERE id = $1)",
			importacionID)
		if err != nil {
			// Falla el test a proposito: una limpieza que no limpia en silencio
			// es como llegamos a los 1836.
			t.Errorf("limpiando lo que creo este test: %v", err)
		}
	})
}

func borrador(t *testing.T, r *banco) id.ID {
	t.Helper()
	ctx := context.Background()

	restauranteID, importacionID := id.Nuevo(), id.Nuevo()
	ip := netip.MustParseAddr("190.234.1.1")

	err := r.CrearBorrador(ctx, restauranteID, importacionID,
		"Pollos Galponcito", nil, []byte("hash-de-prueba"), &ip,
		domain.Leyendo, []string{"cartas/x/1.jpg"}, dinero.USD(3.00))
	if err != nil {
		t.Fatalf("CrearBorrador: %v", err)
	}
	limpiarAlTerminar(t, r.pool, importacionID)
	return importacionID
}

// La carta de Galponcito, reducida a lo que ejercita el mapeo: dos categorias,
// un plato con dos precios etiquetados, uno con precio manuscrito y uno marcado
// para revision.
func cartaDePrueba() domain.Carta {
	return domain.Carta{Categorias: []domain.Categoria{
		{Nombre: "EN MESA", Platos: []domain.Plato{
			{
				Nombre:  "1/4 pollo",
				Precios: []domain.Precio{{Texto: "s/ 11.00", Centimos: 1100, Procedencia: domain.Impreso}},
			},
			{
				Nombre:      "Mostrito",
				Descripcion: "pollo + papas + chaufa",
				Precios:     []domain.Precio{{Texto: "s/ 9.00", Centimos: 900, Procedencia: domain.Impreso}},
			},
		}},
		{Nombre: "PARA LLEVAR", Platos: []domain.Plato{
			{
				Nombre:  "1/4 pollo",
				Precios: []domain.Precio{{Texto: "13.00", Centimos: 1300, Procedencia: domain.Manuscrito}},
				Revisar: domain.PrecioManuscrito,
			},
			{
				Nombre: "Trio marino",
				Precios: []domain.Precio{
					{Etiqueta: "personal", Texto: "S/ 45", Centimos: 4500},
					{Etiqueta: "fuente", Texto: "S/ 80", Centimos: 8000},
				},
			},
		}},
	}}
}

func TestGuardarYLeerUnaCartaCompleta(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	carta := cartaDePrueba()
	cruda, _ := json.Marshal(map[string]string{"lo": "que dijo el modelo"})
	marcas := domain.Marcas{Revisar: 0, Confirmar: 1}

	if err := r.GuardarCarta(ctx, impID, carta, cruda, marcas); err != nil {
		t.Fatalf("GuardarCarta: %v", err)
	}

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatalf("Obtener: %v", err)
	}

	if imp.Estado != domain.Lista {
		t.Errorf("estado = %q, esperaba lista", imp.Estado)
	}
	if imp.Restaurante.Nombre != "Pollos Galponcito" {
		t.Errorf("restaurante = %q", imp.Restaurante.Nombre)
	}
	if imp.Marcas != marcas {
		t.Errorf("marcas = %+v, esperaba %+v", imp.Marcas, marcas)
	}
	if imp.Presupuesto != dinero.USD(3.00) {
		t.Errorf("presupuesto = %s, esperaba $3.00", imp.Presupuesto)
	}

	// El orden de categorias y de platos DENTRO de cada una tiene que
	// sobrevivir la ida y vuelta: es el orden del catalogo, no el de insercion.
	if n := len(imp.Carta.Categorias); n != 2 {
		t.Fatalf("%d categorias, esperaba 2", n)
	}
	if imp.Carta.Categorias[0].Nombre != "EN MESA" || imp.Carta.Categorias[1].Nombre != "PARA LLEVAR" {
		t.Fatalf("las categorias salieron en otro orden: %q, %q",
			imp.Carta.Categorias[0].Nombre, imp.Carta.Categorias[1].Nombre)
	}
	if n := len(imp.Carta.Platos()); n != 4 {
		t.Fatalf("%d platos, esperaba 4", n)
	}
}

// Cada plato tiene que salir con un id propio y estable: sin el no hay
// PATCH /platos/:id ni foto que colgarle.
func TestCadaPlatoSaleConSuID(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	if err := r.GuardarCarta(ctx, impID, cartaDePrueba(), nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}
	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}

	vistos := map[id.ID]bool{}
	for _, p := range imp.Carta.Platos() {
		if p.ID == id.Nulo {
			t.Fatalf("el plato %q salio sin id", p.Nombre)
		}
		if vistos[p.ID] {
			t.Fatalf("dos platos con el mismo id: %s", p.ID)
		}
		vistos[p.ID] = true
	}

	// Y el mismo id en la siguiente lectura: si cambiara, cada poll del front
	// invalidaria las tarjetas.
	otra, _ := r.Obtener(ctx, impID)
	for i, p := range otra.Carta.Platos() {
		if p.ID != imp.Carta.Platos()[i].ID {
			t.Fatalf("el id del plato %q cambio entre lecturas", p.Nombre)
		}
	}
}

// Los precios son JSONB: si el mapeo pierde la etiqueta o la procedencia, el
// dueno ve dos precios sin saber cual es cual, o un manuscrito como impreso.
func TestLosPreciosSobrevivenEnteros(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	if err := r.GuardarCarta(ctx, impID, cartaDePrueba(), nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}
	imp, _ := r.Obtener(ctx, impID)

	var trio, manuscrito *domain.Plato
	for _, p := range imp.Carta.Platos() {
		switch {
		case p.Nombre == "Trio marino":
			trio = &p
		case p.Revisar == domain.PrecioManuscrito:
			manuscrito = &p
		}
	}

	if trio == nil {
		t.Fatal("no volvio el plato con dos precios")
	}
	if n := len(trio.Precios); n != 2 {
		t.Fatalf("el trio volvio con %d precios, esperaba 2", n)
	}
	if trio.Precios[0].Etiqueta != "personal" || trio.Precios[1].Etiqueta != "fuente" {
		t.Errorf("las etiquetas se perdieron: %+v", trio.Precios)
	}
	if trio.Precios[0].Centimos != 4500 || trio.Precios[1].Centimos != 8000 {
		t.Errorf("los montos se perdieron: %+v", trio.Precios)
	}
	if trio.Desde() != 4500 {
		t.Errorf("Desde() = %s, esperaba S/ 45.00", trio.Desde())
	}

	if manuscrito == nil {
		t.Fatal("no volvio el plato marcado")
	}
	if manuscrito.Precios[0].Procedencia != domain.Manuscrito {
		t.Error("la procedencia manuscrita se perdio: se publicaria como precio impreso")
	}
}

// Reintentar el job de lectura no puede dejar los platos de la corrida anterior
// mezclados con los nuevos. Por eso GuardarCarta borra antes de insertar.
func TestGuardarDosVecesNoDuplicaPlatos(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	for range 3 {
		if err := r.GuardarCarta(ctx, impID, cartaDePrueba(), nil, domain.Marcas{}); err != nil {
			t.Fatal(err)
		}
	}

	imp, _ := r.Obtener(ctx, impID)
	if n := len(imp.Carta.Platos()); n != 4 {
		t.Fatalf("%d platos tras tres guardados, esperaba 4", n)
	}
}

// Mientras lee todavia no hay platos: la pantalla pinta esqueletos con esto.
func TestUnaImportacionRecienCreadaEstaLeyendo(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Estado != domain.Leyendo {
		t.Fatalf("estado = %q, esperaba leyendo", imp.Estado)
	}
	if n := len(imp.Carta.Categorias); n != 0 {
		t.Fatalf("trajo %d categorias antes de leer la carta", n)
	}
	if imp.Presupuesto != dinero.USD(3.00) || imp.Gastado != 0 || imp.Reservado != 0 {
		t.Errorf("presupuesto mal inicializado: %+v", imp)
	}
}

func TestObtenerLoQueNoExiste(t *testing.T) {
	r, _ := repo(t)

	_, err := r.Obtener(context.Background(), id.Nuevo())
	if !errors.Is(err, postgres.ErrNoExiste) {
		t.Fatalf("err = %v, esperaba ErrNoExiste: sin eso el borde devuelve 500 en vez de 404", err)
	}
}

func TestMarcarFallidaGuardaElMotivo(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	const motivo = "la cadena completa fallo para leer_carta"
	if err := r.MarcarFallida(ctx, impID, motivo); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	if imp.Estado != domain.Fallida {
		t.Fatalf("estado = %q, esperaba fallida", imp.Estado)
	}
	if imp.Error != motivo {
		t.Errorf("error = %q, esperaba %q", imp.Error, motivo)
	}
}

// El gasto es el unit economics del producto: si se anota el detalle pero no el
// acumulado, el presupuesto miente.
func TestElGastoSeAnotaYSeAcumula(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	// Una lectura de carta y tres fotos, con los costos reales medidos.
	if err := r.AnotarGasto(ctx, &impID, "leer_carta", "gemini/gemini-3.7-flash",
		1756, 2870, 0, dinero.USD(0.017), 1); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := r.AnotarGasto(ctx, &impID, "generar_foto", "gemini/gemini-3.1-flash-lite-image",
			0, 0, 1, dinero.USD(0.0336), 1); err != nil {
			t.Fatal(err)
		}
	}

	imp, _ := r.Obtener(ctx, impID)
	esperado := dinero.USD(0.017) + 3*dinero.USD(0.0336)
	if imp.Gastado != esperado {
		t.Fatalf("gastado = %s, esperaba %s", imp.Gastado, esperado)
	}

	porTarea, err := r.GastoPorTarea(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	if porTarea["leer_carta"] != dinero.USD(0.017) {
		t.Errorf("leer_carta = %s", porTarea["leer_carta"])
	}
	if porTarea["generar_foto"] != 3*dinero.USD(0.0336) {
		t.Errorf("generar_foto = %s, esperaba %s", porTarea["generar_foto"], 3*dinero.USD(0.0336))
	}
}

// Los CLIs de laboratorio gastan sin pertenecer a ninguna importacion. Ese gasto
// es real y tiene que quedar registrado igual, no perderse.
func TestElGastoSinImportacionSeRegistraIgual(t *testing.T) {
	r, _ := repo(t)

	if err := r.AnotarGasto(context.Background(), nil, "generar_foto",
		"gemini/gemini-3.1-flash-lite-image", 0, 0, 1, dinero.USD(0.0336), 1); err != nil {
		t.Fatalf("un gasto sin importacion tiene que registrarse: %v", err)
	}
}

func TestElTokenHashVuelveIntacto(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	h, err := r.TokenHash(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}
	if string(h) != "hash-de-prueba" {
		t.Fatalf("hash = %q", h)
	}

	if _, err := r.TokenHash(ctx, id.Nuevo()); !errors.Is(err, postgres.ErrNoExiste) {
		t.Errorf("err = %v, esperaba ErrNoExiste", err)
	}
}
