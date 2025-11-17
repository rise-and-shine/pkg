# Outbox Pattern Implementation

A comprehensive, production-ready outbox pattern implementation for Go applications using PostgreSQL and Kafka.

## Overview

The outbox pattern is a reliable way to ensure that database changes and event publishing happen atomically. This implementation provides:

- **Transactional Safety**: Events are stored in the same database transaction as your business data
- **Reliable Delivery**: Background processor ensures events are eventually published
- **Event Ordering**: Events are processed in the order they were created
- **Retry Logic**: Failed events are retried with configurable intervals
- **Monitoring**: Built-in health checks and statistics
- **Cleanup**: Automatic cleanup of old processed events

## Features

- ✅ **Watermill Integration**: Built on top of the existing Watermill infrastructure
- ✅ **PostgreSQL Storage**: Uses PostgreSQL with JSONB for flexible event data
- ✅ **Kafka Publishing**: Publishes events to Kafka topics with partition key support
- ✅ **Event Versioning**: Support for event schema evolution
- ✅ **Distributed Tracing**: TraceID propagation for observability
- ✅ **User Context**: User ID tracking for audit trails
- ✅ **Background Processing**: Configurable processor with health monitoring
- ✅ **Batch Processing**: Efficient batch processing of events
- ✅ **Automatic Cleanup**: Removes old processed events to prevent table bloat
- ✅ **Type Safety**: Strong typing with interfaces and builders

## Quick Start

### 1. Database Setup

First, create the outbox tables in your PostgreSQL database:

```sql
-- Create outbox table
CREATE TABLE _outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(255) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    event_version VARCHAR(50) NOT NULL DEFAULT 'v1',
    destination_topic VARCHAR(255) NOT NULL,
    partition_key VARCHAR(255),
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMP,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP,
    error_message TEXT,
    trace_id VARCHAR(255),
    user_id VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create outbox offset table
CREATE TABLE _outbox_offset (
    service_name VARCHAR(255) PRIMARY KEY,
    last_event_id UUID NOT NULL,
    last_processed TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_outbox_published_created ON _outbox (published, created_at);
CREATE INDEX idx_outbox_destination_topic ON _outbox (destination_topic);
CREATE INDEX idx_outbox_aggregate_id ON _outbox (aggregate_id);
CREATE INDEX idx_outbox_trace_id ON _outbox (trace_id) WHERE trace_id IS NOT NULL;
```

### 2. Basic Usage

```go
package main

import (
    "context"
    "github.com/rise-and-shine/pkg/outbox"
    "github.com/uptrace/bun"
)

// 1. Define your event
type UserRegisteredEvent struct {
    UserID   string `json:"user_id"`
    Email    string `json:"email"`
    Username string `json:"username"`
}

func (e UserRegisteredEvent) EventType() string {
    return "user.registered"
}

func (e UserRegisteredEvent) EventData() interface{} {
    return e
}

func (e UserRegisteredEvent) AggregateID() string {
    return e.UserID
}

func (e UserRegisteredEvent) EventVersion() string {
    return "v1"
}

func main() {
    ctx := context.Background()
    
    // Setup database and Kafka publisher (your existing setup)
    var db *bun.DB
    var kafkaPublisher message.Publisher
    
    // 2. Create outbox components
    repo := outbox.NewRepository(db)
    publisher := outbox.NewKafkaPublisher(kafkaPublisher)
    service := outbox.NewService(repo, publisher)
    
    // 3. Store events within transactions
    err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        // Your business logic here
        user := createUser(ctx, tx, "john@example.com")
        
        // Store outbox event in same transaction
        event := UserRegisteredEvent{
            UserID:   user.ID,
            Email:    user.Email,
            Username: user.Username,
        }
        
        return service.StoreEvent(
            ctx, 
            tx, 
            event, 
            "user-events",
            outbox.WithPartitionKey(user.ID),
            outbox.WithUserID(user.ID),
        )
    })
    
    if err != nil {
        panic(err)
    }
    
    // 4. Start background processor
    config := outbox.DefaultProcessorConfig("user-service")
    processor := outbox.NewProcessor(config, service, logger)
    
    if err := processor.Start(ctx); err != nil {
        panic(err)
    }
    
    // Events will be automatically published to Kafka
}
```

### 3. Using Event Builder

For more complex events, use the builder pattern:

```go
err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
    // Create order (business logic)
    order := createOrder(ctx, tx, orderData)
    
    // Use builder for event
    builder := outbox.NewEventBuilder("order.created", order.ID).
        WithVersion("v2").
        WithUserID(order.CustomerID).
        WithTraceID(getTraceID(ctx)).
        WithData(map[string]interface{}{
            "order_id":    order.ID,
            "customer_id": order.CustomerID,
            "amount":      order.Amount,
            "currency":    order.Currency,
            "items":       order.Items,
        })
    
    return service.StoreEventWithBuilder(
        ctx,
        tx,
        builder,
        "order-events",
        outbox.WithPartitionKey(order.CustomerID),
    )
})
```

## Configuration

### Processor Configuration

