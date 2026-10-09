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

// TestNack_MatchesTheLeaseItWasDequeuedWith pins that a run's own nack still
// applies after its lease made a real round trip through the database. The
// lease end is compared to the microsecond, so a precision slip would make
// every nack a no-op. Each run's lease lands on a different microsecond.
func TestNack_MatchesTheLeaseItWasDequeuedWith(t *testing.T) {
	t.Parallel()

	const runs = 20

	db := testdb.Open(t)
	ctx := t.Context()
	queueName := testdb.QueueName(t)
	q := newQueue(t)

	id := enqueue(t, q, db, queueName, pgqueue.TaskParams{MaxAttempts: runs})
	params := pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1}

	for run := 1; run <= runs; run++ {
		tasks, err := q.Dequeue(ctx, db, params)
		require.NoError(t, err)
		require.Len(t, tasks, 1, "run %d is handed out", run)

		applied, err := q.Nack(ctx, db, id, pgqueue.LeaseOf(tasks[0]), map[string]any{"code": "RUN_FAILED"})
		require.NoError(t, err)
		require.True(t, applied, "run %d's own nack missed its lease", run)
	}

	parked, err := q.ListDLQTasks(ctx, db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Len(t, parked, 1, "the last run's nack parks the task")
	assert.Equal(t, runs, parked[0].Attempts)
}

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
	run1 := dequeueOne(t, q, db, queueName, time.Second)

	// AND run 2 took it over once that lease passed, on a lease of a minute
	run2 := awaitPickup(t, q, db, queueName, func(task pgqueue.Task) bool { return task.Attempts == 2 })

	// WHEN run 1 reports its failure late
	applied, err := q.Nack(ctx, db, id, pgqueue.LeaseOf(run1), map[string]any{"code": "RUN_1_FAILED"})

	// THEN nothing changes: the task is not parked and run 2 keeps its lease
	require.NoError(t, err)
	assert.False(t, applied)
	assertUntouched(t, q, db, queueName, id, run2)

	// AND run 2's own failure parks the task with its error
	applied, err = q.Nack(ctx, db, id, pgqueue.LeaseOf(run2), map[string]any{"code": "RUN_2_FAILED"})
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, "RUN_2_FAILED", parkedOne(t, q, db, queueName).DLQReason["code"])
}

// TestNack_ALateNackAfterARequeueChangesNothing pins why the lease end is part
// of the check. Run 1 never reports, so the task is parked as exhausted, and
// an operator requeues it. The requeue resets the count, so run 2 holds run 1's
// count again. Run 1's late failure must still change nothing.
func TestNack_ALateNackAfterARequeueChangesNothing(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)
	ctx := t.Context()
	queueName := testdb.QueueName(t)
	q := newQueue(t)

	id := enqueue(t, q, db, queueName, pgqueue.TaskParams{MaxAttempts: 1})

	// GIVEN run 1 took the task on a one-second lease and never reported
	run1 := dequeueOne(t, q, db, queueName, time.Second)

	// AND the next pickup parked the task as exhausted
	awaitParked(t, q, db, queueName, func() {
		_, dequeueErr := q.Dequeue(ctx, db,
			pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1})
		require.NoError(t, dequeueErr)
	})

	// AND an operator requeued it, and run 2 took it with run 1's count
	require.NoError(t, q.RequeueFromDLQ(ctx, db, id))
	run2 := dequeueOne(t, q, db, queueName, time.Minute)
	require.Equal(t, run1.Attempts, run2.Attempts)

	// WHEN run 1 reports its failure late
	applied, err := q.Nack(ctx, db, id, pgqueue.LeaseOf(run1), map[string]any{"code": "RUN_1_FAILED"})

	// THEN nothing changes: run 2 still owns the task
	require.NoError(t, err)
	assert.False(t, applied)
	assertUntouched(t, q, db, queueName, id, run2)

	// AND run 2's own failure parks it with its error
	applied, err = q.Nack(ctx, db, id, pgqueue.LeaseOf(run2), map[string]any{"code": "RUN_2_FAILED"})
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, "RUN_2_FAILED", parkedOne(t, q, db, queueName).DLQReason["code"])
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
	run := dequeueOne(t, q, db, queueName, time.Minute)

	applied, err := q.Nack(ctx, db, id, pgqueue.LeaseOf(run),
		map[string]any{"code": "RUN_FAILED", "ratio": math.NaN()})
	require.NoError(t, err)
	assert.True(t, applied)

	parked := parkedOne(t, q, db, queueName)
	assert.Equal(t, "RUN_FAILED", parked.DLQReason["code"])
	assert.NotContains(t, parked.DLQReason, "ratio")
	assert.Contains(t, parked.DLQReason["encode_error"], "ratio")
}

// dequeueOne hands out the queue's next task on the given lease.
func dequeueOne(t *testing.T, q pgqueue.Queue, db bun.IDB, queueName string, lease time.Duration) pgqueue.Task {
	t.Helper()

	tasks, err := q.Dequeue(t.Context(), db,
		pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: lease, BatchSize: 1})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	return tasks[0]
}

// awaitPickup polls the queue on a lease of a minute until it hands out a task
// that matches, and returns it.
func awaitPickup(
	t *testing.T,
	q pgqueue.Queue,
	db bun.IDB,
	queueName string,
	matches func(pgqueue.Task) bool,
) pgqueue.Task {
	t.Helper()

	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		require.True(t, time.Now().Before(deadline), "no matching pickup")

		tasks, err := q.Dequeue(t.Context(), db,
			pgqueue.DequeueParams{QueueName: queueName, VisibilityTimeout: time.Minute, BatchSize: 1})
		require.NoError(t, err)
		if len(tasks) == 1 && matches(tasks[0]) {
			return tasks[0]
		}
	}
}

// assertUntouched checks that the owning run's task is still active, on the
// count and the lease it was dequeued with.
func assertUntouched(t *testing.T, q pgqueue.Queue, db *bun.DB, queueName string, id int64, owner pgqueue.Task) {
	t.Helper()

	parked, err := q.ListDLQTasks(t.Context(), db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	assert.Empty(t, parked, "a late nack parked the task under the run that owns it")

	var row struct {
		Attempts  int       `bun:"attempts"`
		VisibleAt time.Time `bun:"visible_at"`
	}
	require.NoError(t, db.NewRaw("SELECT attempts, visible_at FROM taskmill.task_queue WHERE id = ?", id).
		Scan(t.Context(), &row))
	assert.Equal(t, owner.Attempts, row.Attempts)
	assert.True(t, owner.VisibleAt.Equal(row.VisibleAt), "the owning run's lease was moved")
}

// parkedOne returns the queue's only parked task.
func parkedOne(t *testing.T, q pgqueue.Queue, db bun.IDB, queueName string) pgqueue.DLQTask {
	t.Helper()

	parked, err := q.ListDLQTasks(t.Context(), db, pgqueue.ListDLQTasksParams{QueueName: &queueName})
	require.NoError(t, err)
	require.Len(t, parked, 1)

	return parked[0]
}
