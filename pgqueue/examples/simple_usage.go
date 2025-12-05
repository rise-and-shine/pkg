package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rise-and-shine/pkg/pg"
	"github.com/rise-and-shine/pkg/pgqueue"
)

func simple_usage() {
	// Create database connection
	db, err := pg.NewBunDB(pg.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "testdb",
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create queue with functional options
	queue, err := pgqueue.New(db,
		pgqueue.WithAutoMigrate(true), // Auto-migrate schema on startup
	)
	if err != nil {
		log.Fatalf("failed to create queue: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Example 1: Basic enqueue/dequeue
	fmt.Println("=== Example 1: Basic enqueue/dequeue ===")
	basicExample(ctx, queue)

	// Example 2: Priority queue
	fmt.Println("\n=== Example 2: Priority queue ===")
	priorityExample(ctx, queue)

	// Example 3: Delayed messages
	fmt.Println("\n=== Example 3: Delayed messages ===")
	delayedExample(ctx, queue)

	// Example 4: Message idempotency
	fmt.Println("\n=== Example 4: Message idempotency ===")
	deduplicationExample(ctx, queue)

	// Example 5: Batch enqueue
	fmt.Println("\n=== Example 5: Batch enqueue ===")
	batchExample(ctx, queue)

	// Example 6: Queue stats
	fmt.Println("\n=== Example 6: Queue stats ===")
	statsExample(ctx, queue)
}

func basicExample(ctx context.Context, queue pgqueue.Queue) {
	// Enqueue a message
	msgID, err := queue.Enqueue(ctx, "orders", map[string]any{
		"order_id": "12345",
		"amount":   99.99,
	}, "order-12345")
	if err != nil {
		log.Printf("failed to enqueue: %v", err)
		return
	}
	fmt.Printf("Enqueued message ID: %d\n", msgID)

	// Dequeue the message
	messages, err := queue.Dequeue(ctx, "orders",
		pgqueue.WithVisibilityTimeout(30*time.Second),
	)
	if err != nil {
		log.Printf("failed to dequeue: %v", err)
		return
	}

	for _, msg := range messages {
		fmt.Printf("Dequeued message ID: %d, Payload: %v\n", msg.ID, msg.Payload)

		// Process the message...
		fmt.Println("Processing order...")

		// Acknowledge the message
		err := queue.Ack(ctx, msg.ID)
		if err != nil {
			log.Printf("failed to ack: %v", err)
		}
		fmt.Println("Message acknowledged")
	}
}

func priorityExample(ctx context.Context, queue pgqueue.Queue) {
	// Enqueue messages with different priorities
	priorities := []struct {
		name     string
		priority int
	}{
		{"low priority task", -10},
		{"normal task", 0},
		{"high priority task", 10},
		{"critical task", 50},
	}

	for i, p := range priorities {
		msgID, err := queue.Enqueue(ctx, "tasks",
			map[string]any{"name": p.name},
			fmt.Sprintf("task-%d", i),
			pgqueue.WithPriority(p.priority),
		)
		if err != nil {
			log.Printf("failed to enqueue: %v", err)
			continue
		}
		fmt.Printf("Enqueued '%s' (priority %d) with ID: %d\n", p.name, p.priority, msgID)
	}

	// Dequeue - should get them in priority order
	messages, err := queue.Dequeue(ctx, "tasks",
		pgqueue.WithVisibilityTimeout(30*time.Second),
		pgqueue.WithBatchSize(4),
	)
	if err != nil {
		log.Printf("failed to dequeue: %v", err)
		return
	}

	fmt.Println("Dequeued in priority order:")
	for i, msg := range messages {
		fmt.Printf("%d. %s (priority: %d)\n", i+1, msg.Payload["name"], msg.Priority)
		queue.Ack(ctx, msg.ID)
	}
}

func delayedExample(ctx context.Context, queue pgqueue.Queue) {
	// Schedule a message for 5 seconds from now
	msgID, err := queue.Enqueue(ctx, "scheduled",
		map[string]any{"task": "scheduled task"},
		"scheduled-task-1",
		pgqueue.WithDelay(5*time.Second),
	)
	if err != nil {
		log.Printf("failed to enqueue: %v", err)
		return
	}
	fmt.Printf("Scheduled message ID: %d (will be available in 5 seconds)\n", msgID)

	// Try to dequeue immediately - should get no messages
	_, err = queue.Dequeue(ctx, "scheduled")
	if err == pgqueue.ErrNoMessages {
		fmt.Println("No messages available yet (as expected)")
	}

	// Wait and try again
	fmt.Println("Waiting 5 seconds...")
	time.Sleep(5 * time.Second)

	messages, err := queue.Dequeue(ctx, "scheduled")
	if err != nil {
		log.Printf("failed to dequeue: %v", err)
		return
	}

	if len(messages) > 0 {
		fmt.Printf("Message now available: %v\n", messages[0].Payload)
		queue.Ack(ctx, messages[0].ID)
	}
}

func deduplicationExample(ctx context.Context, queue pgqueue.Queue) {
	idempotencyKey := "order-12345-payment"

	// Enqueue with idempotency key
	msgID1, err := queue.Enqueue(ctx, "payments",
		map[string]any{"amount": 100.00},
		idempotencyKey,
	)
	if err != nil {
		log.Printf("failed to enqueue: %v", err)
		return
	}
	fmt.Printf("First enqueue, message ID: %d\n", msgID1)

	// Try to enqueue again with same idempotency key - should return same ID
	msgID2, err := queue.Enqueue(ctx, "payments",
		map[string]any{"amount": 200.00}, // Different payload
		idempotencyKey,
	)
	if err != nil {
		log.Printf("failed to enqueue: %v", err)
		return
	}
	fmt.Printf("Second enqueue, message ID: %d\n", msgID2)

	if msgID1 == msgID2 {
		fmt.Println("Idempotency worked! Same message ID returned.")
	}

	// Clean up
	messages, _ := queue.Dequeue(ctx, "payments")
	if len(messages) > 0 {
		queue.Ack(ctx, messages[0].ID)
	}
}

func batchExample(ctx context.Context, queue pgqueue.Queue) {
	// Prepare multiple messages
	messages := []pgqueue.BatchMessage{
		{
			Payload:        map[string]any{"to": "user1@example.com", "subject": "Welcome"},
			IdempotencyKey: "email-user1-welcome",
		},
		{
			Payload:        map[string]any{"to": "user2@example.com", "subject": "Confirmation"},
			IdempotencyKey: "email-user2-confirmation",
		},
		{
			Payload:        map[string]any{"to": "user3@example.com", "subject": "Newsletter"},
			IdempotencyKey: "email-user3-newsletter",
		},
	}

	// Enqueue in batch (more efficient)
	messageIDs, err := queue.EnqueueBatch(ctx, "emails", messages)
	if err != nil {
		log.Printf("failed to batch enqueue: %v", err)
		return
	}

	fmt.Printf("Batch enqueued %d messages: %v\n", len(messageIDs), messageIDs)

	// Clean up
	dequeued, _ := queue.Dequeue(ctx, "emails",
		pgqueue.WithBatchSize(3),
	)
	for _, msg := range dequeued {
		queue.Ack(ctx, msg.ID)
	}
}

func statsExample(ctx context.Context, queue pgqueue.Queue) {
	// Get queue statistics
	stats, err := queue.Stats(ctx, "orders")
	if err != nil {
		log.Printf("failed to get stats: %v", err)
		return
	}

	fmt.Printf("Queue: %s\n", stats.QueueName)
	fmt.Printf("  Total messages: %d\n", stats.Total)
	fmt.Printf("  Available: %d\n", stats.Available)
	fmt.Printf("  In-flight: %d\n", stats.InFlight)
	fmt.Printf("  Scheduled: %d\n", stats.Scheduled)
	fmt.Printf("  In DLQ: %d\n", stats.InDLQ)
	fmt.Printf("  Avg attempts: %.2f\n", stats.AvgAttempts)
	if stats.OldestMessage != nil {
		fmt.Printf("  Oldest message: %s\n", stats.OldestMessage.Format(time.RFC3339))
	}
}
