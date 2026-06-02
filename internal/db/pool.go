package db

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds all tuning parameters for the pgxpool connection pool.
// Only DSN is required — all other fields fall back to safe defaults.
type PoolConfig struct {
	// DSN is the full PostgreSQL connection string.
	// Example: "host=postgres user=chat_admin password=secret dbname=chat sslmode=disable TimeZone=UTC"
	DSN string

	// MaxConns is the maximum number of connections kept in the pool.
	// Default: 25
	MaxConns int32

	// MinConns is the minimum number of idle connections kept open.
	// Default: 5
	MinConns int32

	// MaxConnLife is the maximum time a connection may be reused before
	// being closed and replaced. Prevents stale connections after a
	// Postgres restart or network partition.
	// Default: 30 minutes
	MaxConnLife time.Duration

	// MaxConnIdle is how long an idle connection stays in the pool
	// before being closed.
	// Default: 10 minutes
	MaxConnIdle time.Duration

	// HealthPeriod is how often pgxpool pings idle connections to verify
	// they are still alive.
	// Default: 1 minute
	HealthPeriod time.Duration
}

// applyDefaults fills in zero-value fields with production-safe defaults.
func (c *PoolConfig) applyDefaults() {
	if c.MaxConns == 0 {
		c.MaxConns = 25
	}
	if c.MinConns == 0 {
		c.MinConns = 5
	}
	if c.MaxConnLife == 0 {
		c.MaxConnLife = 30 * time.Minute
	}
	if c.MaxConnIdle == 0 {
		c.MaxConnIdle = 10 * time.Minute
	}
	if c.HealthPeriod == 0 {
		c.HealthPeriod = 1 * time.Minute
	}
}

// NewPool creates a *pgxpool.Pool from cfg.
//
// It applies OTel tracing to every query via otelpgx so each SQL statement
// appears as a child span in Tempo, pings once to verify connectivity, and
// closes the pool on ping failure so the caller never holds a broken pool.
func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("db.NewPool: DSN must not be empty")
	}

	cfg.applyDefaults()

	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db.NewPool: parse DSN: %w", err)
	}

	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnLifetime = cfg.MaxConnLife
	pcfg.MaxConnIdleTime = cfg.MaxConnIdle
	pcfg.HealthCheckPeriod = cfg.HealthPeriod

	// Every SQL statement becomes a child OTel span visible in Tempo.
	pcfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db.NewPool: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.NewPool: initial ping: %w", err)
	}

	return pool, nil
}
