// Package pg provides PostgreSQL database connection and utility functions.
//
// It offers abstractions for creating connection pools, working with the Bun ORM,
// handling PostgreSQL-specific errors, and managing database models with automatic
// timestamp tracking. The package integrates with OpenTelemetry for observability.
package pg

import (
	"github.com/code19m/errx"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rise-and-shine/pkg/pg/hooks"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/extra/bunotel"
)

// NewBunDB creates a new Bun database connection with the provided configuration.
func NewBunDB(cfg Config, opts ...Option) (*bun.DB, error) {
	for _, opt := range opts {
		opt(&cfg)
	}

	pool, err := NewPool(cfg)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	sqldb := stdlib.OpenDBFromPool(pool)

	bunDB := bun.NewDB(sqldb, pgdialect.New())
	applyHooks(bunDB, cfg.verbose)

	return bunDB, nil
}

// applyHooks configures Bun database with query hooks for debugging and telemetry.
//
// It adds two hooks:
// 1. A custom query logging hook that integrates with the rise-and-shine logger
// 2. An OpenTelemetry hook that provides tracing information for database operations
//
// The query logging hook will only be active when debug=true, while the OpenTelemetry
// hook is always enabled.
func applyHooks(db *bun.DB, verbose bool) {
	// Add custom query logging hook
	if verbose {
		db.AddQueryHook(
			hooks.NewDebugHook(),
		)
	}

	// Add OpenTelemetry hook
	db.AddQueryHook(bunotel.NewQueryHook())
}

func WithVerbose() Option {
	return func(c *Config) {
		c.verbose = true
	}
}

type Option func(*Config)
