package taskmill

import (
	"context"

	"github.com/uptrace/bun"
)

type TaskMill interface {
	EnqueueTask(ctx context.Context, idb bun.IDB, operationID string, payload any, opts ...EnqueueOption) (int64, error)
	EnqueueTaskBatch(ctx context.Context, idb bun.IDB, operationID string)
}
