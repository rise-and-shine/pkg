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
	// Returns the ID of the enqueued message from pgqueue.
	Enqueue(ctx context.Context, tx *bun.Tx, operationID string, payload any, opts ...EnqueueOption) (int64, error)
}

// NewEnqueuer creates a new Enqueuer instance.
func NewEnqueuer(queue pgqueue.Queue, queueName string) (Enqueuer, error) {
	if queue == nil {
		return nil, errx.New("[taskmill]: Queue is required")
	}
	if queueName == "" {
		return nil, errx.New("[taskmill]: QueueName is required")
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
	tx *bun.Tx,
	operationID string,
	payload any,
	opts ...EnqueueOption,
) (int64, error) {
	options := defaultEnqueueOptions()
	for _, opt := range opts {
		opt(options)
	}

	singleMessage := pgqueue.SingleMessage{
		Payload:        buildPayload(ctx, operationID, payload),
		IdempotencyKey: options.idempotencyKey,
		MessageGroupID: options.messageGroupID,
		Priority:       options.priority,
		ScheduledAt:    options.scheduledAt,
		MaxAttempts:    options.maxAttempts,
		ExpiresAt:      options.expiresAt,
	}

	msgIDs, err := e.queue.EnqueueBatch(ctx, tx, e.queueName, []pgqueue.SingleMessage{singleMessage})
	if err != nil {
		return 0, errx.Wrap(err)
	}

	if len(msgIDs) != 1 {
		return 0, errx.New("[taskmill]: got unexpected number of message IDs", errx.WithDetails(errx.D{
			"expected": 1,
			"got":      len(msgIDs),
			"payload":  singleMessage,
		}))
	}

	return msgIDs[0], nil
}

func buildPayload(ctx context.Context, operationID string, payload any) map[string]any {
	propagator := otel.GetTextMapPropagator()
	carrier := make(map[string]string)
	propagator.Inject(ctx, propagation.MapCarrier(carrier))

	msg := make(map[string]any)
	msg["_operation_id"] = operationID
	msg["_trace_ctx"] = carrier

	if payload == nil {
		msg["payload"] = struct{}{}
	} else {
		msg["payload"] = payload
	}

	return msg
}
