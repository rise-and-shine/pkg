package pgqueue_test

import (
	"math"
	"testing"
	"time"

	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/rise-and-shine/pkg/taskmill/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestNack_ALateNackFromAnOverlappedRunChangesNothing pins Nack's ownership
// check. Run 1 outlives its visibility timeout and run 2, the last allowed,
// takes the task. Run 1's late failure must neither park the task under run 2
// nor cut run 2's lease short. Run 2's own failure still parks it.
func TestNack_ALateNackFromAnOverlappedRunChangesNothing(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)
	ctx := t.Context()
	queueName := testdb.QueueName(t)
	q := newQueue(t)

	id := enqueue(t, q, db, queueName, pgqueue.TaskParams{MaxAttempts: 2})

	// GIVEN run 1 took the task on a one-second lease
	tasks, err := q.Dequeue(ctx, db,
		pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Second, BatchSize: 1})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// AND run 2 took it over once that lease passed, on a lease of a minute
	for deadline := time.Now().Add(10 * time.Second); len(tasks) == 0 || tasks[0].Attempts != 2; {
		require.True(t, time.Now().Before(deadline), "run 2 never took the task")
		time.Sleep(50 * time.Millisecond)

		tasks, err = q.Dequeue(ctx, db,
			pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1})
		require.NoError(t, err)
	}

	lease := visibleAt(t, db, id)

	// WHEN run 1 reports its failure late
	applied, err := q.Nack(ctx, db, id, 1, map[string]any{"code": "RUN_1_FAILED"})

	// THEN nothing changes: the task is not parked and run 2 keeps its lease
	require.NoError(t, err)
	assert.False(t, applied)
	assert.True(t, lease.Equal(visibleAt(t, db, id)), "run 2's lease was cut short")

	parked, err := q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Empty(t, parked, "a late nack parked the task under the run that owns it")

	// AND run 2's own failure parks the task with its error
	applied, err = q.Nack(ctx, db, id, 2, map[string]any{"code": "RUN_2_FAILED"})
	require.NoError(t, err)
	assert.True(t, applied)

	parked, err = q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Len(t, parked, 1)
	assert.Equal(t, "RUN_2_FAILED", parked[0].DLQReason["code"])
}

// TestNack_ParksAReasonThatIsNotJSON pins that a reason JSON cannot hold still
// parks the task with what it can keep, instead of failing the nack and leaving
// the task to be parked later as ATTEMPTS_EXHAUSTED with its error lost.
func TestNack_ParksAReasonThatIsNotJSON(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)
	ctx := t.Context()
	queueName := testdb.QueueName(t)
	q := newQueue(t)

	id := enqueue(t, q, db, queueName, pgqueue.TaskParams{MaxAttempts: 1})

	tasks, err := q.Dequeue(ctx, db,
		pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	applied, err := q.Nack(ctx, db, id, 1, map[string]any{"code": "RUN_FAILED", "ratio": math.NaN()})
	require.NoError(t, err)
	assert.True(t, applied)

	parked, err := q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Len(t, parked, 1)
	assert.Equal(t, "RUN_FAILED", parked[0].DLQReason["code"])
	assert.NotContains(t, parked[0].DLQReason, "ratio")
	assert.Contains(t, parked[0].DLQReason["encode_error"], "ratio")
}

// visibleAt reads the instant a task's lease ends.
func visibleAt(t *testing.T, db *bun.DB, id int64) time.Time {
	t.Helper()

	var at time.Time
	require.NoError(t, db.NewRaw("SELECT visible_at FROM taskmill.task_queue WHERE id = ?", id).
		Scan(t.Context(), &at))

	return at
}
