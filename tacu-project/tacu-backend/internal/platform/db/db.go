// Package db es el acceso a Postgres: el pool y la transaccion.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Abrir crea el pool y comprueba que la base responda.
//
// El Ping NO es ceremonia: pgxpool crea conexiones perezosamente, asi que sin el
// un DSN equivocado no falla al arrancar sino en la primera peticion de un
// usuario. Es la diferencia entre un despliegue que no arranca y uno que arranca
// roto.
func Abrir(ctx context.Context, dsn string, maxConexiones int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("dsn invalido: %w", err)
	}

	if maxConexiones > 0 {
		cfg.MaxConns = maxConexiones
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	// Sin esto, una conexion que el otro lado cerro en silencio (un balanceador,
	// un failover) se descubre rota recien cuando alguien la usa.
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creando el pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("la base no responde: %w", err)
	}
	return pool, nil
}

// EnTx corre fn dentro de una transaccion. Si fn devuelve error o entra en
// panico, se deshace todo.
//
// El rollback usa context.WithoutCancel A PROPOSITO. Si el contexto de la
// peticion ya esta cancelado —el cliente cerro la pestana, vencio un timeout—
// un tx.Rollback(ctx) con ese mismo contexto no llega a enviarse y la
// transaccion queda abierta en el servidor.
//
// Lo que pasa entonces NO es corrupcion: pgxpool comprueba TxStatus() al
// devolver la conexion y, si no esta limpia, la DESTRUYE (pgxpool/conn.go:32).
// El efecto real es churn — cada peticion cancelada tira una conexion y obliga a
// abrir otra, con su TCP, su TLS y su autenticacion.
//
// MEDIDO: 20 transacciones canceladas crean 19 conexiones nuevas sin
// WithoutCancel y 0 con el. Lo afirma TestElRollbackNoTiraLaConexion.
//
// (La version anterior de este comentario decia que la conexion volvia al pool
// inutilizable. Es falso: el pool no lo permite. El coste es de recursos, no de
// correctitud, y conviene no exagerarlo.)
func EnTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abriendo transaccion: %w", err)
	}

	deshacer := func() {
		if err := tx.Rollback(context.WithoutCancel(ctx)); err != nil &&
			!errors.Is(err, pgx.ErrTxClosed) {
			// No se puede devolver: ya estamos deshaciendo por otro error, y
			// ese otro error es el que le importa a quien llamo.
			_ = err
		}
	}

	defer func() {
		if p := recover(); p != nil {
			deshacer()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		deshacer()
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		deshacer()
		return fmt.Errorf("confirmando transaccion: %w", err)
	}
	return nil
}

// EsViolacionDeUnico dice si el error viene de un indice unico.
//
// Sirve para el slug: en vez de consultar si existe y despues insertar —que es
// una carrera— se intenta insertar y se reacciona al 23505.
func EsViolacionDeUnico(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// SinFilas dice si una consulta no devolvio nada.
//
// Existe para que los repositorios no tengan que importar pgx solo por esto, y
// para que "no existe" se distinga de "fallo la base": son un 404 y un 500.
func SinFilas(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
