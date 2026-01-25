package console

import (
	"context"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/uptrace/bun"
)

// Error codes for console operations.
const (
	// CodeScheduleNotFound is returned when a schedule is not found.
	CodeScheduleNotFound = "SCHEDULE_NOT_FOUND"
)

// Console provides administrative and monitoring operations for taskmill.
// It is a global component that works across all queues.
type Console interface {
	// ListQueues returns all distinct queue names.
	ListQueues(ctx context.Context) ([]string, error)

	// Stats returns queue statistics.
	Stats(ctx context.Context, queueName string) (*QueueStats, error)

	// ListSchedules returns registered cron schedules with execution history.
	// If queueName is provided, only schedules for that queue are returned.
	ListSchedules(ctx context.Context, queueName *string) ([]ScheduleInfo, error)

	// Purge removes all non-DLQ tasks from a queue.
	Purge(ctx context.Context, queueName string) error

	// PurgeDLQ removes all tasks from the dead letter queue.
	PurgeDLQ(ctx context.Context, queueName string) error

	// RequeueFromDLQ moves a task from DLQ back to the main queue for retry.
	RequeueFromDLQ(ctx context.Context, taskID int64) error

	// TriggerSchedule manually triggers a scheduled task to run immediately.
	// The task is enqueued to the queue specified in the schedule.
	TriggerSchedule(ctx context.Context, operationID string, opts ...enqueuer.Option) error

	// ListResults queries completed task history with filtering.
	ListResults(ctx context.Context, params ListResultsParams) ([]TaskResult, error)

	// CleanupResults deletes old task results. Returns count of deleted records.
	CleanupResults(ctx context.Context, params CleanupResultsParams) (int64, error)

	// ListDLQTasks queries tasks in the dead letter queue with filtering.
	ListDLQTasks(ctx context.Context, params ListDLQTasksParams) ([]DLQTask, error)
}

// New creates a new Console instance.
// Console is a global administrative component that works across all queues.
func New(db *bun.DB) (Console, error) {
	queue, err := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &console{
		db:    db,
		queue: queue,
	}, nil
}

type console struct {
	db    *bun.DB
	queue pgqueue.Queue
}

func (c *console) ListQueues(ctx context.Context) ([]string, error) {
	queues, err := c.queue.ListQueues(ctx, c.db)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	return queues, nil
}

func (c *console) Stats(ctx context.Context, queueName string) (*QueueStats, error) {
	stats, err := c.queue.Stats(ctx, c.db, queueName)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &QueueStats{
		QueueName:   stats.QueueName,
		Total:       stats.Total,
		Available:   stats.Available,
		InFlight:    stats.InFlight,
		Scheduled:   stats.Scheduled,
		InDLQ:       stats.InDLQ,
		OldestTask:  stats.OldestTask,
		AvgAttempts: stats.AvgAttempts,
		P95Attempts: stats.P95Attempts,
	}, nil
}

func (c *console) ListSchedules(ctx context.Context, queueName *string) ([]ScheduleInfo, error) {
	dbSchedules, err := c.queue.ListSchedules(ctx, c.db, queueName)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	result := make([]ScheduleInfo, len(dbSchedules))
	for i, sched := range dbSchedules {
		result[i] = ScheduleInfo{
			OperationID:   sched.OperationID,
			QueueName:     sched.QueueName,
			CronPattern:   sched.CronPattern,
			NextRunAt:     sched.NextRunAt,
			LastRunAt:     sched.LastRunAt,
			LastRunStatus: sched.LastRunStatus,
			LastError:     sched.LastError,
			RunCount:      sched.RunCount,
		}
	}

	return result, nil
}

func (c *console) Purge(ctx context.Context, queueName string) error {
	return errx.Wrap(c.queue.Purge(ctx, c.db, queueName))
}

func (c *console) PurgeDLQ(ctx context.Context, queueName string) error {
	return errx.Wrap(c.queue.PurgeDLQ(ctx, c.db, queueName))
}

