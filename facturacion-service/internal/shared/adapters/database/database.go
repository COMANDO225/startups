package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool with health check and stats.
type Pool struct {
	*pgxpool.Pool
}

// NewPool creates a PostgreSQL connection pool.
func NewPool(ctx context.Context, dsn string, maxConns, minConns int32, maxLifetime, maxIdleTime time.Duration) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxLifetime
	cfg.MaxConnIdleTime = maxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Pool{Pool: pool}, nil
}

// Health returns nil if the database is reachable.
func (p *Pool) Health(ctx context.Context) error {
	return p.Ping(ctx)
}

// Stats returns pool statistics.
type Stats struct {
	TotalConns    int32 `json:"total_conns"`
	IdleConns     int32 `json:"idle_conns"`
	AcquiredConns int32 `json:"acquired_conns"`
}

// GetStats returns current pool statistics.
func (p *Pool) GetStats() Stats {
	s := p.Stat()
	return Stats{
		TotalConns:    s.TotalConns(),
		IdleConns:     s.IdleConns(),
		AcquiredConns: s.AcquiredConns(),
	}
}
