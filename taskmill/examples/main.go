package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creasty/defaults"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/rise-and-shine/pkg/taskmill"
	"github.com/rise-and-shine/pkg/taskmill/console"
	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
	"github.com/rise-and-shine/pkg/taskmill/scheduler"
	"github.com/rise-and-shine/pkg/taskmill/worker"
	"github.com/uptrace/bun"
)

// =============================================================================
// Configuration
// =============================================================================

const queueName = "example-queue"

func getDBConfig() pg.Config {
	cfg := pg.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		Database: getEnv("DB_NAME", "taskmill_example"),
		SSLMode:  "disable",
		Debug:    false,
	}
	setDefaults(&cfg)
	return cfg
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// =============================================================================
// Example Tasks
// =============================================================================

// SendEmailTask sends an email notification.
type SendEmailTask struct{}

func (t *SendEmailTask) OperationID() string {
	return "email.send"
}

func (t *SendEmailTask) Execute(ctx context.Context, payload any) error {
	log.Printf("[SendEmailTask] Processing email task with payload: %+v", payload)
	time.Sleep(100 * time.Millisecond) // Simulate work
	log.Printf("[SendEmailTask] Email sent successfully!")
	return nil
}

// GenerateReportTask generates a report.
type GenerateReportTask struct{}

func (t *GenerateReportTask) OperationID() string {
	return "report.generate"
}

func (t *GenerateReportTask) Execute(ctx context.Context, payload any) error {
	log.Printf("[GenerateReportTask] Generating report with payload: %+v", payload)
	time.Sleep(200 * time.Millisecond) // Simulate work
	log.Printf("[GenerateReportTask] Report generated successfully!")
	return nil
}

// CleanupTask cleans up old data (scheduled task).
type CleanupTask struct{}

func (t *CleanupTask) OperationID() string {
	return "cleanup.expired"
}

func (t *CleanupTask) Execute(ctx context.Context, payload any) error {
	log.Printf("[CleanupTask] Running cleanup task...")
	time.Sleep(50 * time.Millisecond) // Simulate work
	log.Printf("[CleanupTask] Cleanup completed!")
	return nil
}

// HealthCheckTask runs periodic health checks (scheduled task).
type HealthCheckTask struct{}

func (t *HealthCheckTask) OperationID() string {
	return "health.check"
}

func (t *HealthCheckTask) Execute(ctx context.Context, payload any) error {
	log.Printf("[HealthCheckTask] Running health check...")
	log.Printf("[HealthCheckTask] All systems operational!")
	return nil
}

// =============================================================================
// Main
// =============================================================================

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Connect to database
	db := connectDB()
	defer db.Close()

	// Run migrations
	log.Println("Running migrations...")
	if err := taskmill.Migrate(ctx, db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations completed!")

	// Create components
	enq := createEnqueuer()
	wrk := createWorker(db)
	sched := createScheduler(ctx, db)
	cons := createConsole(db)

	// Start worker and scheduler in background
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting worker...")
		if err := wrk.Start(ctx); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting scheduler...")
		if err := sched.Start(ctx); err != nil {
			log.Printf("Scheduler error: %v", err)
		}
	}()

	// Enqueue some tasks
	log.Println("\n=== Enqueuing tasks ===")
	enqueueTasks(ctx, db, enq)

	// Demonstrate Console functionality
	log.Println("\n=== Console Demo ===")
	demonstrateConsole(ctx, cons)

	// Wait for signal
	log.Println("\n=== Running (press Ctrl+C to stop) ===")
	<-sigCh

	// Graceful shutdown
	log.Println("\nShutting down...")
	cancel()

	if err := sched.Stop(); err != nil {
		log.Printf("Scheduler stop error: %v", err)
	}
	if err := wrk.Stop(); err != nil {
		log.Printf("Worker stop error: %v", err)
	}

	wg.Wait()
	log.Println("Shutdown complete!")
}

// =============================================================================
// Setup Functions
// =============================================================================

