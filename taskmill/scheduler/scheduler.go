package scheduler

import (
	"context"
	"database/sql"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/pg/hooks"
	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/robfig/cron/v3"
	"github.com/uptrace/bun"
)

// Error codes for scheduler operations.
const (
	// CodeScheduleNotFound is returned when a schedule is not found.
	CodeScheduleNotFound = "SCHEDULE_NOT_FOUND"
)

// Scheduler is a cron-based task scheduler.
type Scheduler interface {
	// RegisterSchedules registers new schedules.
	// This syncs schedules to the database: upserts provided schedules
	// and deletes any schedules not in the list.
	RegisterSchedules(ctx context.Context, schedules ...Schedule) error

	// Start begins the scheduler loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the scheduler.
	Stop() error
}

// New creates a new Scheduler instance.
func New(db *bun.DB, queueName string, opts ...Option) (Scheduler, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	queue, err := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	enq, err := enqueuer.New(queueName)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)

	scheduler := &scheduler{
		db:            db,
		queue:         queue,
		enqueuer:      enq,
		queueName:     queueName,
		checkInterval: o.checkInterval,
		cronParser:    parser,

		enqueueOptsMap: make(map[string][]enqueuer.Option),
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
	enqueuer      enqueuer.Enqueuer
	queueName     string
	checkInterval time.Duration
	cronParser    cron.Parser

	// enqueueOptsMap stores enqueue options by operation ID (in-memory only)
	enqueueOptsMap map[string][]enqueuer.Option

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
		).Info("[scheduler]: schedule registered")
	}

	// Delete schedules not in the list
	deleted, err := s.queue.DeleteSchedulesNotIn(ctx, s.db, operationIDs)
	if err != nil {
		return errx.Wrap(err)
	}

	if deleted > 0 {
		s.logger.With("count", deleted).Info("[scheduler]: removed old schedules")
	}

	return nil
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
		return errx.New("[scheduler]: shutdown timeout exceeded")
	}
}

func (s *scheduler) checkSchedules(ctx context.Context) {
	// Process due schedules one at a time within transactions
	for {
		processed, err := s.processOneSchedule(hooks.WithSuppressedQueryLogs(ctx))
		if err != nil {
			s.logger.With("error", err).Error("[scheduler]: failed to process schedule")
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
		).Error("[scheduler]: task scheduling failed")
	} else {
		s.logger.With(
			"operation_id", schedule.OperationID,
			"next_run", nextRun.Format(time.RFC3339),
		).Info("[scheduler]: task scheduled successfully")
	}

	return true, nil
}
