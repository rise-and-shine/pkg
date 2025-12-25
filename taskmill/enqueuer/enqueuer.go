package enqueuer

import (
	"context"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Enqueuer is a component for enqueueing tasks.
// It is used to add tasks to the queue
// and should be called either by the scheduler or via use cases.
type Enqueuer interface {
	// Enqueue enqueues a task for execution.
	// Returns the ID of the enqueued task from pgqueue.
	Enqueue(ctx context.Context, db bun.IDB, operationID string, payload any, opts ...Option) (int64, error)

	// EnqueueBatch enqueues multiple tasks in a single database operation.
	// Returns a slice of task IDs corresponding to each task in the batch.
	// If any task has a duplicate idempotency key, the entire batch fails.
	EnqueueBatch(ctx context.Context, db bun.IDB, tasks []BatchTask) ([]int64, error)
}

// New creates a new Enqueuer instance.
func New(queueName string) (Enqueuer, error) {
	queue, err := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &enqueuer{
		queue:     queue,
		queueName: queueName,
	}, nil
}

type enqueuer struct {
	queue     pgqueue.Queue
	queueName string
}

func (e *enqueuer) Enqueue(
	ctx context.Context,
	db bun.IDB,
	operationID string,
	payload any,
	opts ...Option,
) (int64, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Build meta with trace context
	meta := buildMeta(ctx)

	singleTask := pgqueue.TaskParams{
		OperationID:    operationID,
		Meta:           meta,
		Payload:        payload,
		IdempotencyKey: options.idempotencyKey,
		TaskGroupID:    options.taskGroupID,
		Priority:       options.priority,
		ScheduledAt:    options.scheduledAt,
		MaxAttempts:    options.maxAttempts,
		ExpiresAt:      options.expiresAt,
		Ephemeral:      options.ephemeral,
	}

	taskIDs, err := e.queue.EnqueueBatch(ctx, db, e.queueName, []pgqueue.TaskParams{singleTask})
	if err != nil {
		return 0, errx.Wrap(err)
	}

	if len(taskIDs) != 1 {
		return 0, errx.New("[enqueuer]: got unexpected number of task IDs", errx.WithDetails(errx.D{
			"expected": 1,
			"got":      len(taskIDs),
			"payload":  singleTask,
		}))
	}

	return taskIDs[0], nil
}

func (e *enqueuer) EnqueueBatch(
	ctx context.Context,
	db bun.IDB,
	tasks []BatchTask,
) ([]int64, error) {
	if len(tasks) == 0 {
		return []int64{}, nil
	}

	// Build meta with trace context (shared for all tasks in the batch)
	meta := buildMeta(ctx)

	// Convert BatchTask to pgqueue.TaskParams
	pgTasks := make([]pgqueue.TaskParams, 0, len(tasks))
	for _, task := range tasks {
		options := defaultOptions()
		for _, opt := range task.Options {
			opt(options)
		}

		pgTasks = append(pgTasks, pgqueue.TaskParams{
			OperationID:    task.OperationID,
			Meta:           meta,
			Payload:        task.Payload,
			IdempotencyKey: options.idempotencyKey,
			TaskGroupID:    options.taskGroupID,
			Priority:       options.priority,
			ScheduledAt:    options.scheduledAt,
			MaxAttempts:    options.maxAttempts,
			ExpiresAt:      options.expiresAt,
			Ephemeral:      options.ephemeral,
		})
	}

	taskIDs, err := e.queue.EnqueueBatch(ctx, db, e.queueName, pgTasks)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return taskIDs, nil
}

// buildMeta extracts trace context from the context and returns it as a map.
func buildMeta(ctx context.Context) map[string]string {
	propagator := otel.GetTextMapPropagator()
	carrier := make(map[string]string)
	propagator.Inject(ctx, propagation.MapCarrier(carrier))
	return carrier
}