func (c *console) RequeueFromDLQ(ctx context.Context, taskID int64) error {
	return errx.Wrap(c.queue.RequeueFromDLQ(ctx, c.db, taskID))
}

func (c *console) TriggerSchedule(ctx context.Context, operationID string, opts ...enqueuer.Option) error {
	// Get schedule from DB to verify it exists and get its queue name
	schedule, err := c.queue.GetScheduleByOperationID(ctx, c.db, operationID)
	if err != nil {
		return errx.Wrap(err, errx.WithCode(CodeScheduleNotFound))
	}

	// Create enqueuer for the schedule's queue
	enq, err := enqueuer.New(schedule.QueueName)
	if err != nil {
		return errx.Wrap(err)
	}

	// Enqueue to the queue specified in the schedule
	_, err = enq.Enqueue(ctx, c.db, operationID, struct{}{}, opts...)
	if err != nil {
		return errx.Wrap(err)
	}

	// Update last_run_at in DB (but don't change next_run_at)
	_ = c.queue.UpdateScheduleSuccess(ctx, c.db, schedule.ID, schedule.NextRunAt)

	return nil
}

//nolint:dupl // Similar adapter pattern to ListDLQTasks but different types
func (c *console) ListResults(ctx context.Context, params ListResultsParams) ([]TaskResult, error) {
	pgParams := pgqueue.ListResultsParams{
		QueueName:       params.QueueName,
		TaskGroupID:     params.TaskGroupID,
		CompletedAfter:  params.CompletedAfter,
		CompletedBefore: params.CompletedBefore,
		Limit:           params.Limit,
		Offset:          params.Offset,
	}

	dbResults, err := c.queue.ListResults(ctx, c.db, pgParams)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	results := make([]TaskResult, len(dbResults))
	for i, r := range dbResults {
		results[i] = TaskResult{
			ID:             r.ID,
			QueueName:      r.QueueName,
			TaskGroupID:    r.TaskGroupID,
			OperationID:    r.OperationID,
			Payload:        r.Payload,
			Priority:       r.Priority,
			Attempts:       r.Attempts,
			MaxAttempts:    r.MaxAttempts,
			IdempotencyKey: r.IdempotencyKey,
			ScheduledAt:    r.ScheduledAt,
			CreatedAt:      r.CreatedAt,
			CompletedAt:    r.CompletedAt,
		}
	}

	return results, nil
}

func (c *console) CleanupResults(ctx context.Context, params CleanupResultsParams) (int64, error) {
	pgParams := pgqueue.CleanupResultsParams{
		CompletedBefore: params.CompletedBefore,
		QueueName:       params.QueueName,
	}

	count, err := c.queue.CleanupResults(ctx, c.db, pgParams)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return count, nil
}

//nolint:dupl // Similar adapter pattern to ListResults but different types
func (c *console) ListDLQTasks(ctx context.Context, params ListDLQTasksParams) ([]DLQTask, error) {
	pgParams := pgqueue.ListDLQTasksParams{
		QueueName:   params.QueueName,
		OperationID: params.OperationID,
		DLQAfter:    params.DLQAfter,
		DLQBefore:   params.DLQBefore,
		Limit:       params.Limit,
		Offset:      params.Offset,
	}

	dbTasks, err := c.queue.ListDLQTasks(ctx, c.db, pgParams)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	tasks := make([]DLQTask, len(dbTasks))
	for i, t := range dbTasks {
		tasks[i] = DLQTask{
			ID:             t.ID,
			QueueName:      t.QueueName,
			TaskGroupID:    t.TaskGroupID,
			OperationID:    t.OperationID,
			Payload:        t.Payload,
			Priority:       t.Priority,
			Attempts:       t.Attempts,
			MaxAttempts:    t.MaxAttempts,
			IdempotencyKey: t.IdempotencyKey,
			CreatedAt:      t.CreatedAt,
			DLQAt:          t.DLQAt,
			DLQReason:      t.DLQReason,
		}
	}

	return tasks, nil
}
