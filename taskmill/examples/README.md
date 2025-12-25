# Taskmill Example

This example demonstrates the full functionality of the taskmill package.

## Prerequisites

1. PostgreSQL running on localhost:5432
2. Create a database named `taskmill_example`:

```bash
createdb taskmill_example
```

Or with docker:

```bash
docker run -d \
  --name taskmill-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=taskmill_example \
  -p 5432:5432 \
  postgres:16
```

## Running the Example

```bash
# From the examples directory
go run main.go

# Or with custom database settings
DB_HOST=localhost DB_USER=postgres DB_PASSWORD=postgres DB_NAME=taskmill_example go run main.go
```

## What the Example Demonstrates

### 1. Migration
- Automatically creates the `taskmill` schema with tables:
  - `task_queue` - main queue for pending tasks
  - `task_results` - completed tasks history
  - `task_schedules` - cron schedule state

### 2. Task Types
- `email.send` - Email notification task
- `report.generate` - Report generation task
- `cleanup.expired` - Scheduled cleanup task
- `health.check` - Scheduled health check task

### 3. Enqueuing Features
- Simple task with payload
- Priority tasks (higher priority processed first)
- Delayed/scheduled tasks
- Custom retry settings
- Ephemeral tasks (no results saved)
- Idempotency keys (duplicate prevention)

### 4. Worker
- Concurrent processing (2 workers)
- Automatic retry with exponential backoff
- Dead letter queue for failed tasks

### 5. Scheduler
- Cron-based scheduling
- Persisted schedule state in database
- Multi-instance safe (uses `FOR UPDATE SKIP LOCKED`)

## Database Tables

After running, check the tables:

```sql
-- View pending tasks
SELECT id, operation_id, payload, priority, attempts FROM taskmill.task_queue;

-- View completed tasks
SELECT id, operation_id, completed_at FROM taskmill.task_results;

-- View schedule state
SELECT operation_id, cron_pattern, next_run_at, last_run_at, run_count FROM taskmill.task_schedules;
```
