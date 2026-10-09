package worker_test

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/taskmill/console"
	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
	"github.com/rise-and-shine/pkg/taskmill/internal/testdb"
	"github.com/rise-and-shine/pkg/taskmill/worker"
	"github.com/rise-and-shine/pkg/ucdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const (
	flakyOperation = "flaky"
	codeRunFailed  = "RUN_FAILED"

	// outcomeTimeout covers three runs and the default backoff between them,
	// 2s and then 4s.
	outcomeTimeout = 30 * time.Second
)

// TestWorker_RunsATaskMaxAttemptsTimes pins what max_attempts means: a task
// runs at most that many times, and a task whose last run fails is parked with
// that run's own error.
func TestWorker_RunsATaskMaxAttemptsTimes(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t)

	tests := []struct {
		name        string
		maxAttempts int
		failures    int32
		details     errx.D
		wantRuns    int32
		wantParked  bool
	}{
		{
			name:        "a single attempt runs once and parks its failure",
			maxAttempts: 1,
			failures:    1,
			wantRuns:    1,
			wantParked:  true,
		},
		{
			name:        "three attempts that all fail run three times",
			maxAttempts: 3,
			failures:    3,
			wantRuns:    3,
			wantParked:  true,
		},
		{
			name:        "a task that succeeds on its last attempt completes",
			maxAttempts: 3,
			failures:    2,
			wantRuns:    3,
			wantParked:  false,
		},
		{
			name:        "a failure whose details JSON cannot hold is parked with its code",
			maxAttempts: 1,
			failures:    1,
			details:     errx.D{"ratio": math.NaN()},
			wantRuns:    1,
			wantParked:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN a worker serving a task that fails its first runs
			queueName := testdb.QueueName(t)
			task := &flaky{failures: tt.failures, details: tt.details}
			startWorker(t, db, queueName, task)

			// WHEN the task is enqueued with its attempt limit
			eq, err := enqueuer.New(queueName)
			require.NoError(t, err)
			_, err = eq.Enqueue(t.Context(), db, flakyOperation, struct{}{}, enqueuer.WithMaxAttempts(tt.maxAttempts))
			require.NoError(t, err)

			// THEN it runs the expected number of times and ends where it should
			parked, completed := awaitOutcome(t, db, queueName)
			assert.Equal(t, tt.wantRuns, task.runs.Load(), "runs")

			if !tt.wantParked {
				require.NotNil(t, completed, "the task was parked, want it completed")
				assert.Equal(t, tt.maxAttempts, completed.Attempts)
				return
			}

			require.NotNil(t, parked, "the task completed, want it parked")
			assert.Equal(t, tt.maxAttempts, parked.Attempts)
			assert.Equal(t, codeRunFailed, parked.DLQReason["code"])
			assert.Contains(t, parked.DLQReason["message"], fmt.Sprintf("run %d failed", tt.wantRuns),
				"the reason is the last run's error")

			if tt.details != nil {
				assert.NotContains(t, parked.DLQReason, "details")
				assert.NotEmpty(t, parked.DLQReason["details_error"])
			}
		})
	}
}

