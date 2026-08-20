package postgres_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// EL TECHO POR RESTAURANTE, MEDIDO BAJO CONCURRENCIA.
//
// Sin el lock, este test falla: en READ COMMITTED varios claims cuentan la misma
// instantanea —que no incluye los UPDATE sin confirmar de los otros— y entran
// todos. Medido antes del arreglo: con el tope en 4, seis generando a la vez.
//
// Se lanzan MAS goroutines que el techo a proposito, y todas a la vez, que es lo
// que un pool de 8 workers hace de verdad con una carta de 60 platos.
func TestElTechoPorRestauranteAguantaConcurrencia(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()

	const platos = 20
	impID := borrador(t, r)

	var cat domain.Categoria
	cat.Nombre = "Segundos"
	for range platos {
		cat.Platos = append(cat.Platos, domain.Plato{
			Nombre:  "plato",
			Precios: []domain.Precio{{Texto: "S/ 20", Centimos: 2000}},
		})
	}
	if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{cat}},
		nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}

	imp, err := r.Obtener(ctx, impID)
	if err != nil {
		t.Fatal(err)
	}

	// Todos a 'pendiente', que es de donde el claim los toma.
	for _, p := range imp.Carta.Platos() {
		if err := r.MarcarFotoConEstado(ctx, p.ID, domain.FotoPendiente); err != nil {
			t.Fatal(err)
		}
	}

	var reclamados atomic.Int32
	var arranque sync.WaitGroup
	var fin sync.WaitGroup
	arranque.Add(1)

	for _, p := range imp.Carta.Platos() {
		fin.Add(1)
		go func(platoID id.ID) {
			defer fin.Done()
			arranque.Wait() // todas salen en el mismo instante
			_, mio, err := r.ReclamarFoto(context.Background(), platoID, impID)
			if err != nil {
				t.Errorf("ReclamarFoto: %v", err)
				return
			}
			if mio {
				reclamados.Add(1)
			}
		}(p.ID)
	}

	arranque.Done()
	fin.Wait()

	// El techo son 4. Ni uno mas, por muchas que salgan a la vez.
	if n := reclamados.Load(); n > 4 {
		t.Fatalf("%d workers reclamaron a la vez con un techo de 4: el techo se filtra", n)
	} else if n != 4 {
		t.Errorf("solo %d reclamaron de 4 posibles: el techo esta frenando de mas", n)
	}

	// Y en la base tiene que verse lo mismo.
	imp, _ = r.Obtener(ctx, impID)
	generando := 0
	for _, p := range imp.Carta.Platos() {
		if p.Foto.Estado == domain.FotoGenerando {
			generando++
		}
	}
	if generando != 4 {
		t.Fatalf("%d platos en generando, esperaba 4", generando)
	}
}

// Dos restaurantes distintos NO se bloquean entre si: el lock es por
// importacion. Si fuera global, la segunda carta esperaria a la primera y el
// techo dejaria de repartir para pasar a serializar.
func TestDosRestaurantesNoSeBloqueanEntreSi(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()

	crear := func() (id.ID, []id.ID) {
		impID := borrador(t, r)
		var cat domain.Categoria
		cat.Nombre = "Segundos"
		for range 6 {
			cat.Platos = append(cat.Platos, domain.Plato{
				Nombre:  "plato",
				Precios: []domain.Precio{{Texto: "S/ 20", Centimos: 2000}},
			})
		}
		if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{cat}},
			nil, domain.Marcas{}); err != nil {
			t.Fatal(err)
		}
		imp, _ := r.Obtener(ctx, impID)
		var ids []id.ID
		for _, p := range imp.Carta.Platos() {
			_ = r.MarcarFotoConEstado(ctx, p.ID, domain.FotoPendiente)
			ids = append(ids, p.ID)
		}
		return impID, ids
	}

	impA, platosA := crear()
	impB, platosB := crear()

	var a, b atomic.Int32
	var wg sync.WaitGroup
	reclamar := func(imp id.ID, platos []id.ID, cuenta *atomic.Int32) {
		defer wg.Done()
		for _, p := range platos {
			if _, mio, err := r.ReclamarFoto(context.Background(), p, imp); err == nil && mio {
				cuenta.Add(1)
			}
		}
	}
	wg.Add(2)
	go reclamar(impA, platosA, &a)
	go reclamar(impB, platosB, &b)
	wg.Wait()

	// Cada uno consigue su cupo completo, independientemente del otro.
	if a.Load() != 4 || b.Load() != 4 {
		t.Fatalf("A reclamo %d y B %d, esperaba 4 cada uno: un restaurante esta frenando al otro",
			a.Load(), b.Load())
	}
}

