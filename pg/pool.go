package pg

import (
	"context"

	"github.com/code19m/errx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a new PostgreSQL connection pool with the provided configuration.
func NewPool(cfg Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.dsn())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	poolConfig.MaxConns = cfg.PoolMaxConns
	poolConfig.MinConns = cfg.PoolMinConns
	poolConfig.MaxConnIdleTime = cfg.PoolMaxConnIdleTime
	poolConfig.MaxConnLifetime = cfg.PoolMaxConnLifetime

	pgPool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return pgPool, nil
}