// TestWorker_RunsEachTaskMaxAttemptsTimesAcrossWorkers pins the same limit
// under contention: several workers poll one queue, and each task still runs
// exactly as often as its attempts allow, never twice for one attempt.
func TestWorker_RunsEachTaskMaxAttemptsTimesAcrossWorkers(t *testing.T) {
	t.Parallel()

	const (
		workers     = 3
		tasks       = 12
		maxAttempts = 3
	)

	db := testdb.Open(t)
	queueName := testdb.QueueName(t)

	// GIVEN several workers serving a task whose even payloads always fail and
	// whose odd ones fail once
	task := &counted{runs: make(map[int]int)}
	for range workers {
		startWorker(t, db, queueName, task, worker.WithConcurrency(3))
	}

	// WHEN the tasks are enqueued
	eq, err := enqueuer.New(queueName)
	require.NoError(t, err)

	batch := make([]enqueuer.BatchTask, 0, tasks)
	for n := range tasks {
		batch = append(batch, enqueuer.BatchTask{
			OperationID: countedOperation,
			Payload:     map[string]int{"n": n},
			Options:     []enqueuer.Option{enqueuer.WithMaxAttempts(maxAttempts)},
		})
	}
	_, err = eq.EnqueueBatch(t.Context(), db, batch)
	require.NoError(t, err)

	// THEN half are parked and half complete
	cons, err := console.New(db)
	require.NoError(t, err)

	var parked []console.DLQTask
	for deadline := time.Now().Add(outcomeTimeout); ; time.Sleep(25 * time.Millisecond) {
		require.True(t, time.Now().Before(deadline), "the tasks did not all finish within %v", outcomeTimeout)

		parked, err = cons.ListDLQTasks(t.Context(), console.ListDLQTasksParams{QueueName: &queueName})
		require.NoError(t, err)
		completed, listErr := cons.ListResults(t.Context(), console.ListResultsParams{QueueName: &queueName})
		require.NoError(t, listErr)

		if len(parked)+len(completed) == tasks {
			assert.Len(t, completed, tasks/2)
			break
		}
	}

	// AND every task ran exactly as often as its attempts allowed
	for n := range tasks {
		want := 2
		if n%2 == 0 {
			want = maxAttempts
		}
		assert.Equal(t, want, task.runsOf(n), "runs of task %d", n)
	}

	for _, p := range parked {
		assert.Equal(t, maxAttempts, p.Attempts)
		assert.Equal(t, codeRunFailed, p.DLQReason["code"])
	}
}

const countedOperation = "counted"

// counted counts its runs per payload. An even payload always fails; an odd one
// fails its first run only.
type counted struct {
	mu   sync.Mutex
	runs map[int]int
}

func (c *counted) OperationID() string { return countedOperation }

func (c *counted) Execute(_ context.Context, payload any) error {
	fields, ok := payload.(map[string]any)
	if !ok {
		return errx.New("payload is not an object")
	}

	number, ok := fields["n"].(float64)
	if !ok {
		return errx.New("payload carries no n")
	}

	n := int(number)

	c.mu.Lock()
	c.runs[n]++
	run := c.runs[n]
	c.mu.Unlock()

	if n%2 == 0 || run == 1 {
		return errx.New(fmt.Sprintf("run %d failed", run), errx.WithCode(codeRunFailed))
	}

	return nil
}

func (c *counted) runsOf(n int) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.runs[n]
}

// flaky fails its first `failures` runs, each with an error naming the run
// and carrying details, and succeeds after them.
type flaky struct {
	failures int32
	details  errx.D
	runs     atomic.Int32
}

func (f *flaky) OperationID() string { return flakyOperation }

func (f *flaky) Execute(context.Context, any) error {
	run := f.runs.Add(1)
	if run <= f.failures {
		return errx.New(fmt.Sprintf("run %d failed", run), errx.WithCode(codeRunFailed), errx.WithDetails(f.details))
	}

	return nil
}

func startWorker(t *testing.T, db *bun.DB, queueName string, task ucdef.AsyncTask[any], opts ...worker.Option) {
	t.Helper()

	opts = append([]worker.Option{
		worker.WithConcurrency(1),
		worker.WithPollInterval(10 * time.Millisecond),
	}, opts...)

	w, err := worker.New(db, queueName, opts...)
	require.NoError(t, err)

	w.RegisterAsyncTask(task)

	done := make(chan error, 1)
	go func() { done <- w.Start(t.Context()) }()

	t.Cleanup(func() {
		assert.NoError(t, w.Stop())
		assert.NoError(t, <-done)
	})
}

// awaitOutcome waits until the queue's only task is parked or completed, and
// returns the one that happened.
func awaitOutcome(t *testing.T, db *bun.DB, queueName string) (*console.DLQTask, *console.TaskResult) {
	t.Helper()

	cons, err := console.New(db)
	require.NoError(t, err)

	deadline := time.Now().Add(outcomeTimeout)
	for time.Now().Before(deadline) {
		parked, listErr := cons.ListDLQTasks(t.Context(), console.ListDLQTasksParams{QueueName: &queueName})
		require.NoError(t, listErr)
		if len(parked) > 0 {
			return &parked[0], nil
		}

		completed, listErr := cons.ListResults(t.Context(), console.ListResultsParams{QueueName: &queueName})
		require.NoError(t, listErr)
		if len(completed) > 0 {
			return nil, &completed[0]
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("the task was neither parked nor completed within %v", outcomeTimeout)

	return nil, nil
}
