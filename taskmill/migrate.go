package taskmill

import (
	"context"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/uptrace/bun"
)

// Migrate creates the taskmill schema and all required tables.
// This should be called once during application startup.
func Migrate(ctx context.Context, db *bun.DB) error {
	queue, err := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
	if err != nil {
		return errx.Wrap(err)
	}

	return queue.Migrate(ctx, db, config.SchemaName())
}
