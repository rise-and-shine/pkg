package pgqueue_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/rise-and-shine/pkg/taskmill/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDequeue_ParksATaskWhoseLastAttemptNeverReported pins the one case the
// queue parks a task on its own: its last allowed run started and never
// reported back, as when the worker crashed mid-run. Picked up again once its
// visibility timeout passes, the task is parked rather than run once more.
func TestDequeue_ParksATaskWhoseLastAttemptNeverReported(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)
	queueName := testdb.QueueName(t)
	ctx := t.Context()

	q, err := pgqueue.NewQueue(config.SchemaName(), pgqueue.NewFixedDelayStrategy(0))
	require.NoError(t, err)

	_, err = q.EnqueueBatch(ctx, db, queueName, []pgqueue.TaskParams{{
		OperationID:    "crashes",
		Payload:        struct{}{},
		IdempotencyKey: uuid.NewString(),
		ScheduledAt:    time.Now(),
		MaxAttempts:    2,
	}})
	require.NoError(t, err)

	params := pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Second, BatchSize: 1}

	// GIVEN run 1 failed and said so
	tasks, err := q.Dequeue(ctx, db, params)
	require.NoError(t, err)
	require.Len(t, tasks, 1, "run 1 is handed out")
	require.NoError(t, q.Nack(ctx, db, tasks[0].ID, map[string]any{"code": "RUN_1_FAILED"}))

	// AND run 2, the last one allowed, started and never reported back
	tasks, err = q.Dequeue(ctx, db, params)
	require.NoError(t, err)
	require.Len(t, tasks, 1, "run 2 is handed out")

	// WHEN its visibility timeout passes and the queue is polled again
	// THEN no third run is handed out, and the task is parked
	var parked []pgqueue.DLQTask
	for deadline := time.Now().Add(10 * time.Second); len(parked) == 0; time.Sleep(50 * time.Millisecond) {
		require.True(t, time.Now().Before(deadline), "the task was never parked")

		tasks, err = q.Dequeue(ctx, db, params)
		require.NoError(t, err)
		require.Empty(t, tasks, "a run past max_attempts was handed out")

		parked, err = q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
		require.NoError(t, err)
	}

	require.Len(t, parked, 1)
	assert.Equal(t, pgqueue.CodeAttemptsExhausted, parked[0].DLQReason["code"])
	assert.Equal(t, 3, parked[0].Attempts, "the count includes the pickup that found the task spent")
}
