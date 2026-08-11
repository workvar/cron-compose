// Package db opens a pgx connection pool to Postgres.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open returns a pgx pool for url. It does not block on a live connection: pgx
// dials lazily per query, so Open succeeds even if Postgres isn't reachable yet
// (e.g. not started, still booting). This lets the control plane come up and
// serve the UI/API in a degraded state instead of crashing on boot. Callers that
// need to know actual connectivity should Ping, same as the /healthz handler does.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return pool, nil
}
