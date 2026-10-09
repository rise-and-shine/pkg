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
	"github.com/uptrace/bun"
)

// TestDequeue_ParksATaskWhoseLastAttemptNeverReported pins the case the queue
// parks a task on its own: its last allowed run started and never reported
// back, as when the worker crashed mid-run. Picked up again once its visibility
// timeout passes, the task is parked rather than run once more.
func TestDequeue_ParksATaskWhoseLastAttemptNeverReported(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)

	tests := []struct {
		name        string
		maxAttempts int // as stored on the row
		runs        int // the runs it gets
	}{
		{name: "two attempts", maxAttempts: 2, runs: 2},
		{name: "a row stored with zero attempts still gets one run", maxAttempts: 0, runs: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			queueName := testdb.QueueName(t)
			q := newQueue(t)

			id := enqueue(t, q, db, queueName, pgqueue.TaskParams{MaxAttempts: max(tt.maxAttempts, 1)})
			_, err := db.ExecContext(ctx, "UPDATE taskmill.task_queue SET max_attempts = ? WHERE id = ?",
				tt.maxAttempts, id)
			require.NoError(t, err)

			params := pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Second, BatchSize: 1}

			// GIVEN every run but the last failed and said so, and the last one
			// started and never reported back
			for run := 1; run <= tt.runs; run++ {
				tasks, dequeueErr := q.Dequeue(ctx, db, params)
				require.NoError(t, dequeueErr)
				require.Len(t, tasks, 1, "run %d is handed out", run)
				require.Equal(t, run, tasks[0].Attempts)

				if run < tt.runs {
					applied, nackErr := q.Nack(ctx, db, id, run, map[string]any{"code": "RUN_FAILED"})
					require.NoError(t, nackErr)
					require.True(t, applied)
				}
			}

			// WHEN its visibility timeout passes and the queue is polled again
			// THEN no further run is handed out, and the task is parked
			parked := awaitParked(t, q, db, queueName, func() {
				tasks, dequeueErr := q.Dequeue(ctx, db, params)
				require.NoError(t, dequeueErr)
				require.Empty(t, tasks, "a run past max_attempts was handed out")
			})

			assert.Equal(t, pgqueue.CodeAttemptsExhausted, parked.DLQReason["code"])
			assert.Equal(t, tt.runs+1, parked.Attempts, "the count includes the pickup that found the task spent")
		})
	}
}

// TestDequeue_ParksAnExpiredTask pins the code an expired task is parked with.
func TestDequeue_ParksAnExpiredTask(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)
	ctx := t.Context()
	queueName := testdb.QueueName(t)
	q := newQueue(t)

	expiresAt := time.Now().Add(-time.Second)
	enqueue(t, q, db, queueName, pgqueue.TaskParams{
		MaxAttempts: 3,
		ScheduledAt: expiresAt.Add(-time.Second),
		ExpiresAt:   &expiresAt,
	})

	tasks, err := q.Dequeue(ctx, db,
		pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1})
	require.NoError(t, err)
	assert.Empty(t, tasks, "an expired task was handed out")

	parked, err := q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Len(t, parked, 1)
	assert.Equal(t, pgqueue.CodeTaskExpired, parked[0].DLQReason["code"])
}

// newQueue builds a queue that retries at once, so a test never waits on a
// backoff.
func newQueue(t *testing.T) pgqueue.Queue {
	t.Helper()

	q, err := pgqueue.NewQueue(config.SchemaName(), pgqueue.NewFixedDelayStrategy(0))
	require.NoError(t, err)

	return q
}

// enqueue writes one task with the given params, filling what a test does not
// care about, and returns its id.
func enqueue(t *testing.T, q pgqueue.Queue, db bun.IDB, queueName string, params pgqueue.TaskParams) int64 {
	t.Helper()

	params.OperationID = "op"
	params.Payload = struct{}{}
	params.IdempotencyKey = uuid.NewString()
	if params.ScheduledAt.IsZero() {
		params.ScheduledAt = time.Now()
	}

	ids, err := q.EnqueueBatch(t.Context(), db, queueName, []pgqueue.TaskParams{params})
	require.NoError(t, err)
	require.Len(t, ids, 1)

	return ids[0]
}

// awaitParked calls poll until the queue's only task is parked, and returns it.
func awaitParked(t *testing.T, q pgqueue.Queue, db bun.IDB, queueName string, poll func()) pgqueue.DLQTask {
	t.Helper()

	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		require.True(t, time.Now().Before(deadline), "the task was never parked")

		poll()

		parked, err := q.ListDLQTasks(t.Context(), db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
		require.NoError(t, err)
		if len(parked) > 0 {
			require.Len(t, parked, 1)
			return parked[0]
		}
	}
}
