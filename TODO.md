# Taskmill Package - Critical Bugs and Fixes

## Critical Issues

### 1. Deadlock in Scheduler

**File:** `pkg/taskmill/scheduler.go:214-280`

**Problem:** The `checkSchedules` method holds a read lock (`RLock`) and calls `scheduleTask`, which tries to acquire a write lock (`Lock`). This is a guaranteed deadlock because Go's `sync.RWMutex` doesn't support lock upgrading.

```go
func (s *scheduler) checkSchedules(now time.Time) {
    s.mu.RLock()          // Read lock acquired
    defer s.mu.RUnlock()

    for _, st := range s.schedulesMap {
        err := s.scheduleTask(st)  // Calls Lock() inside → DEADLOCK
    }
}

func (s *scheduler) scheduleTask(st scheduleTrack) error {
    // ...
    s.mu.Lock()  // Tries to acquire write lock while RLock held → BLOCKS FOREVER
    s.schedulesMap[st.schedule.OperationID] = st
    s.mu.Unlock()
}
```

**How to fix:** Copy schedules to a local slice while holding RLock, release it, then iterate and schedule tasks without holding any lock. Update the map with Lock only when needed.

```go
func (s *scheduler) checkSchedules(now time.Time) {
    s.mu.RLock()
    toCheck := make([]scheduleTrack, 0, len(s.schedulesMap))
    for _, st := range s.schedulesMap {
        toCheck = append(toCheck, st)
    }
    s.mu.RUnlock()  // Release lock before processing

    for _, st := range toCheck {
        if now.Before(st.nextRun) {
            continue
        }
        err := s.scheduleTask(st)
        // ... error handling
    }
}
```

---

### 2. Wrong VisibilityTimeout Causes Duplicate Processing

**File:** `pkg/taskmill/worker.go:166`

**Problem:** The visibility timeout is set to `pollInterval` (default 1s) instead of a proper value. If a task takes longer than 1 second to execute, the message becomes visible again and another worker picks it up, causing duplicate execution.

```go
params := pgqueue.DequeueParams{
    VisibilityTimeout: w.pollInterval,  // BUG: typically 1 second!
}
```

**How to fix:** Use `processTimeout` or a dedicated visibility timeout config value. Visibility timeout should be significantly longer than the expected maximum task execution time.

```go
params := pgqueue.DequeueParams{
    QueueName:         w.queueName,
    MessageGroupID:    w.messageGroupID,
    VisibilityTimeout: w.processTimeout + 30*time.Second,  // Task timeout + buffer
    BatchSize:         w.batchSize,
}
```

Or add a dedicated `visibilityTimeout` field to the worker struct and accept it as a parameter in `NewWorker`.

---

### 3. Missing Transaction Rollback

**File:** `pkg/taskmill/worker.go:226-260`

**Problem:** Both `ackMessage` and `nackMessage` are missing `defer tx.Rollback()`. If `AckTx` or `NackTx` fails, the transaction is never rolled back, potentially causing connection leaks and hung connections.

```go
func (w *worker) ackMessage(ctx context.Context, message pgqueue.Message) error {
    tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
    if err != nil {
        return errx.Wrap(err)
    }
    // MISSING: defer tx.Rollback()

    err = w.queue.AckTx(ctx, &tx, message.ID)
    if err != nil {
        return errx.Wrap(err)  // Transaction left hanging!
    }
    // ...
}
```

**How to fix:** Add `defer tx.Rollback()` after `BeginTx` in both methods. Rollback is a no-op after commit, so this is safe.

```go
func (w *worker) ackMessage(ctx context.Context, message pgqueue.Message) error {
    ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
    defer cancel()

    tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
    if err != nil {
        return errx.Wrap(err)
    }
    defer tx.Rollback()  // ADD THIS LINE

    err = w.queue.AckTx(ctx, &tx, message.ID)
    if err != nil {
        return errx.Wrap(err)
    }

    err = tx.Commit()
    return errx.Wrap(err)
}
```

Apply the same fix to `nackMessage`.

---

### 4. Panic in Middleware Chain Causes Silent ACK

**File:** `pkg/taskmill/worker.go:359-387`

**Problem:** When `processWithRecovery` catches a panic, it logs and alerts but doesn't return an error. Since the function returns `nil` after a panic, `processMessage` will ACK the message (treating it as success), causing silent data loss.

```go
func (w *worker) processWithRecovery(next handleFunc) handleFunc {
    return func(ctx context.Context, m pgqueue.Message) error {
        defer func() {
            if r := recover(); r != nil {
                w.logger.Error("taskmill worker paniced...")
                // No error returned! Returns nil implicitly
            }
        }()
        return next(ctx, m)
    }
}
```

**How to fix:** The recovery wrapper should set and return an error when a panic is caught, so the message gets nacked.

```go
func (w *worker) processWithRecovery(next handleFunc) handleFunc {
    return func(ctx context.Context, m pgqueue.Message) (err error) {
        defer func() {
            if r := recover(); r != nil {
                w.logger.With(
                    "recover", r,
                    "message", m,
                ).Error("taskmill worker paniced at recovery wrapper")

                alertctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
                operationID := fmt.Sprintf("async-task: %s", m.Payload["_operation_id"])

                go func() {
                    defer cancel()
                    _ = alert.SendError(
                        alertctx,
                        "PANIC",
                        "taskmill worker paniced at recovery wrapper",
                        operationID,
                        map[string]string{"recover": fmt.Sprintf("%v", r)},
                    )
                }()

                // SET ERROR so message gets nacked
                err = errx.New("taskmill worker paniced", errx.WithDetails(errx.D{
                    "panic": fmt.Sprintf("%v", r),
                }))
            }
        }()
        return next(ctx, m)
    }
}
```

