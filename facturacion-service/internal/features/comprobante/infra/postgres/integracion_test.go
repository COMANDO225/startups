package postgres_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"facturacion-service/internal/core"
	"facturacion-service/internal/features/comprobante/application/command"
	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/features/comprobante/infra/postgres"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/pgxerr"
	"facturacion-service/pkg/ulid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Los fakes de los tests unitarios simulan la unicidad; estos la ejercitan de
// verdad. uq_comprobantes_idempotency, uq_comprobantes_numeracion y el
// UPDATE ... WHERE estado son las guardas que sostienen la correctitud, y hasta
// aqui ninguna se habia ejecutado nunca en una prueba.

const tenantDePrueba = "tenant-integracion"

func nuevoPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("necesita Docker; se omite con -short")
	}

	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("facturacion"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("levantando postgres: %v", err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(ctr) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	if err := core.MigrateDomain(dsn); err != nil {
		t.Fatalf("migrando: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sembrar(t, pool)
	return pool
}

func sembrar(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO "tenants" (id, ruc, razon_social, direccion, cert_cifrado,
		                       sol_user, sol_pass_cifrado, api_key_hash)
		VALUES ($1, '20000000001', 'EMPRESA DE PRUEBA SAC', 'AV. PRUEBA 123',
		        '\x00', 'MODDATOS', '\x00', 'hash-de-prueba')`, tenantDePrueba)
	if err != nil {
		t.Fatalf("sembrando tenant: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO "series" (tenant_id, tipo_doc, serie, correlativo) VALUES ($1, '01', 'F001', 0)`,
		tenantDePrueba); err != nil {
		t.Fatalf("sembrando serie: %v", err)
	}
}

func cmdEmision(clave string) command.EmitirCmd {
	return command.EmitirCmd{
		TenantID:       tenantDePrueba,
		IdempotencyKey: clave,
		TipoDoc:        "01",
		Serie:          "F001",
		Payload: json.RawMessage(`{
			"totales": {"importe_total": 118.00},
			"receptor": {"tipo_doc": "6", "num_doc": "20100070970", "razon_social": "CLIENTE SAC"},
			"items": [{"descripcion": "menu del dia"}]
		}`),
		Moneda:       "PEN",
		ImporteTotal: "118.00",
		FechaEmision: domain.HoyEnLima(),
	}
}

type colaNula struct{}

func (colaNula) EncolarEmision(context.Context, string) error { return nil }

// La carrera real: dos peticiones con la misma clave llegan a la vez, ambas
// pasan el chequeo previo y una choca contra el constraint. La perdedora debe
// devolver el comprobante de la ganadora, no un error ni un duplicado.
func TestIdempotenciaBajoCarrera(t *testing.T) {
	pool := nuevoPool(t)
	repo := postgres.NewRepo(pool)
	uc := command.NewEmitir(repo, colaNula{}, transaction.NewTransactor(pool))

	const paralelas = 12
	ids := make([]string, paralelas)
	errores := make([]error, paralelas)

	var wg sync.WaitGroup
	for i := range paralelas {
		wg.Go(func() {
			c, err := uc.Execute(context.Background(), cmdEmision("misma-venta"))
			if err != nil {
				errores[i] = err
				return
			}
			ids[i] = c.ID()
		})
	}
	wg.Wait()

	for i, err := range errores {
		if err != nil {
			t.Fatalf("peticion %d fallo en vez de recuperar el existente: %v", i, err)
		}
	}

	for i, id := range ids {
		if id != ids[0] {
			t.Fatalf("peticion %d devolvio %s, la primera devolvio %s: se duplico la venta", i, id, ids[0])
		}
	}

	var total int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "comprobantes" WHERE "tenant_id" = $1`, tenantDePrueba).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("se crearon %d comprobantes para una sola venta", total)
	}
}

// El constraint tiene que existir de verdad, no solo en los fakes.
func TestConstraintDeIdempotenciaExiste(t *testing.T) {
	pool := nuevoPool(t)
	ctx := context.Background()

	insertar := func(id string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO "comprobantes" (id, tenant_id, idempotency_key, tipo_doc, serie,
			                            correlativo, estado, payload, moneda, importe_total, fecha_emision)
			VALUES ($1, $2, 'clave-repetida', '01', 'F001', $3, 'pendiente', '{}', 'PEN', 118.00, current_date)`,
			id, tenantDePrueba, len(id))
		return err
	}

	if err := insertar(string(ulid.New())); err != nil {
		t.Fatalf("primera insercion: %v", err)
	}

	err := insertar(string(ulid.New()) + "x")
	if err == nil {
		t.Fatal("acepto dos comprobantes con la misma idempotency_key")
	}
	if !pgxerr.IsUniqueViolation(err) {
		t.Fatalf("esperaba violacion de unicidad, obtuve: %v", err)
	}
}