func connectDB() *bun.DB {
	cfg := getDBConfig()
	log.Printf("Connecting to database: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.Database)

	db, err := pg.NewBunDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected!")
	return db
}

func createEnqueuer() enqueuer.Enqueuer {
	enq, err := enqueuer.New(queueName)
	if err != nil {
		log.Fatalf("Failed to create enqueuer: %v", err)
	}
	return enq
}

func createWorker(db *bun.DB) worker.Worker {
	wrk, err := worker.New(db, queueName,
		worker.WithConcurrency(2),
		worker.WithPollInterval(500*time.Millisecond),
		worker.WithProcessTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// Register task handlers
	wrk.RegisterAsyncTask(&SendEmailTask{})
	wrk.RegisterAsyncTask(&GenerateReportTask{})
	wrk.RegisterAsyncTask(&CleanupTask{})
	wrk.RegisterAsyncTask(&HealthCheckTask{})

	log.Println("Worker created with registered tasks: email.send, report.generate, cleanup.expired, health.check")
	return wrk
}

func createScheduler(ctx context.Context, db *bun.DB) scheduler.Scheduler {
	sched, err := scheduler.New(db, queueName,
		scheduler.WithCheckInterval(1*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}

	// Register schedules
	err = sched.RegisterSchedules(ctx,
		// Run cleanup every minute
		scheduler.Schedule{
			OperationID: "cleanup.expired",
			CronPattern: "* * * * *", // Every minute
			EnqueueOptions: []enqueuer.Option{
				enqueuer.WithEphemeral(), // Don't save results for scheduled cleanup
			},
		},
		// Run health check every 2 minutes
		scheduler.Schedule{
			OperationID: "health.check",
			CronPattern: "*/2 * * * *", // Every 2 minutes
			EnqueueOptions: []enqueuer.Option{
				enqueuer.WithEphemeral(),
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to register schedules: %v", err)
	}

	log.Println("Scheduler created with schedules: cleanup.expired (every min), health.check (every 2 min)")
	return sched
}

func createConsole(db *bun.DB) console.Console {
	cons, err := console.New(db)
	if err != nil {
		log.Fatalf("Failed to create console: %v", err)
	}
	return cons
}

func enqueueTasks(ctx context.Context, db *bun.DB, enq enqueuer.Enqueuer) {
	// Example 1: Simple task with map payload
	taskID, err := enq.Enqueue(ctx, db, "email.send", map[string]any{
		"to":      "user@example.com",
		"subject": "Welcome!",
		"body":    "Hello and welcome to our service!",
	})
	if err != nil {
		log.Printf("Failed to enqueue email task: %v", err)
	} else {
		log.Printf("Enqueued email task with ID: %d", taskID)
	}

	// Example 2: Task with priority
	taskID, err = enq.Enqueue(ctx, db, "report.generate",
		map[string]any{"report_type": "monthly", "month": "2024-01"},
		enqueuer.WithPriority(10), // Higher priority
	)
	if err != nil {
		log.Printf("Failed to enqueue report task: %v", err)
	} else {
		log.Printf("Enqueued report task with ID: %d (priority: 10)", taskID)
	}

	// Example 3: Scheduled task (delayed execution)
	taskID, err = enq.Enqueue(ctx, db, "email.send",
		map[string]any{"to": "delayed@example.com", "subject": "Delayed Email"},
		enqueuer.WithScheduledAt(time.Now().Add(10*time.Second)),
	)
	if err != nil {
		log.Printf("Failed to enqueue delayed task: %v", err)
	} else {
		log.Printf("Enqueued delayed email task with ID: %d (runs in 10s)", taskID)
	}

	// Example 4: Task with custom retry settings
	taskID, err = enq.Enqueue(ctx, db, "report.generate",
		map[string]any{"report_type": "critical"},
		enqueuer.WithMaxAttempts(5),
	)
	if err != nil {
		log.Printf("Failed to enqueue critical task: %v", err)
	} else {
		log.Printf("Enqueued critical report task with ID: %d (max attempts: 5)", taskID)
	}

	// Example 5: Ephemeral task (won't be saved to results)
	taskID, err = enq.Enqueue(ctx, db, "email.send",
		map[string]any{"to": "ephemeral@example.com"},
		enqueuer.WithEphemeral(),
	)
	if err != nil {
		log.Printf("Failed to enqueue ephemeral task: %v", err)
	} else {
		log.Printf("Enqueued ephemeral email task with ID: %d (no results saved)", taskID)
	}

	// Example 6: Task with idempotency key (prevents duplicates)
	idempotencyKey := fmt.Sprintf("invoice-%s", "inv_12345")
	taskID, err = enq.Enqueue(ctx, db, "report.generate",
		map[string]any{"invoice_id": "inv_12345"},
		enqueuer.WithIdempotencyKey(idempotencyKey),
	)
	if err != nil {
		log.Printf("Failed to enqueue idempotent task: %v", err)
	} else {
		log.Printf("Enqueued idempotent task with ID: %d (key: %s)", taskID, idempotencyKey)
	}

	// Try to enqueue duplicate - should fail
	_, err = enq.Enqueue(ctx, db, "report.generate",
		map[string]any{"invoice_id": "inv_12345"},
		enqueuer.WithIdempotencyKey(idempotencyKey),
	)
	if err != nil {
		log.Printf("Duplicate task rejected (expected): %v", err)
	}

	// Example 7: Batch enqueue - multiple tasks in a single operation
	log.Println("\n--- Batch Enqueue ---")
	batchTasks := []enqueuer.BatchTask{
		{
			OperationID: "email.send",
			Payload:     map[string]any{"to": "batch1@example.com", "subject": "Batch Email 1"},
		},
		{
			OperationID: "email.send",
			Payload:     map[string]any{"to": "batch2@example.com", "subject": "Batch Email 2"},
			Options:     []enqueuer.Option{enqueuer.WithPriority(5)},
		},
		{
			OperationID: "report.generate",
			Payload:     map[string]any{"report_type": "batch_report", "items": []string{"a", "b", "c"}},
			Options:     []enqueuer.Option{enqueuer.WithMaxAttempts(5), enqueuer.WithEphemeral()},
		},
	}

	taskIDs, err := enq.EnqueueBatch(ctx, db, batchTasks)
	if err != nil {
		log.Printf("Failed to enqueue batch: %v", err)
	} else {
		log.Printf("Enqueued batch of %d tasks with IDs: %v", len(taskIDs), taskIDs)
	}
}

func demonstrateConsole(ctx context.Context, cons console.Console) {
	// 1. List all queues
	log.Println("\n--- ListQueues ---")
	queues, err := cons.ListQueues(ctx)
	if err != nil {
		log.Printf("Failed to list queues: %v", err)
	} else {
		log.Printf("Available queues: %v", queues)
	}

	// 2. Get queue statistics
	log.Println("\n--- Stats ---")
	stats, err := cons.Stats(ctx, queueName)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		log.Printf("Queue '%s' stats:", stats.QueueName)
		log.Printf("  Total: %d, Available: %d, InFlight: %d, Scheduled: %d, InDLQ: %d",
			stats.Total, stats.Available, stats.InFlight, stats.Scheduled, stats.InDLQ)
		log.Printf("  AvgAttempts: %.2f, P95Attempts: %.2f", stats.AvgAttempts, stats.P95Attempts)
	}

	// 3. List all schedules
	log.Println("\n--- ListSchedules ---")
	schedules, err := cons.ListSchedules(ctx, nil)
	if err != nil {
		log.Printf("Failed to list schedules: %v", err)
	} else {
		for _, s := range schedules {
			status := "never run"
			if s.LastRunStatus != nil {
				status = *s.LastRunStatus
			}
			log.Printf("  - %s (%s) next=%s runs=%d status=%s",
				s.OperationID,
				s.CronPattern,
				s.NextRunAt.Format(time.RFC3339),
				s.RunCount,
				status,
			)
		}
	}

	// 4. Trigger a schedule manually
	log.Println("\n--- TriggerSchedule ---")
	err = cons.TriggerSchedule(ctx, "health.check", enqueuer.WithPriority(50))
	if err != nil {
		log.Printf("Failed to trigger schedule: %v", err)
	} else {
		log.Println("Triggered 'health.check' schedule manually with priority 50")
	}

	// 5. List completed task results
	log.Println("\n--- ListResults ---")
	results, err := cons.ListResults(ctx, console.ListResultsParams{
		Limit: 5,
	})
	if err != nil {
		log.Printf("Failed to list results: %v", err)
	} else {
		log.Printf("Recent completed tasks (%d):", len(results))
		for _, r := range results {
			log.Printf("  - ID=%d op=%s queue=%s attempts=%d completed=%s",
				r.ID, r.OperationID, r.QueueName, r.Attempts,
				r.CompletedAt.Format(time.RFC3339))
		}
	}

	// 6. Cleanup old results (example: delete results older than 1 hour)
	log.Println("\n--- CleanupResults ---")
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	deleted, err := cons.CleanupResults(ctx, console.CleanupResultsParams{
		CompletedBefore: oneHourAgo,
	})
	if err != nil {
		log.Printf("Failed to cleanup results: %v", err)
	} else {
		log.Printf("Cleaned up %d old task results (completed before %s)", deleted, oneHourAgo.Format(time.RFC3339))
	}

	// 7. Purge operations (commented out - destructive!)
	// log.Println("\n--- Purge/PurgeDLQ ---")
	// cons.Purge(ctx, queueName)      // Deletes all pending tasks
	// cons.PurgeDLQ(ctx, queueName)   // Deletes all DLQ tasks

	// 8. Requeue from DLQ (would need a task ID)
	// log.Println("\n--- RequeueFromDLQ ---")
	// cons.RequeueFromDLQ(ctx, taskID) // Moves task from DLQ back to queue
}

func setDefaults(config any) {
	if err := defaults.Set(config); err != nil {
		slog.Error(
			fmt.Sprintf("[cfgloader]: failed to set default values for config: %s", err),
		)
		os.Exit(1)
	}
}