// LA RESERVA DE PRESUPUESTO, BAJO CONCURRENCIA.
//
// Es lo que impide que 60 workers lean todos "aun queda" y se pasen todos. La
// condicion y el incremento tienen que ser la MISMA sentencia.
func TestElPresupuestoNoSePasaConWorkersConcurrentes(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r) // presupuesto $3.00

	const costo = dinero.MicrosUSD(33_600) // una foto
	caben := int(dinero.USD(3.00) / costo) // 89

	var concedidas atomic.Int32
	var wg sync.WaitGroup
	// Se piden MAS de las que caben, todas a la vez.
	for range caben + 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hay, err := r.ReservarPresupuesto(context.Background(), impID, costo)
			if err != nil {
				t.Errorf("ReservarPresupuesto: %v", err)
				return
			}
			if hay {
				concedidas.Add(1)
			}
		}()
	}
	wg.Wait()

	if n := int(concedidas.Load()); n != caben {
		t.Fatalf("se concedieron %d reservas y caben %d", n, caben)
	}

	imp, _ := r.Obtener(ctx, impID)
	if imp.Reservado > imp.Presupuesto {
		t.Fatalf("reservado %s se paso del presupuesto %s", imp.Reservado, imp.Presupuesto)
	}
}

// "Regenerar" estuvo roto: PlatosGenerables solo miraba 'vacia' y 'error', asi
// que un plato en 'lista' no se elegia nunca. 202 con encoladas=0 y sin error.
func TestRegenerarUnaFotoQueYaEstaLista(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	var cat domain.Categoria
	cat.Nombre = "Fuente Familiar"
	for range 3 {
		cat.Platos = append(cat.Platos, domain.Plato{
			Nombre: "Fuente de Ceviche", Precios: []domain.Precio{{Texto: "S/ 80", Centimos: 8000}},
		})
	}
	if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{cat}},
		nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	platos := imp.Carta.Platos()

	// Los tres ya tienen su foto de IA.
	for _, p := range platos {
		if err := r.MarcarFotoLista(ctx, p.ID, domain.FotoDeIA, "fotos/x/ia.jpg"); err != nil {
			t.Fatal(err)
		}
	}

	sinCola := func(context.Context, pgx.Tx, []id.ID) error { return nil }

	// "Generar todas" NO las vuelve a pagar: ya estan.
	todas, err := r.MarcarPendientes(ctx, impID, nil, sinCola)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 0 {
		t.Fatalf("'generar todas' eligio %d fotos ya hechas: se pagarian dos veces", len(todas))
	}

	// Pedir UNO por su nombre si lo regenera: es una peticion explicita.
	uno, err := r.MarcarPendientes(ctx, impID, []id.ID{platos[0].ID}, sinCola)
	if err != nil {
		t.Fatal(err)
	}
	if len(uno) != 1 || uno[0] != platos[0].ID {
		t.Fatalf("regenerar un plato con foto eligio %v, esperaba [%s]", uno, platos[0].ID)
	}
}

