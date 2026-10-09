package worker_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/taskmill/console"
	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
	"github.com/rise-and-shine/pkg/taskmill/internal/testdb"
	"github.com/rise-and-shine/pkg/taskmill/worker"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN a worker serving a task that fails its first runs
			queueName := testdb.QueueName(t)
			task := &flaky{failures: tt.failures}
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
		})
	}
}

// flaky fails its first `failures` runs, each with an error naming the run,
// and succeeds after them.
type flaky struct {
	failures int32
	runs     atomic.Int32
}

func (f *flaky) OperationID() string { return flakyOperation }

func (f *flaky) Execute(context.Context, any) error {
	run := f.runs.Add(1)
	if run <= f.failures {
		return errx.New(fmt.Sprintf("run %d failed", run), errx.WithCode(codeRunFailed))
	}

	return nil
}

func startWorker(t *testing.T, db *bun.DB, queueName string, task *flaky) {
	t.Helper()

	w, err := worker.New(db, queueName,
		worker.WithConcurrency(1),
		worker.WithPollInterval(10*time.Millisecond),
	)
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
