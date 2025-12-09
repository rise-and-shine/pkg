package taskmill

import (
	"context"
	"sync"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based task scheduling.
type Scheduler interface {
	// AddSchedule registers a new cron schedule for a task.
	AddSchedule(schedule Schedule) error

	// RemoveSchedule removes a schedule by its ID.
	RemoveSchedule(scheduleID string) error

	// ListSchedules returns all registered schedules.
	ListSchedules() []Schedule

	// Start begins the scheduler loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the scheduler.
	Stop() error
}

// scheduledTask wraps a Schedule with parsed cron.
type scheduledTask struct {
	schedule     Schedule
	cronSchedule cron.Schedule
	location     *time.Location
	lastRun      time.Time
	nextRun      time.Time
}

// scheduler is the concrete implementation of Scheduler.
type scheduler struct {
	worker    Worker
	cfg       SchedulerConfig
	logger    logger.Logger
	schedules map[string]*scheduledTask
	mu        sync.RWMutex
	ticker    *time.Ticker
	stopCh    chan struct{}
	stoppedCh chan struct{}
	parser    cron.Parser
}

// NewScheduler creates a new Scheduler instance.
// The scheduler uses the provided Worker to enqueue tasks.
func NewScheduler(cfg SchedulerConfig, worker Worker) (Scheduler, error) {
	applySchedulerDefaults(&cfg)

	if err := validateSchedulerConfig(&cfg); err != nil {
		return nil, errx.Wrap(err)
	}

	if worker == nil {
		return nil, errx.New("[taskmill.scheduler]: worker is required")
	}

	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)

	return &scheduler{
		worker:    worker,
		cfg:       cfg,
		logger:    cfg.Logger.Named("taskmill.scheduler"),
		schedules: make(map[string]*scheduledTask),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
		parser:    parser,
	}, nil
}

// AddSchedule registers a new cron schedule.
func (s *scheduler) AddSchedule(schedule Schedule) error {
	if err := validateSchedule(&schedule); err != nil {
		return errx.Wrap(err)
	}

	// Parse cron pattern
	cronSchedule, err := s.parser.Parse(schedule.CronPattern)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(errx.D{
			"cron_pattern": schedule.CronPattern,
		}))
	}

	// Get timezone
	var location *time.Location
	if schedule.Timezone != "" {
		loc, tzErr := time.LoadLocation(schedule.Timezone)
		if tzErr != nil {
			return errx.Wrap(tzErr, errx.WithDetails(errx.D{
				"timezone": schedule.Timezone,
			}))
		}
		location = loc
	} else {
		location = time.UTC
	}

	// Calculate next run
	now := time.Now().In(location)
	nextRun := cronSchedule.Next(now)

	s.mu.Lock()
	if _, exists := s.schedules[schedule.ID]; exists {
		s.mu.Unlock()
		return errx.New("[taskmill.scheduler]: schedule already exists", errx.WithDetails(errx.D{
			"schedule_id": schedule.ID,
		}))
	}
	s.schedules[schedule.ID] = &scheduledTask{
		schedule:     schedule,
		cronSchedule: cronSchedule,
		location:     location,
		nextRun:      nextRun,
	}
	s.mu.Unlock()

	s.logger.With(
		"schedule_id", schedule.ID,
		"task_name", schedule.TaskName,
		"cron_pattern", schedule.CronPattern,
		"next_run", nextRun,
	).Info("[taskmill.scheduler] schedule added")

	return nil
}

// RemoveSchedule removes a schedule by ID.
func (s *scheduler) RemoveSchedule(scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.schedules[scheduleID]; !exists {
		return errx.New("[taskmill.scheduler]: schedule not found", errx.WithDetails(errx.D{
			"schedule_id": scheduleID,
		}))
	}

	delete(s.schedules, scheduleID)
	s.logger.With("schedule_id", scheduleID).Info("[taskmill.scheduler] schedule removed")

	return nil
}

// ListSchedules returns all registered schedules.
func (s *scheduler) ListSchedules() []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]Schedule, 0, len(s.schedules))
	for _, st := range s.schedules {
		schedules = append(schedules, st.schedule)
	}

	return schedules
}

// Start begins the scheduler loop.
func (s *scheduler) Start(ctx context.Context) error {
	s.logger.Info("[taskmill.scheduler] starting")

	s.ticker = time.NewTicker(s.cfg.TickInterval)
	defer s.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("[taskmill.scheduler] context cancelled")
			return nil
		case <-s.stopCh:
			s.logger.Debug("[taskmill.scheduler] stop signal received")
			close(s.stoppedCh)
			return nil
		case now := <-s.ticker.C:
			s.checkSchedules(ctx, now)
		}
	}
}

// Stop gracefully shuts down the scheduler.
func (s *scheduler) Stop() error {
	s.logger.Info("[taskmill.scheduler] stopping")
	close(s.stopCh)

	select {
	case <-s.stoppedCh:
		return nil
	case <-time.After(10 * time.Second):
		return errx.New("[taskmill.scheduler]: shutdown timeout exceeded")
	}
}

// checkSchedules checks all schedules and enqueues due tasks.
func (s *scheduler) checkSchedules(ctx context.Context, now time.Time) {
	s.mu.RLock()
	schedules := make([]*scheduledTask, 0, len(s.schedules))
	for _, st := range s.schedules {
		schedules = append(schedules, st)
	}
	s.mu.RUnlock()

	for _, st := range schedules {
		if !st.schedule.Enabled {
			continue
		}

		nowInLoc := now.In(st.location)
		if nowInLoc.Before(st.nextRun) {
			continue
		}

		// Build enqueue options
		opts := []EnqueueOption{}
		if st.schedule.QueueName != "" {
			opts = append(opts, WithQueue(st.schedule.QueueName))
		}
		if st.schedule.Priority != 0 {
			opts = append(opts, WithPriority(st.schedule.Priority))
		}
		if st.schedule.MaxAttempts > 0 {
			opts = append(opts, WithMaxAttempts(st.schedule.MaxAttempts))
		}

		// Enqueue via worker
		_, err := s.worker.Enqueue(ctx, st.schedule.TaskName, st.schedule.Payload, opts...)
		if err != nil {
			s.logger.With(
				"schedule_id", st.schedule.ID,
				"task_name", st.schedule.TaskName,
			).Errorx(err)
			continue
		}

		// Update next run
		st.lastRun = nowInLoc
		st.nextRun = st.cronSchedule.Next(nowInLoc)

		s.logger.With(
			"schedule_id", st.schedule.ID,
			"task_name", st.schedule.TaskName,
			"next_run", st.nextRun,
		).Debug("[taskmill.scheduler] task enqueued")
	}
}

// validateSchedule validates a Schedule.
func validateSchedule(schedule *Schedule) error {
	if schedule == nil {
		return errx.New("[taskmill.scheduler]: schedule is required")
	}
	if schedule.ID == "" {
		return errx.New("[taskmill.scheduler]: schedule ID is required")
	}
	if schedule.TaskName == "" {
		return errx.New("[taskmill.scheduler]: task name is required")
	}
	if schedule.CronPattern == "" {
		return errx.New("[taskmill.scheduler]: cron pattern is required")
	}
	if schedule.Priority < -100 || schedule.Priority > 100 {
		return errx.New("[taskmill.scheduler]: priority must be between -100 and 100")
	}
	return nil
}
