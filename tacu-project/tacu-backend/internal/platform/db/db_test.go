package db_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tacu-backend/internal/platform/db"
)

// El DSN sale del entorno para que los tests corran contra el Postgres de
// compose.yaml. Sin el, se saltan: un `go test ./...` en una maquina limpia no
// puede fallar por falta de docker.
//
// La variable es TACU_BD_DSN, la MISMA que lee el servidor. Durante un rato
// hubo dos nombres —TACU_DSN en los tests y TACU_BD_DSN en la config— y eso es
// como se termina depurando contra una base distinta de la que se cree.
const dsnPorDefecto = "postgres://tacu:tacu@localhost:5433/tacu?sslmode=disable"

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TACU_BD_DSN")
	if dsn == "" {
		dsn = dsnPorDefecto
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := db.Abrir(ctx, dsn, 4)
	if err != nil {
		t.Skipf("sin Postgres en %s (docker compose up -d): %v", dsn, err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestAbrirFallaConUnDSNRoto(t *testing.T) {
	ctx := context.Background()

	if _, err := db.Abrir(ctx, "esto no es un dsn", 4); err == nil {
		t.Error("acepto un DSN que no se puede parsear")
	}

	// Un DSN valido contra una base que no existe: el pool se crea igual porque
	// pgx conecta perezosamente. Sin el Ping de Abrir, esto arrancaria bien y
	// fallaria en la primera peticion de un usuario.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := db.Abrir(ctx, "postgres://nadie:nadie@127.0.0.1:1/vacio?sslmode=disable", 4)
	if err == nil {
		t.Fatal("no detecto que la base no responde: el Ping no esta haciendo su trabajo")
	}
	if !strings.Contains(err.Error(), "no responde") {
		t.Errorf("el error no dice que la base no responde: %v", err)
	}
}

func TestEnTxConfirmaAlTerminarBien(t *testing.T) {
	p := pool(t)
	ctx := context.Background()

	tabla := crearTablaTemporal(t, p)

	err := db.EnTx(ctx, p, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('confirmado')")
		return err
	})
	if err != nil {
		t.Fatalf("EnTx: %v", err)
	}

	if n := contar(t, p, tabla); n != 1 {
		t.Fatalf("hay %d filas, esperaba 1: no se confirmo", n)
	}
}

func TestEnTxDeshaceCuandoLaFuncionFalla(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	tabla := crearTablaTemporal(t, p)

	fallo := errors.New("algo salio mal a mitad")
	err := db.EnTx(ctx, p, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('a deshacer')"); err != nil {
			return err
		}
		return fallo
	})
	if !errors.Is(err, fallo) {
		t.Fatalf("EnTx devolvio %v, esperaba el error de la funcion", err)
	}

	if n := contar(t, p, tabla); n != 0 {
		t.Fatalf("quedaron %d filas: no se deshizo", n)
	}
}

// EL BUG QUE MOTIVA ESTA FUNCION.
//
// Con el contexto ya cancelado, un tx.Rollback(ctx) no llega a enviarse y la
// transaccion queda abierta en el servidor. pgxpool entonces DESTRUYE esa
// conexion al devolverla (comprueba TxStatus y no la reusa), asi que el efecto
// no es corrupcion sino churn: cada peticion cancelada tira una conexion.
//
// Se afirma sobre el contador de conexiones nuevas y no sobre un error, porque
// no hay error que observar: sin el arreglo todo "funciona", solo que el pool se
// reconstruye entero cada vez. Medido: 19 conexiones nuevas sin el arreglo, 0
// con el.
func TestElRollbackNoTiraLaConexion(t *testing.T) {
	p := pool(t)
	tabla := crearTablaTemporal(t, p)

	// Se calienta el pool para no contar las conexiones iniciales.
	if n := contar(t, p, tabla); n != 0 {
		t.Fatalf("la tabla no estaba vacia: %d", n)
	}
	antes := p.Stat().NewConnsCount()

	const cancelaciones = 20
	for range cancelaciones {
		ctx, cancel := context.WithCancel(context.Background())
		_ = db.EnTx(ctx, p, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('perdido')"); err != nil {
				return err
			}
			cancel() // el cliente cierra la pestana justo aqui
			return errors.New("cancelado a mitad")
		})
	}

	if nuevas := p.Stat().NewConnsCount() - antes; nuevas > 2 {
		t.Fatalf("%d conexiones nuevas tras %d cancelaciones: el rollback no llega y el pool "+
			"esta destruyendo conexiones", nuevas, cancelaciones)
	}

	// Y lo obvio: nada se guardo.
	if n := contar(t, p, tabla); n != 0 {
		t.Fatalf("quedaron %d filas: no se deshizo", n)
	}
}

func TestEnTxDeshaceYPropagaElPanico(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	tabla := crearTablaTemporal(t, p)

	defer func() {
		if recover() == nil {
			t.Error("el panico no se propago: se lo trago EnTx")
		}
		if n := contar(t, p, tabla); n != 0 {
			t.Errorf("quedaron %d filas tras el panico: no se deshizo", n)
		}
	}()

	_ = db.EnTx(ctx, p, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('panico')"); err != nil {
			return err
		}
		panic("boom")
	})
}

func TestEsViolacionDeUnico(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	tabla := crearTablaTemporal(t, p)

	if _, err := p.Exec(ctx, "CREATE UNIQUE INDEX ON "+tabla+" (v)"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('repetido')"); err != nil {
		t.Fatal(err)
	}

	_, err := p.Exec(ctx, "INSERT INTO "+tabla+" (v) VALUES ('repetido')")
	if !db.EsViolacionDeUnico(err) {
		t.Fatalf("no reconocio la violacion de unico: %v", err)
	}
	if db.EsViolacionDeUnico(errors.New("otro error")) {
		t.Error("tomo un error cualquiera por violacion de unico")
	}
}

func TestSinFilas(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	tabla := crearTablaTemporal(t, p)

	var v string
	err := p.QueryRow(ctx, "SELECT v FROM "+tabla+" WHERE v = 'no existe'").Scan(&v)
	if !db.SinFilas(err) {
		t.Fatalf("no reconocio la ausencia de filas: %v", err)
	}
	if db.SinFilas(errors.New("otro error")) {
		t.Error("tomo un error cualquiera por ausencia de filas")
	}
}

// --- ayudas ---

func crearTablaTemporal(t *testing.T, p *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	// Nombre unico por test: son tablas reales, no TEMP, porque una TEMP vive en
	// la sesion de UNA conexion y aca el pool reparte varias.
	nombre := "prueba_" + strings.ToLower(strings.NewReplacer("/", "_", "-", "_").Replace(t.Name()))
	if _, err := p.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+nombre+" (v text)"); err != nil {
		t.Fatalf("creando la tabla de prueba: %v", err)
	}
	if _, err := p.Exec(ctx, "TRUNCATE "+nombre); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = p.Exec(context.Background(), "DROP TABLE IF EXISTS "+nombre) })
	return nombre
}

func contar(t *testing.T, p *pgxpool.Pool, tabla string) int {
	t.Helper()
	var n int
	if err := p.QueryRow(context.Background(), "SELECT count(*) FROM "+tabla).Scan(&n); err != nil {
		t.Fatalf("contando: %v", err)
	}
	return n
}
