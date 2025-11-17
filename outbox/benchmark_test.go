package outbox_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/rise-and-shine/pkg/outbox"
)

// setupBenchDB creates a test database connection for benchmarks
func setupBenchDB(b *testing.B) *bun.DB {
	// Use test database
	dsn := "postgres://postgres:postgres@localhost:5432/example?sslmode=disable"
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		b.Fatalf("Failed to connect to database: %v", err)
	}

	// Configure connection pool for optimal performance
	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(10)
	sqldb.SetConnMaxLifetime(5 * time.Minute)
	sqldb.SetConnMaxIdleTime(1 * time.Minute)

	db := bun.NewDB(sqldb, pgdialect.New())

	// Cleanup after benchmark
	b.Cleanup(func() {
		db.Close()
	})

	return db
}

// createTestEvent creates a realistic test event
func createTestEvent() *TestEvent {
	return &TestEvent{
		OrderID:       uuid.New().String(),
		CustomerID:    12345,
		CustomerName:  "John Doe",
		CustomerEmail: "john@example.com",
		TotalAmount:   199.99,
		Items: []OrderItem{
			{ProductID: "PROD-001", Quantity: 2, Price: 49.99},
			{ProductID: "PROD-002", Quantity: 1, Price: 99.99},
		},
		CreatedAt: time.Now(),
	}
}

