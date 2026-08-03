package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sqlfs "facturacion-service/sql"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func migrateAll(ctx context.Context, pool *pgxpool.Pool, dsn string) error {
	if err := migrateDomain(dsn); err != nil {
		return fmt.Errorf("migrando dominio: %w", err)
	}
	if err := migrateRiver(ctx, pool); err != nil {
		return fmt.Errorf("migrando river: %w", err)
	}
	return nil
}

func migrateDomain(dsn string) error {
	src, err := iofs.New(sqlfs.Migrations, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, toPgxDSN(dsn))
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func migrateRiver(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

func toPgxDSN(dsn string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(dsn, p); ok {
			return "pgx5://" + rest
		}
	}
	return dsn
}