---

## High Severity Issues

### 5. pgqueue Returns Already-DLQ'd Messages

**File:** `pkg/pgqueue/dequeue.go:71-93`

**Problem:** After moving expired/max-attempts messages to DLQ, they're still returned in the messages slice. Workers will try to process these messages, and ack/nack will fail because they're already in DLQ.

```go
for _, msg := range messages {
    if msg.ExpiresAt != nil && msg.ExpiresAt.Before(time.Now()) {
        err = q.moveToDLQ(ctx, tx, msg.ID, ...)
        // Message still in returned slice!
    }
}
return messages, nil  // Returns all including DLQ'd ones
```

**How to fix:** Filter out messages that were moved to DLQ before returning.

```go
func (q *queue) DequeueTx(ctx context.Context, tx *bun.Tx, params DequeueParams) ([]Message, error) {
    // ... validation and dequeue ...

    // Filter valid messages
    validMessages := make([]Message, 0, len(messages))

    for _, msg := range messages {
        if msg.ExpiresAt != nil && msg.ExpiresAt.Before(time.Now()) {
            err = q.moveToDLQ(ctx, tx, msg.ID, time.Now(), map[string]any{
                "reason": "message's expires_at timestamp has been reached before it could be processed",
            })
            if err != nil {
                return nil, errx.Wrap(err)
            }
            continue  // Skip this message
        }

        if msg.MaxAttempts > 0 && msg.Attempts >= msg.MaxAttempts {
            err = q.moveToDLQ(ctx, tx, msg.ID, time.Now(), map[string]any{
                "reason": "message's attempt counter has already reached or exceeded max_attempts limit",
            })
            if err != nil {
                return nil, errx.Wrap(err)
            }
            continue  // Skip this message
        }

        validMessages = append(validMessages, msg)
    }

    return validMessages, nil
}
```

---

### 6. Idempotency Keys Are Always Unique

**File:** `pkg/taskmill/enqueuer.go:49`

**Problem:** Idempotency key is generated as a random UUID every time, which defeats the purpose of idempotency. The same task with identical payload can be enqueued multiple times.

```go
singleMessage := pgqueue.SingleMessage{
    IdempotencyKey: uuid.NewString(),  // Always unique!
}
```

**How to fix:** Generate idempotency key from task name and payload content, or allow caller to specify it.

Option A - Hash-based key:
```go
func generateIdempotencyKey(operationID string, payload any) string {
    jsonBytes, _ := json.Marshal(payload)
    data := fmt.Sprintf("%s:%s", operationID, string(jsonBytes))
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash[:16])
}
```

Option B - Add option to specify key:
```go
// In options.go
func WithIdempotencyKey(key string) EnqueueOption {
    return func(opts *enqueueOptions) {
        opts.idempotencyKey = &key
    }
}

// In enqueuer.go
idempotencyKey := uuid.NewString()
if options.idempotencyKey != nil {
    idempotencyKey = *options.idempotencyKey
}
```

---

## Medium Severity Issues

### 7. No Input Validation in Worker

**File:** `pkg/taskmill/worker.go:35-63`

**Problem:** `NewWorker` accepts many parameters without validation. Invalid values will cause panics or undefined behavior:
- `concurrency` could be 0 or negative
- `batchSize` could be 0 or negative
- `pollInterval` could be 0 (tight CPU loop)
- `db`, `queue` could be nil

**How to fix:** Add validation at the start of `NewWorker`:

```go
func NewWorker(
    db *bun.DB,
    serviceName string,
    serviceVersion string,
    queue pgqueue.Queue,
    queueName string,
    messageGroupID *string,
    concurrency int,
    pollInterval time.Duration,
    batchSize int,
    processTimeout time.Duration,
) (Worker, error) {
    if db == nil {
        return nil, errx.New("[taskmill]: db is required")
    }
    if queue == nil {
        return nil, errx.New("[taskmill]: queue is required")
    }
    if serviceName == "" {
        return nil, errx.New("[taskmill]: serviceName is required")
    }
    if concurrency < 1 || concurrency > 1000 {
        return nil, errx.New("[taskmill]: concurrency must be between 1 and 1000")
    }
    if batchSize < 1 || batchSize > 100 {
        return nil, errx.New("[taskmill]: batchSize must be between 1 and 100")
    }
    if pollInterval <= 0 {
        return nil, errx.New("[taskmill]: pollInterval must be positive")
    }
    if processTimeout <= 0 {
        return nil, errx.New("[taskmill]: processTimeout must be positive")
    }

    return &worker{
        // ... existing initialization
    }, nil
}
```

---

## Summary

| # | Issue | Severity | File |
|---|-------|----------|------|
| 1 | Deadlock in scheduler | Critical | scheduler.go:214-280 |
| 2 | Wrong VisibilityTimeout | Critical | worker.go:166 |
| 3 | Missing tx.Rollback() | Critical | worker.go:226-260 |
| 4 | Panic causes silent ACK | Critical | worker.go:359-387 |
| 5 | DLQ'd messages returned | High | pgqueue/dequeue.go:71-93 |
| 6 | Idempotency key always unique | High | enqueuer.go:49 |
| 7 | No input validation | Medium | worker.go:35-63 |

Priority order for fixes:
1. **Deadlock** - Will freeze scheduler completely
2. **VisibilityTimeout** - Causes duplicate processing under any load
3. **Missing rollback** - Connection leaks over time
4. **Panic handling** - Silent data loss on panics
5. **DLQ filter** - Confusing errors on expired messages
6. **Idempotency** - Duplicate tasks possible
7. **Validation** - Runtime panics on bad config