// TestEvent implements outbox.Event interface
type TestEvent struct {
	OrderID       string      `json:"order_id"`
	CustomerID    int64       `json:"customer_id"`
	CustomerName  string      `json:"customer_name"`
	CustomerEmail string      `json:"customer_email"`
	TotalAmount   float64     `json:"total_amount"`
	Items         []OrderItem `json:"items"`
	CreatedAt     time.Time   `json:"created_at"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

func (e *TestEvent) EventType() string {
	return "order.created"
}

func (e *TestEvent) EventData() interface{} {
	return map[string]interface{}{
		"order_id":       e.OrderID,
		"customer_id":    e.CustomerID,
		"customer_name":  e.CustomerName,
		"customer_email": e.CustomerEmail,
		"total_amount":   e.TotalAmount,
		"items":          e.Items,
		"created_at":     e.CreatedAt,
	}
}

func (e *TestEvent) AggregateID() string {
	return e.OrderID
}

func (e *TestEvent) EventVersion() string {
	return "v1"
}

// =============================================================================
// WRITE BENCHMARKS - How fast can we store events?
// =============================================================================

// BenchmarkStoreEvent measures single event storage performance
func BenchmarkStoreEvent(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	event := createTestEvent()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTx(ctx, nil)
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
		tx.Commit()
	}

	b.StopTimer()
	// Report throughput
	eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}

// BenchmarkStoreEventWithOptions measures storage with all options
func BenchmarkStoreEventWithOptions(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	event := createTestEvent()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTx(ctx, nil)
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created",
			outbox.WithPartitionKey(fmt.Sprintf("customer-%d", i%100)),
			outbox.WithUserID(fmt.Sprintf("user-%d", 12345)),
			outbox.WithTraceID(uuid.New().String()),
			outbox.WithDedupKey(fmt.Sprintf("order-%d", i)),
			outbox.WithHeaders(map[string]interface{}{
				"correlation_id": uuid.New().String(),
				"source":         "api",
			}),
		)
		tx.Commit()
	}

	b.StopTimer()
	eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}

// BenchmarkStoreEventWithDedup measures deduplication overhead
func BenchmarkStoreEventWithDedup(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	b.Run("WithDedup", func(b *testing.B) {
		event := createTestEvent()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tx, _ := db.BeginTx(ctx, nil)
			repo.StoreWithDeduplication(ctx, tx, event, "orders.created",
				outbox.WithDedupKey(fmt.Sprintf("order-%d", i)),
			)
			tx.Commit()
		}
	})

	b.Run("WithoutDedup", func(b *testing.B) {
		event := createTestEvent()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tx, _ := db.BeginTx(ctx, nil)
			repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
			tx.Commit()
		}
	})
}

// BenchmarkStoreBatch measures batch storage performance
func BenchmarkStoreBatch(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Batch-%d", size), func(b *testing.B) {
			// Create batch of events
			events := make([]outbox.Event, size)
			for i := 0; i < size; i++ {
				events[i] = createTestEvent()
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				tx, _ := db.BeginTx(ctx, nil)
				for _, event := range events {
					repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
				}
				tx.Commit()
			}

			b.StopTimer()
			totalEvents := float64(b.N * size)
			eventsPerSec := totalEvents / b.Elapsed().Seconds()
			b.ReportMetric(eventsPerSec, "events/sec")
		})
	}
}

// =============================================================================
// READ BENCHMARKS - How fast can we fetch events?
// =============================================================================

// BenchmarkGetAvailableEvents measures event retrieval performance
func BenchmarkGetAvailableEvents(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	// Pre-populate with 10,000 events
	tx, _ := db.BeginTx(ctx, nil)
	for i := 0; i < 10000; i++ {
		event := createTestEvent()
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
	}
	tx.Commit()

	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Limit-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				events, _ := repo.GetAvailableEvents(ctx, size)
				if len(events) == 0 {
					b.Fatal("No events returned")
				}
			}

			b.StopTimer()
			eventsPerSec := float64(b.N*size) / b.Elapsed().Seconds()
			b.ReportMetric(eventsPerSec, "events/sec")
		})
	}
}

// =============================================================================
// UPDATE BENCHMARKS - How fast can we update event status?
// =============================================================================

// BenchmarkMarkAsPublished measures update performance
func BenchmarkMarkAsPublished(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	// Pre-create events and collect IDs
	ids := make([]uuid.UUID, b.N)
	tx, _ := db.BeginTx(ctx, nil)
	for i := 0; i < b.N; i++ {
		event := createTestEvent()
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")

		// Get the last inserted ID
		var entry outbox.OutboxEntry
		db.NewSelect().
			Model(&entry).
			Order("created_at DESC").
			Limit(1).
			Scan(ctx)
		ids[i] = entry.ID
	}
	tx.Commit()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		repo.MarkAsPublished(ctx, ids[i])
	}

	b.StopTimer()
	updatesPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(updatesPerSec, "updates/sec")
}

// BenchmarkMarkAsFailed measures failure update performance
func BenchmarkMarkAsFailed(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	// Pre-create events and collect IDs
	ids := make([]uuid.UUID, b.N)
	tx, _ := db.BeginTx(ctx, nil)
	for i := 0; i < b.N; i++ {
		event := createTestEvent()
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")

		var entry outbox.OutboxEntry
		db.NewSelect().
			Model(&entry).
			Order("created_at DESC").
			Limit(1).
			Scan(ctx)
		ids[i] = entry.ID
	}
	tx.Commit()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		repo.MarkAsFailed(ctx, ids[i], "Test error")
	}

	b.StopTimer()
	updatesPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(updatesPerSec, "updates/sec")
}

// =============================================================================
// END-TO-END BENCHMARKS - Full workflow performance
// =============================================================================

// BenchmarkFullCycle measures complete store -> fetch -> update cycle
func BenchmarkFullCycle(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 1. Store event
		event := createTestEvent()
		tx, _ := db.BeginTx(ctx, nil)
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
		tx.Commit()

		// 2. Fetch event
		events, _ := repo.GetAvailableEvents(ctx, 1)
		if len(events) == 0 {
			b.Fatal("No events returned")
		}

		// 3. Mark as published
		repo.MarkAsPublished(ctx, events[0].ID)
	}

	b.StopTimer()
	cyclesPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(cyclesPerSec, "cycles/sec")
}

// BenchmarkWorkerBatchProcess simulates worker processing batches
func BenchmarkWorkerBatchProcess(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	batchSize := 100

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 1. Store batch of events
		tx, _ := db.BeginTx(ctx, nil)
		for j := 0; j < batchSize; j++ {
			event := createTestEvent()
			repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
		}
		tx.Commit()

		// 2. Fetch batch
		events, _ := repo.GetAvailableEvents(ctx, batchSize)

		// 3. Mark all as published
		for _, event := range events {
			repo.MarkAsPublished(ctx, event.ID)
		}
	}

	b.StopTimer()
	totalEvents := float64(b.N * batchSize)
	eventsPerSec := totalEvents / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}

// =============================================================================
// CONCURRENCY BENCHMARKS - How does it perform under concurrent load?
// =============================================================================

// BenchmarkConcurrentStore measures concurrent write performance
func BenchmarkConcurrentStore(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	concurrencyLevels := []int{1, 5, 10, 25, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Goroutines-%d", concurrency), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					event := createTestEvent()
					tx, _ := db.BeginTx(ctx, nil)
					repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
					tx.Commit()
				}
			})

			b.StopTimer()
			eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(eventsPerSec, "events/sec")
		})
	}
}

// =============================================================================
// CLEANUP BENCHMARKS - Maintenance operation performance
// =============================================================================

// BenchmarkDeleteOldEvents measures cleanup performance
func BenchmarkDeleteOldEvents(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	// Pre-populate with old events
	tx, _ := db.BeginTx(ctx, nil)
	for i := 0; i < 10000; i++ {
		event := createTestEvent()
		repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
	}
	tx.Commit()

	// Mark all as published
	events, _ := repo.GetAvailableEvents(ctx, 10000)
	for _, event := range events {
		repo.MarkAsPublished(ctx, event.ID)
	}

	olderThan := 24 * time.Hour // Delete events older than 24 hours

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		repo.DeleteOldEvents(ctx, olderThan)
	}
}

// =============================================================================
// OFFSET BENCHMARKS - Worker offset tracking
// =============================================================================

// BenchmarkOffsetOperations measures offset get/update performance
func BenchmarkOffsetOperations(b *testing.B) {
	db := setupBenchDB(b)
	repo := outbox.NewRepository(db)
	ctx := context.Background()

	serviceName := "test-worker"

	b.Run("GetOffset", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			repo.GetOffset(ctx, serviceName)
		}
	})

	b.Run("UpdateOffset", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			repo.UpdateOffset(ctx, serviceName, uuid.New())
		}
	})
}

// =============================================================================
// MEMORY ALLOCATION BENCHMARKS
// =============================================================================

// BenchmarkEventSerialization measures JSON marshaling overhead
func BenchmarkEventSerialization(b *testing.B) {
	event := createTestEvent()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.EventData()
	}
}

// =============================================================================
// COMPARISON BENCHMARKS - Compare different approaches
// =============================================================================

// BenchmarkStoreVsUoWStore compares traditional vs UoW repository
func BenchmarkStoreVsUoWStore(b *testing.B) {
	db := setupBenchDB(b)
	ctx := context.Background()

	b.Run("TraditionalRepo", func(b *testing.B) {
		repo := outbox.NewRepository(db)
		event := createTestEvent()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tx, _ := db.BeginTx(ctx, nil)
			repo.StoreWithDeduplication(ctx, tx, event, "orders.created")
			tx.Commit()
		}
	})

	b.Run("UoWRepo", func(b *testing.B) {
		event := createTestEvent()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tx, _ := db.BeginTx(ctx, nil)
			uowRepo := outbox.NewUoWRepository(tx, db)
			uowRepo.StoreEvent(ctx, event, "orders.created")
			tx.Commit()
		}
	})
}
