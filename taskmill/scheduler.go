package taskmill

import (
	"context"
	"database/sql"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/robfig/cron/v3"
	"github.com/uptrace/bun"
)

// Scheduler is a cron-based task scheduler.
type Scheduler interface {
	// RegisterSchedules registers new schedules.
	// This syncs schedules to the database: upserts provided schedules
	// and deletes any schedules not in the list.
	RegisterSchedules(ctx context.Context, schedules ...Schedule) error

	// TriggerNow enqueues a task to run immediately.
	TriggerNow(ctx context.Context, operationID string) error

	// ListSchedules returns all registered schedules from the database.
	ListSchedules(ctx context.Context) ([]ScheduleInfo, error)

	// Start begins the scheduler loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the scheduler.
	Stop() error
}

// Schedule defines a cron-based schedule.
type Schedule struct {
	// CronPattern is a standard cron expression (e.g., "0 0 * * *" for daily at midnight).
	CronPattern string

	// OperationID is a unique identifier for the task.
	// Should match the OperationID of the ucdef.AsyncTask.
	OperationID string

	// EnqueueOptions are additional options for enqueuing the task.
	EnqueueOptions []EnqueueOption
}

// ScheduleInfo contains schedule information from the database.
type ScheduleInfo struct {
	OperationID   string
	CronPattern   string
	NextRunAt     time.Time
	LastRunAt     *time.Time
	LastRunStatus *string
	LastError     *string
	RunCount      int64
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(db *bun.DB, queueName string, opts ...SchedulerOption) (Scheduler, error) {
	options := defaultSchedulerOptions()
	for _, opt := range opts {
		opt(&options)
	}

	queue, err := pgqueue.NewQueue(getSchemaName(), getRetryStrategy())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	enqueuer, err := NewEnqueuer(queueName)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)

	scheduler := &scheduler{
		db:            db,
		queue:         queue,
		enqueuer:      enqueuer,
		queueName:     queueName,
		checkInterval: options.checkInterval,
		cronParser:    parser,

		enqueueOptsMap: make(map[string][]EnqueueOption),
		stopCh:         make(chan struct{}),
		stoppedCh:      make(chan struct{}),
		logger:         logger.Named("taskmill.scheduler"),
	}

	return scheduler, nil
}

const (
	shutdownTimeout = 10 * time.Second
)

type scheduler struct {
	db *bun.DB

	queue         pgqueue.Queue
	enqueuer      Enqueuer
	queueName     string
	checkInterval time.Duration
	cronParser    cron.Parser

	// enqueueOptsMap stores enqueue options by operation ID (in-memory only)
	enqueueOptsMap map[string][]EnqueueOption

	stopCh    chan struct{}
	stoppedCh chan struct{}

	logger logger.Logger
}

func (s *scheduler) RegisterSchedules(ctx context.Context, schedules ...Schedule) error {
	operationIDs := make([]string, 0, len(schedules))

	for _, schedule := range schedules {
		// Parse and validate cron pattern
		cronSchedule, err := s.cronParser.Parse(schedule.CronPattern)
		if err != nil {
			return errx.Wrap(err, errx.WithDetails(errx.D{"cron_pattern": schedule.CronPattern}))
		}

		// Calculate next run time
		nextRun := cronSchedule.Next(time.Now())

		// Upsert to database
		dbSchedule := pgqueue.TaskSchedule{
			OperationID: schedule.OperationID,
			QueueName:   s.queueName,
			CronPattern: schedule.CronPattern,
			NextRunAt:   nextRun,
		}

		err = s.queue.UpsertSchedule(ctx, s.db, dbSchedule)
		if err != nil {
			return errx.Wrap(err)
		}

		// Store enqueue options in memory
		s.enqueueOptsMap[schedule.OperationID] = schedule.EnqueueOptions
		operationIDs = append(operationIDs, schedule.OperationID)

		s.logger.With(
			"operation_id", schedule.OperationID,
			"cron_pattern", schedule.CronPattern,
			"next_run", nextRun.Format(time.RFC3339),
		).Info("[taskmill]: schedule registered")
	}

	// Delete schedules not in the list
	deleted, err := s.queue.DeleteSchedulesNotIn(ctx, s.db, operationIDs)
	if err != nil {
		return errx.Wrap(err)
	}

	if deleted > 0 {
		s.logger.With("count", deleted).Info("[taskmill]: removed old schedules")
	}

	return nil
}