// Los intentos se reinician al encolar: sin eso, corregir el texto dos veces
// dejaba el plato en el tope y el claim lo rechazaba en silencio.
func TestRegenerarVariasVecesNoSeQuedaSinIntentos(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Fuente Familiar",
		Platos: []domain.Plato{{
			Nombre: "Fuente de Ceviche", Precios: []domain.Precio{{Texto: "S/ 80", Centimos: 8000}},
		}},
	}}}, nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	platoID := imp.Carta.Platos()[0].ID
	sinCola := func(context.Context, pgx.Tx, []id.ID) error { return nil }

	// Tres rondas de "corrijo el texto y regenero", que es el uso normal del lapiz.
	for ronda := 1; ronda <= 3; ronda++ {
		elegidos, err := r.MarcarPendientes(ctx, impID, []id.ID{platoID}, sinCola)
		if err != nil {
			t.Fatal(err)
		}
		if len(elegidos) != 1 {
			t.Fatalf("ronda %d: no se encolo la regeneracion", ronda)
		}

		_, mio, err := r.ReclamarFoto(ctx, platoID, impID)
		if err != nil {
			t.Fatal(err)
		}
		if !mio {
			t.Fatalf("ronda %d: el claim rechazo una regeneracion pedida a mano", ronda)
		}

		if err := r.MarcarFotoLista(ctx, platoID, domain.FotoDeIA, "fotos/x/ia.jpg"); err != nil {
			t.Fatal(err)
		}
	}
}

// Una foto que subio el dueno NO la pisa "generar todas". Es trabajo suyo y es
// mejor que cualquier cosa que generemos.
func TestGenerarTodasNoPisaLasFotosPropias(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	var cat domain.Categoria
	cat.Nombre = "Segundos"
	for range 3 {
		cat.Platos = append(cat.Platos, domain.Plato{
			Nombre: "plato", Precios: []domain.Precio{{Texto: "S/ 20", Centimos: 2000}},
		})
	}
	if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{cat}},
		nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	platos := imp.Carta.Platos()

	// El dueno sube la suya en el primero.
	if err := r.MarcarFotoLista(ctx, platos[0].ID, domain.FotoPropia, "fotos/x/mia.jpg"); err != nil {
		t.Fatal(err)
	}

	elegidos, err := r.MarcarPendientes(ctx, impID, nil,
		func(context.Context, pgx.Tx, []id.ID) error { return nil })
	if err != nil {
		t.Fatal(err)
	}

	if len(elegidos) != 2 {
		t.Fatalf("se eligieron %d platos, esperaba 2: la foto propia no se toca", len(elegidos))
	}
	for _, e := range elegidos {
		if e == platos[0].ID {
			t.Fatal("se eligio el plato con foto propia: se le habria borrado su trabajo")
		}
	}
}

// PlatoPorID tiene que traer la CLAVE de la foto, no solo el nombre y el precio.
//
// De ella cuelga el borrado: quien reemplaza o quita una foto la usa para llevarse
// el archivo. Cuando el mapeo no rellenaba Foto, la clave llegaba vacia, el
// borrado salia sin hacer nada y cada foto reemplazada quedaba en el almacen para
// siempre — sin un solo error en ningun log, que es lo que lo hizo invisible.
func TestPlatoPorIDTraeLaClaveDeLaFoto(t *testing.T) {
	r, _ := repo(t)
	ctx := context.Background()
	impID := borrador(t, r)

	if err := r.GuardarCarta(ctx, impID, domain.Carta{Categorias: []domain.Categoria{{
		Nombre: "Fondos",
		Platos: []domain.Plato{{
			Nombre:  "Lomo saltado",
			Precios: []domain.Precio{{Texto: "S/ 30", Centimos: 3000}},
		}},
	}}}, nil, domain.Marcas{}); err != nil {
		t.Fatal(err)
	}

	imp, _ := r.Obtener(ctx, impID)
	platoID := imp.Carta.Platos()[0].ID

	const clave = "fotos/abc/def.webp"
	if err := r.MarcarFotoLista(ctx, platoID, domain.FotoPropia, clave); err != nil {
		t.Fatal(err)
	}

	p, err := r.PlatoPorID(ctx, platoID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Foto.Clave != clave {
		t.Fatalf("Foto.Clave = %q, quiere %q: sin ella el borrado no encuentra el archivo", p.Foto.Clave, clave)
	}
	if p.Foto.Origen != domain.FotoPropia {
		t.Fatalf("Foto.Origen = %q, quiere %q", p.Foto.Origen, domain.FotoPropia)
	}
}