// La invariante que sostiene todo: SUNAT observa los huecos en la numeracion.
func TestCorrelativosConcurrentesSinHuecosNiDuplicados(t *testing.T) {
	pool := nuevoPool(t)
	repo := postgres.NewRepo(pool)
	uc := command.NewEmitir(repo, colaNula{}, transaction.NewTransactor(pool))

	const ventas = 40

	var wg sync.WaitGroup
	for i := range ventas {
		wg.Go(func() {
			if _, err := uc.Execute(context.Background(), cmdEmision(string(ulid.New()))); err != nil {
				t.Errorf("venta %d: %v", i, err)
			}
		})
	}
	wg.Wait()

	var total, distintos int
	var minimo, maximo int64
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*), count(DISTINCT correlativo), min(correlativo), max(correlativo)
		  FROM "comprobantes" WHERE "tenant_id" = $1`, tenantDePrueba).
		Scan(&total, &distintos, &minimo, &maximo); err != nil {
		t.Fatal(err)
	}

	if total != ventas {
		t.Fatalf("se crearon %d comprobantes de %d ventas", total, ventas)
	}
	if distintos != ventas {
		t.Fatalf("hay correlativos repetidos: %d distintos para %d comprobantes", distintos, ventas)
	}
	if minimo != 1 || maximo != int64(ventas) {
		t.Fatalf("numeracion de %d a %d, esperaba 1 a %d: hay huecos", minimo, maximo, ventas)
	}
}

// River entrega at-least-once: el mismo job puede llegar varias veces a la vez.
// Solo un worker puede ganar, o el comprobante se enviaria dos veces a SUNAT.
func TestTomarConcurrenteSoloUnGanador(t *testing.T) {
	pool := nuevoPool(t)
	repo := postgres.NewRepo(pool)
	uc := command.NewEmitir(repo, colaNula{}, transaction.NewTransactor(pool))

	c, err := uc.Execute(context.Background(), cmdEmision("venta-unica"))
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}

	const workers = 10
	ganadores := make([]bool, workers)

	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			if _, err := repo.Tomar(context.Background(), c.ID()); err == nil {
				ganadores[i] = true
			}
		})
	}
	wg.Wait()

	n := 0
	for _, gano := range ganadores {
		if gano {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d workers tomaron el mismo comprobante: se enviaria %d veces a SUNAT", n, n)
	}
}

// Si el worker que lo tenia murio, otro debe poder retomarlo pasada la ventana.
// Sin esto un comprobante queda en 'procesando' para siempre.
func TestRescateTrasVentanaVencida(t *testing.T) {
	pool := nuevoPool(t)
	repo := postgres.NewRepo(pool)
	uc := command.NewEmitir(repo, colaNula{}, transaction.NewTransactor(pool))
	ctx := context.Background()

	c, err := uc.Execute(ctx, cmdEmision("venta-abandonada"))
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}

	if _, err := repo.Tomar(ctx, c.ID()); err != nil {
		t.Fatalf("primer worker: %v", err)
	}

	if _, err := repo.Tomar(ctx, c.ID()); err == nil {
		t.Fatal("otro worker lo retomo dentro de la ventana: se enviaria dos veces")
	}

	// El worker murio: se envejece la marca mas alla del timeout.
	if _, err := pool.Exec(ctx,
		`UPDATE "comprobantes" SET "tomado_at" = now() - $2::interval WHERE "id" = $1`,
		c.ID(), (domain.TimeoutProcesando + time.Minute).String()); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Tomar(ctx, c.ID()); err != nil {
		t.Fatalf("nadie pudo rescatar el comprobante abandonado: %v", err)
	}
}