func (s *scheduler) TriggerNow(ctx context.Context, operationID string) error {
	// Get schedule from DB to verify it exists
	schedule, err := s.queue.GetScheduleByOperationID(ctx, s.db, operationID)
	if err != nil {
		return errx.Wrap(err, errx.WithCode(CodeScheduleNotFound))
	}

	// Get enqueue options from memory
	opts := s.enqueueOptsMap[operationID]

	// Enqueue the task
	_, err = s.enqueuer.Enqueue(ctx, s.db, operationID, struct{}{}, opts...)
	if err != nil {
		return errx.Wrap(err)
	}

	// Update last_run_at in DB (but don't change next_run_at)
	err = s.queue.UpdateScheduleSuccess(ctx, s.db, schedule.ID, schedule.NextRunAt)
	if err != nil {
		s.logger.With("operation_id", operationID, "error", err).
			Warn("[taskmill]: failed to update schedule after manual trigger")
	}

	return nil
}

func (s *scheduler) ListSchedules(ctx context.Context) ([]ScheduleInfo, error) {
	dbSchedules, err := s.queue.ListSchedules(ctx, s.db)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	result := make([]ScheduleInfo, len(dbSchedules))
	for i, sched := range dbSchedules {
		result[i] = ScheduleInfo{
			OperationID:   sched.OperationID,
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

func (s *scheduler) Start(ctx context.Context) error {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-s.stopCh:
			return nil

		case <-ticker.C:
			s.checkSchedules(ctx)
		}
	}
}

func (s *scheduler) Stop() error {
	close(s.stopCh)

	select {
	case <-s.stoppedCh:
		return nil
	case <-time.After(shutdownTimeout):
		return errx.New("[taskmill]: scheduler shutdown timeout exceeded")
	}
}

func (s *scheduler) checkSchedules(ctx context.Context) {
	// Process due schedules one at a time within transactions
	for {
		processed, err := s.processOneSchedule(ctx)
		if err != nil {
			s.logger.With("error", err).Error("[taskmill]: failed to process schedule")
			return
		}
		if !processed {
			// No more due schedules
			return
		}
	}
}

func (s *scheduler) processOneSchedule(ctx context.Context) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return false, errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // intentional

	// Claim a due schedule with FOR UPDATE SKIP LOCKED
	schedule, err := s.queue.ClaimDueSchedule(ctx, &tx)
	if err != nil {
		return false, errx.Wrap(err)
	}

	if schedule == nil {
		// No due schedules
		return false, nil
	}

	// Parse cron to calculate next run
	cronSchedule, err := s.cronParser.Parse(schedule.CronPattern)
	if err != nil {
		// Invalid cron pattern - mark as failed and skip
		nextRun := time.Now().Add(time.Hour) // Retry in an hour
		_ = s.queue.UpdateScheduleFailure(ctx, &tx, schedule.ID, nextRun, err.Error())
		_ = tx.Commit()
		return true, nil
	}

	nextRun := cronSchedule.Next(time.Now())

	// Get enqueue options from memory
	opts := s.enqueueOptsMap[schedule.OperationID]

	// Enqueue the task within the same transaction
	_, enqueueErr := s.enqueuer.Enqueue(ctx, &tx, schedule.OperationID, struct{}{}, opts...)

	if enqueueErr != nil {
		// Check if it's a duplicate (idempotency key collision)
		if errx.IsCodeIn(enqueueErr, pgqueue.CodeDuplicateTask) {
			// Not an error - just update next_run_at
			err = s.queue.UpdateScheduleSuccess(ctx, &tx, schedule.ID, nextRun)
		} else {
			// Real error - mark as failed
			err = s.queue.UpdateScheduleFailure(ctx, &tx, schedule.ID, nextRun, enqueueErr.Error())
		}
	} else {
		// Success
		err = s.queue.UpdateScheduleSuccess(ctx, &tx, schedule.ID, nextRun)
	}

	if err != nil {
		return false, errx.Wrap(err)
	}

	err = tx.Commit()
	if err != nil {
		return false, errx.Wrap(err)
	}

	// Log the result
	if enqueueErr != nil && !errx.IsCodeIn(enqueueErr, pgqueue.CodeDuplicateTask) {
		s.logger.With(
			"operation_id", schedule.OperationID,
			"error", enqueueErr,
		).Error("[taskmill]: task scheduling failed")
	} else {
		s.logger.With(
			"operation_id", schedule.OperationID,
			"next_run", nextRun.Format(time.RFC3339),
		).Info("[taskmill]: task scheduled successfully")
	}

	return true, nil
}
