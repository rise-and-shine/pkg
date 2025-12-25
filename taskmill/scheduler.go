package taskmill

import (
	"context"
	"sync"
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
	RegisterSchedules(schedules ...Schedule) error

	// TriggerNow enqueues a task to run immediately.
	TriggerNow(ctx context.Context, operationID string) error

	// ListSchedules returns all registered schedules.
	ListSchedules() []Schedule

	// Start begins the scheduler loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the scheduler.
	Stop() error
}

type Schedule struct {
	// CronPattern is a standard cron expression (e.g., "0 0 * * *" for daily at midnight).
	CronPattern string

	// OperationID is a unique identifier for the task.
	// Should match the OperationID of the ucdef.AsyncTask.
	OperationID string

	// EnqueueOptions are additional options for enqueuing the task.
	EnqueueOptions []EnqueueOption
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(db *bun.DB, queueName string, opts ...SchedulerOption) (Scheduler, error) {
	options := defaultSchedulerOptions()
	for _, opt := range opts {
		opt(&options)
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
		enqueuer:      enqueuer,
		checkInterval: options.checkInterval,
		cronParser:    parser,

		schedulesMap: map[string]scheduleTrack{},
		mu:           sync.RWMutex{},
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
		logger:       logger.Named("taskmill.scheduler"),
	}

	return scheduler, nil
}

const (
	enqueueTimeout  = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

type scheduler struct {
	db *bun.DB

	enqueuer      Enqueuer
	checkInterval time.Duration
	cronParser    cron.Parser

	schedulesMap map[string]scheduleTrack
	mu           sync.RWMutex

	stopCh    chan struct{}
	stoppedCh chan struct{}

	logger logger.Logger
}

type scheduleTrack struct {
	schedule     Schedule
	cronSchedule cron.Schedule

	lastRun time.Time
	nextRun time.Time
}

func (s *scheduler) RegisterSchedules(schedules ...Schedule) error {
	for _, schedule := range schedules {
		err := s.addSchedule(schedule)
		if err != nil {
			return errx.Wrap(err)
		}
	}
	return nil
}

func (s *scheduler) TriggerNow(ctx context.Context, operationID string) error {
	s.mu.RLock()
	st, ok := s.schedulesMap[operationID]
	s.mu.RUnlock()
	if !ok {
		return errx.New(
			"[taskmill]: schedule not found",
			errx.WithCode(CodeScheduleNotFound),
			errx.WithDetails(errx.D{"operation_id": operationID}),
		)
	}

	_, err := s.enqueuer.Enqueue(
		ctx,
		s.db,
		st.schedule.OperationID,
		struct{}{},
		st.schedule.EnqueueOptions...)
	if err != nil {
		return errx.Wrap(err)
	}

	return errx.Wrap(err)
}

func (s *scheduler) ListSchedules() []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	schedules := make([]Schedule, 0, len(s.schedulesMap))
	for _, track := range s.schedulesMap {
		schedules = append(schedules, track.schedule)
	}
	return schedules
}

func (s *scheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-s.stopCh:
			close(s.stoppedCh)
			return nil

		case now := <-ticker.C:
			s.checkSchedules(now)
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

func (s *scheduler) addSchedule(schedule Schedule) error {
	cronSchedule, err := s.cronParser.Parse(schedule.CronPattern)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(errx.D{"cron_pattern": schedule.CronPattern}))
	}

	now := time.Now()
	nextRun := cronSchedule.Next(now)

	s.mu.Lock()
	s.schedulesMap[schedule.OperationID] = scheduleTrack{
		schedule:     schedule,
		cronSchedule: cronSchedule,
		lastRun:      now,
		nextRun:      nextRun,
	}
	s.mu.Unlock()

	s.logger.With(
		"operation_id", schedule.OperationID,
		"cron_pattern", schedule.CronPattern,
		"next_run", nextRun.Format(time.RFC3339),
	).Info("[taskmill]: schedule registered")

	return nil
}

func (s *scheduler) checkSchedules(now time.Time) {
	// Copy schedules while holding read lock to avoid deadlock
	// (scheduleTask needs write lock to update nextRun)
	s.mu.RLock()
	toCheck := make([]scheduleTrack, 0, len(s.schedulesMap))
	for _, st := range s.schedulesMap {
		toCheck = append(toCheck, st)
	}
	s.mu.RUnlock()

	for _, st := range toCheck {
		if now.Before(st.nextRun) {
			continue
		}

		err := s.scheduleTask(st)
		if err != nil {
			s.logger.With(
				"operation_id", st.schedule.OperationID,
				"cron_pattern", st.schedule.CronPattern,
				"next_run", st.nextRun.Format(time.RFC3339),
			).Error("[taskmill]: task scheduling failed: " + err.Error())
		} else {
			s.logger.With(
				"operation_id", st.schedule.OperationID,
				"cron_pattern", st.schedule.CronPattern,
				"next_run", st.nextRun.Format(time.RFC3339),
			).Info("[taskmill]: task scheduled successfully")
		}
	}
}

func (s *scheduler) scheduleTask(st scheduleTrack) error {
	ctx, cancel := context.WithTimeout(context.Background(), enqueueTimeout)
	defer cancel()

	_, err := s.enqueuer.Enqueue(
		ctx,
		s.db,
		st.schedule.OperationID,
		struct{}{},
		st.schedule.EnqueueOptions...,
	)
	if errx.IsCodeIn(err, pgqueue.CodeDuplicateTask) {
		s.mu.Lock()
		if current, ok := s.schedulesMap[st.schedule.OperationID]; ok {
			current.lastRun = st.nextRun
			current.nextRun = current.cronSchedule.Next(st.nextRun)
			s.schedulesMap[st.schedule.OperationID] = current
		}
		s.mu.Unlock()
		return nil
	}
	if err != nil {
		return errx.Wrap(err)
	}

	// Update next run atomically - re-fetch from map to avoid race condition
	s.mu.Lock()
	if current, ok := s.schedulesMap[st.schedule.OperationID]; ok {
		current.lastRun = st.nextRun
		current.nextRun = current.cronSchedule.Next(st.nextRun)
		s.schedulesMap[st.schedule.OperationID] = current
	}
	s.mu.Unlock()
	return nil
}