```go
config := outbox.ProcessorConfig{
    ServiceName:     "my-service",           // Unique service name
    Interval:        5 * time.Second,        // Processing interval
    BatchSize:       100,                    // Events per batch
    Enabled:         true,                   // Enable/disable processor
    RetryAttempts:   3,                      // Retry failed events
    RetryDelay:      10 * time.Second,       // Delay between retries
    CleanupEnabled:  true,                   // Enable automatic cleanup
    CleanupInterval: 24 * time.Hour,         // Cleanup frequency
    CleanupAfter:    7,                      // Days to keep events
}
```

### Environment Variables

You can also configure via environment variables:

```bash
OUTBOX_SERVICE_NAME=my-service
OUTBOX_INTERVAL=5s
OUTBOX_BATCH_SIZE=100
OUTBOX_ENABLED=true
OUTBOX_RETRY_ATTEMPTS=3
OUTBOX_RETRY_DELAY=10s
OUTBOX_CLEANUP_ENABLED=true
OUTBOX_CLEANUP_INTERVAL=24h
OUTBOX_CLEANUP_AFTER_DAYS=7
```

## Architecture

### Components

1. **Event Interface**: Define your domain events
2. **Repository**: Database operations for outbox entries
3. **Service**: High-level business operations
4. **Publisher**: Publishes events to external systems (Kafka)
5. **Processor**: Background worker that processes events

### Flow

```
[Business Transaction]
       ↓
[Store Event in Outbox] ← Same DB transaction
       ↓
[Background Processor] ← Polls for unpublished events
       ↓
[Kafka Publisher] ← Publishes to topics
       ↓
[Mark as Published] ← Updates outbox status
```

## Advanced Usage

### Custom Publishers

Implement the `Publisher` interface for custom publishing logic:

```go
type CustomPublisher struct {
    // your dependencies
}

func (p *CustomPublisher) Publish(ctx context.Context, event *outbox.PublishEvent) error {
    // Custom publishing logic
    return nil
}
```

### Event Versioning

Handle event schema evolution:

```go
type UserRegisteredEventV2 struct {
    UserID    string `json:"user_id"`
    Email     string `json:"email"`
    Username  string `json:"username"`
    Plan      string `json:"plan"`      // New field
    Source    string `json:"source"`    // New field
}

func (e UserRegisteredEventV2) EventVersion() string {
    return "v2"  // Increment version
}
```

### Monitoring

```go
// Health check
if err := processor.Healthcheck(); err != nil {
    log.Error("Processor unhealthy", "error", err)
}

// Statistics
stats := processor.Stats()
log.Info("Processor stats", "stats", stats)

// Manual processing
published, err := service.ProcessEvents(ctx, "my-service", 50)
if err != nil {
    log.Error("Failed to process events", "error", err)
}
```

### Error Handling

```go
// Handle specific event failures
eventID := "event-123"
err := fmt.Errorf("network timeout")
if err := service.HandleEventFailure(ctx, eventID, err); err != nil {
    log.Error("Failed to handle event failure", "error", err)
}

// Batch mark as published (for custom processing)
eventIDs := []string{"event-1", "event-2", "event-3"}
if err := service.MarkEventsAsPublished(ctx, eventIDs); err != nil {
    log.Error("Failed to mark events as published", "error", err)
}
```

## Best Practices

### 1. Event Design

- **Keep events immutable**: Never modify event data after storing
- **Use meaningful names**: `user.registered` not `user.event`
- **Include all necessary data**: Avoid requiring external lookups
- **Version your events**: Plan for schema evolution

### 2. Performance

- **Use appropriate batch sizes**: Balance throughput vs. latency
- **Partition keys**: Use meaningful partition keys for Kafka
- **Cleanup regularly**: Remove old events to prevent table bloat
- **Monitor processing lag**: Set up alerts for unprocessed events

### 3. Error Handling

- **Idempotent consumers**: Ensure event consumers can handle duplicates
- **Dead letter queues**: Handle permanently failed events
- **Circuit breakers**: Prevent cascading failures
- **Structured logging**: Include trace IDs and context

### 4. Testing

```go
// Use NoOpPublisher for tests
publisher := outbox.NewNoOpPublisher()
service := outbox.NewService(repo, publisher)

// Test event storage
event := TestEvent{Data: "test"}
err := service.StoreEvent(ctx, tx, event, "test-topic")
assert.NoError(t, err)

// Verify event was stored
events, err := repo.GetUnpublished(ctx, "test-service", 10)
assert.NoError(t, err)
assert.Len(t, events, 1)
```

## Migration Guide

If you're using the existing outbox implementation in this repository, here's how to migrate:

### 1. Update Dependencies

```bash
go get github.com/rise-and-shine/pkg/outbox@latest
```

### 2. Replace Existing Code

**Old:**
```go
// Old outbox usage
worker := outbox.NewWorker(config, db, publisher)
```

**New:**
```go
// New outbox usage
repo := outbox.NewRepository(db)
publisher := outbox.NewKafkaPublisher(kafkaPublisher)
service := outbox.NewService(repo, publisher)
processor := outbox.NewProcessor(config, service, logger)
```

### 3. Update Event Definitions

**Old:**
```go
type MyEvent struct {
    Data interface{}
}
```

**New:**
```go
type MyEvent struct {
    Data interface{}
}

func (e MyEvent) EventType() string { return "my.event" }
func (e MyEvent) EventData() interface{} { return e.Data }
func (e MyEvent) AggregateID() string { return "aggregate-id" }
func (e MyEvent) EventVersion() string { return "v1" }
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.