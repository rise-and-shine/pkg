package taskmill

import (
	"context"

	"github.com/code19m/errx"
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
	Enqueue(ctx context.Context, db bun.IDB, operationID string, payload any, opts ...EnqueueOption) (int64, error)

	// TODO: implement EnqueueBatch
}

// NewEnqueuer creates a new Enqueuer instance.
func NewEnqueuer(queueName string) (Enqueuer, error) {
	queue, err := pgqueue.NewQueue(getSchemaName(), getRetryStrategy())
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
	opts ...EnqueueOption,
) (int64, error) {
	options := defaultEnqueueOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Build meta with trace context
	meta := buildMeta(ctx)

	// Normalize payload to map[string]any
	normalizedPayload := normalizePayload(payload)

	singleTask := pgqueue.TaskParams{
		OperationID:    operationID,
		Meta:           meta,
		Payload:        normalizedPayload,
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
		return 0, errx.New("[taskmill]: got unexpected number of task IDs", errx.WithDetails(errx.D{
			"expected": 1,
			"got":      len(taskIDs),
			"payload":  singleTask,
		}))
	}

	return taskIDs[0], nil
}

// buildMeta extracts trace context from the context and returns it as a map.
func buildMeta(ctx context.Context) map[string]string {
	propagator := otel.GetTextMapPropagator()
	carrier := make(map[string]string)
	propagator.Inject(ctx, propagation.MapCarrier(carrier))
	return carrier
}

// normalizePayload converts the payload to map[string]any.
// If payload is nil, returns an empty map.
// If payload is already map[string]any, returns it directly.
// Otherwise, wraps it in a map with "data" key.
func normalizePayload(payload any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}

	if m, ok := payload.(map[string]any); ok {
		return m
	}

	return map[string]any{"data": payload}
}
