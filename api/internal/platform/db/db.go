// Package db opens the Postgres connection pool.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

// ConnectTimeout bounds establishing one connection (skill: DB calls 5s).
const ConnectTimeout = 5 * time.Second

// ErrInvalidDSN is returned when the DSN cannot be parsed. It deliberately
// carries no detail: parser errors can echo the DSN.
var ErrInvalidDSN = errors.New("db: DATABASE_URL is not a valid Postgres connection string")

// Open builds the pool. pgx connects lazily, so an unreachable database does
// not fail startup; /readyz reports it instead.
func Open(ctx context.Context, dsn logx.Secret) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn.Reveal())
	if err != nil {
		return nil, ErrInvalidDSN
	}
	cfg.ConnConfig.ConnectTimeout = ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		// Wrapped message may include connection details; callers log it
		// through the redacting logger.
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	return pool, nil
}
