# Taskmill Integration Test Cases

This document outlines integration test cases for the taskmill package.
All tests require a PostgreSQL database and test the full system behavior.

---

## Table of Contents

1. [Enqueuer Integration Tests](#1-enqueuer-integration-tests)
2. [Worker Integration Tests](#2-worker-integration-tests)
3. [Scheduler Integration Tests](#3-scheduler-integration-tests)
4. [Console Integration Tests](#4-console-integration-tests)
5. [Full Lifecycle Tests](#5-full-lifecycle-tests)
6. [Concurrency & Race Condition Tests](#6-concurrency--race-condition-tests)
7. [FIFO Ordering Tests](#7-fifo-ordering-tests)
8. [Dead Letter Queue Tests](#8-dead-letter-queue-tests)
9. [Edge Cases & Error Handling](#9-edge-cases--error-handling)
10. [Performance Tests](#10-performance-tests)

---

## 1. Enqueuer Integration Tests

### 1.1 Single Task Enqueue

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEnqueue_Success` | Enqueue task with valid payload | Clean DB | Enqueue task with map payload | Task ID > 0, task exists in DB with correct fields |
| `TestEnqueue_WithAllOptions` | Enqueue with all options | Clean DB | Enqueue with priority=50, maxAttempts=5, scheduledAt=future, expiresAt=future, taskGroupID, idempotencyKey, ephemeral | All options stored correctly in DB |
| `TestEnqueue_DefaultOptions` | Enqueue without options | Clean DB | Enqueue without options | Task has priority=0, maxAttempts=3, scheduledAt≈now, auto-generated idempotencyKey |
| `TestEnqueue_NilPayload` | Enqueue with nil payload | Clean DB | Enqueue with nil payload | Success, payload stored as null |
| `TestEnqueue_StructPayload` | Enqueue with struct payload | Clean DB | Enqueue with custom struct | Success, payload serialized as JSON |
| `TestEnqueue_TracePropagation` | Verify trace context injection | Clean DB, active trace span | Enqueue task | Task meta contains traceparent header |

### 1.2 Batch Enqueue

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEnqueueBatch_Success` | Batch enqueue multiple tasks | Clean DB | EnqueueBatch with 3 tasks | Returns 3 IDs, all tasks in DB |
| `TestEnqueueBatch_EmptyBatch` | Batch enqueue empty slice | Clean DB | EnqueueBatch with [] | Returns [], no error |
| `TestEnqueueBatch_MixedOptions` | Batch with different options per task | Clean DB | EnqueueBatch with varying priorities, maxAttempts | Each task has its own options |
| `TestEnqueueBatch_LargeBatch` | Batch enqueue 100 tasks | Clean DB | EnqueueBatch with 100 tasks | All 100 created successfully |
| `TestEnqueueBatch_SharedTraceContext` | All tasks share trace context | Clean DB, active trace | EnqueueBatch with 5 tasks | All tasks have same traceparent |

### 1.3 Idempotency

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEnqueue_DuplicateIdempotencyKey` | Reject duplicate idempotency key | Task with key "abc" exists | Enqueue with same key | Error with CodeDuplicateTask |
| `TestEnqueue_SameKeyDifferentQueues` | Same key allowed in different queues | Task with key in queue-a | Enqueue same key in queue-b | Success (different queues) |
| `TestEnqueue_KeyReusableAfterCompletion` | Key reusable after task completes | Task completed (in results) | Enqueue same key | Success (not in active queue) |
| `TestEnqueue_KeyReusableAfterDLQ` | Key reusable after task in DLQ | Task in DLQ | Enqueue same key | Success (DLQ excluded from constraint) |
| `TestEnqueueBatch_DuplicateKeyInBatch` | Batch with duplicate keys | Clean DB | Batch with 2 tasks same key | Entire batch fails |
| `TestEnqueueBatch_ConflictWithExisting` | Batch conflicts with existing | Task with key exists | Batch contains same key | Entire batch fails |

### 1.4 Validation Errors

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEnqueue_InvalidPriority_TooHigh` | Priority > 100 | Clean DB | Enqueue with priority=101 | Validation error |
| `TestEnqueue_InvalidPriority_TooLow` | Priority < -100 | Clean DB | Enqueue with priority=-101 | Validation error |
| `TestEnqueue_InvalidMaxAttempts_Zero` | MaxAttempts = 0 | Clean DB | Enqueue with maxAttempts=0 | Validation error |
| `TestEnqueue_EmptyIdempotencyKey` | Empty custom idempotency key | Clean DB | Enqueue with key="" | Validation error |

---

## 2. Worker Integration Tests

### 2.1 Task Processing

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestWorker_ProcessTask_Success` | Worker processes task successfully | Enqueue task, register handler | Start worker, wait | Task completed, in results, not in queue |
| `TestWorker_ProcessTask_Error` | Worker handles task error | Enqueue task, handler returns error | Start worker, wait | Task nacked, attempts incremented, visible_at updated |
| `TestWorker_ProcessTask_Panic` | Worker recovers from panic | Enqueue task, handler panics | Start worker, wait | Task nacked, panic recovered, alert sent |
| `TestWorker_ProcessTask_UnregisteredOperation` | Unknown operationID | Enqueue task with unknown op | Start worker, wait | Task nacked with CodeTaskNotRegistered |
| `TestWorker_ProcessTask_Timeout` | Task exceeds timeout | Enqueue task, handler sleeps 5s | Worker with 1s timeout | Task nacked, context deadline exceeded |
| `TestWorker_ProcessTask_PayloadParsing` | Complex payload correctly passed | Enqueue nested JSON payload | Start worker, capture payload | Handler receives correct payload structure |

### 2.2 Worker Configuration

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestWorker_Concurrency` | Verify concurrent processing | Enqueue 10 tasks, handler takes 100ms | Worker with concurrency=5 | ~200ms total (2 batches of 5) |
| `TestWorker_BatchSize` | Verify batch dequeue | Enqueue 10 tasks | Worker with batchSize=3 | Dequeues 3 at a time |
| `TestWorker_PollInterval` | Verify poll interval | Empty queue | Worker with pollInterval=500ms | Polls every ~500ms |
| `TestWorker_VisibilityTimeout` | Task becomes visible after timeout | Enqueue task, don't ack | Worker with visibility=2s | Task reappears after 2s |
| `TestWorker_ProcessTimeout` | Configurable process timeout | Enqueue task, handler sleeps | Worker with processTimeout=100ms | Task times out at 100ms |
| `TestWorker_TaskGroupFilter` | Worker filters by task group | Enqueue to group-a and group-b | Worker with taskGroup="group-a" | Only processes group-a tasks |

### 2.3 Worker Lifecycle

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestWorker_Start_Stop` | Graceful start and stop | Register tasks | Start, stop after 1s | No errors, clean shutdown |
| `TestWorker_Stop_WithInFlightTasks` | Stop waits for in-flight tasks | Enqueue task, handler takes 2s | Start, immediately stop | Waits for task completion |
| `TestWorker_Stop_Timeout` | Stop times out | Enqueue task, handler takes 30s | Start, stop | Returns timeout error after 10s |
| `TestWorker_ContextCancellation` | Worker respects context | Start worker | Cancel context | Worker exits cleanly |

### 2.4 Retry Behavior

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestWorker_Retry_ExponentialBackoff` | Verify exponential backoff | Enqueue task, handler fails | Process multiple times | Delays: ~1s, ~2s, ~4s |
| `TestWorker_Retry_MaxAttempts` | Task moves to DLQ after max attempts | Enqueue with maxAttempts=2 | Fail twice | Task in DLQ after 2nd failure |
| `TestWorker_Retry_PreservesAttempts` | Attempts count preserved across retries | Enqueue task | Fail, check DB, fail again | Attempts increments correctly |

---

## 3. Scheduler Integration Tests

### 3.1 Schedule Registration

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestScheduler_RegisterSchedules_Success` | Register valid schedules | Clean DB | Register 2 schedules | Both in task_schedules table |
| `TestScheduler_RegisterSchedules_Update` | Update existing schedule | Schedule exists | Register same operationID, different cron | Schedule updated |
| `TestScheduler_RegisterSchedules_Cleanup` | Remove unregistered schedules | Schedules A, B exist | Register only A | Schedule B deleted |
| `TestScheduler_RegisterSchedules_WithOptions` | Store enqueue options | Clean DB | Register with options | Options used when task enqueued |

### 3.2 Cron Pattern Validation

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestScheduler_CronPattern_EveryMinute` | `* * * * *` | Clean DB | Register schedule | Valid, next_run calculated |
| `TestScheduler_CronPattern_Complex` | `*/15 9-17 * * 1-5` | Clean DB | Register schedule | Valid, next_run correct |
| `TestScheduler_CronPattern_Invalid` | `60 * * * *` | Clean DB | Register schedule | Validation error |
| `TestScheduler_CronPattern_WrongFields` | `* * *` | Clean DB | Register schedule | Validation error |

### 3.3 Schedule Execution

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestScheduler_ProcessDueSchedule` | Scheduler enqueues due task | Schedule due now | Start scheduler | Task enqueued, next_run updated |
| `TestScheduler_ProcessDueSchedule_WithOptions` | Enqueue options applied | Schedule with priority=10 | Start scheduler | Enqueued task has priority=10 |
| `TestScheduler_SkipNotDueSchedule` | Skip future schedules | Schedule due in 1 hour | Start scheduler, check | No task enqueued |
| `TestScheduler_ProcessMultipleDue` | Process all due schedules | 3 schedules due | Start scheduler | All 3 tasks enqueued |
| `TestScheduler_Idempotency` | No duplicate tasks | Schedule due, task exists | Start scheduler | No duplicate, no error |
| `TestScheduler_FailedEnqueue` | Handle enqueue failure | Invalid schedule setup | Start scheduler | Schedule marked failed, retry scheduled |

### 3.4 Scheduler Lifecycle

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestScheduler_Start_Stop` | Clean start and stop | Register schedules | Start, stop | No errors |
| `TestScheduler_CheckInterval` | Verify check interval | Schedule due in 2s | Scheduler with 1s interval | Task enqueued after ~2s |

---

## 4. Console Integration Tests

### 4.1 Queue Operations

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestConsole_ListQueues_Empty` | List queues when empty | Clean DB | ListQueues() | Empty slice |
| `TestConsole_ListQueues_Multiple` | List multiple queues | Tasks in queue-a, queue-b | ListQueues() | ["queue-a", "queue-b"] |
| `TestConsole_Stats_Empty` | Stats for empty queue | Clean DB | Stats("queue") | All zeros |
| `TestConsole_Stats_Mixed` | Stats with various task states | Tasks: available, in-flight, scheduled, DLQ | Stats("queue") | Correct counts for each state |
| `TestConsole_Purge` | Purge pending tasks | 5 pending tasks, 2 DLQ | Purge("queue") | 5 deleted, DLQ preserved |
| `TestConsole_PurgeDLQ` | Purge DLQ tasks | 5 pending, 2 DLQ | PurgeDLQ("queue") | DLQ empty, pending preserved |

### 4.2 DLQ Operations

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestConsole_RequeueFromDLQ_Success` | Requeue task from DLQ | Task in DLQ | RequeueFromDLQ(taskID) | Task back in queue, dlq_at cleared, attempts reset |
| `TestConsole_RequeueFromDLQ_NotInDLQ` | Requeue non-DLQ task | Task pending (not DLQ) | RequeueFromDLQ(taskID) | Error |
| `TestConsole_RequeueFromDLQ_NotFound` | Requeue non-existent task | Clean DB | RequeueFromDLQ(999) | Error |

### 4.3 Schedule Operations

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestConsole_ListSchedules_All` | List all schedules | 3 schedules | ListSchedules(nil) | All 3 returned |
| `TestConsole_ListSchedules_ByQueue` | Filter by queue | Schedules in queue-a, queue-b | ListSchedules("queue-a") | Only queue-a schedules |
| `TestConsole_TriggerSchedule_Success` | Manually trigger schedule | Schedule registered | TriggerSchedule("op") | Task enqueued immediately |
| `TestConsole_TriggerSchedule_WithOptions` | Trigger with options | Schedule registered | TriggerSchedule("op", WithPriority(50)) | Task has priority=50 |
| `TestConsole_TriggerSchedule_NotFound` | Trigger non-existent | No schedules | TriggerSchedule("unknown") | Error CodeScheduleNotFound |

### 4.4 Results Operations

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestConsole_ListResults_All` | List all results | 5 completed tasks | ListResults({Limit:10}) | All 5 returned |
| `TestConsole_ListResults_ByQueue` | Filter by queue | Results in queue-a, queue-b | ListResults({QueueName:"queue-a"}) | Only queue-a results |
| `TestConsole_ListResults_Pagination` | Test pagination | 10 results | ListResults({Limit:3, Offset:2}) | Results 3-5 |
| `TestConsole_ListResults_ByTimeRange` | Filter by time | Results over time | ListResults({CompletedAfter, CompletedBefore}) | Only matching results |
| `TestConsole_CleanupResults_ByAge` | Cleanup old results | Old and new results | CleanupResults({CompletedBefore: 1h ago}) | Old deleted, new preserved |
| `TestConsole_CleanupResults_ByQueue` | Cleanup by queue | Results in queue-a, queue-b | CleanupResults({QueueName:"queue-a"}) | Only queue-a cleaned |

---

## 5. Full Lifecycle Tests

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestLifecycle_EnqueueToCompletion` | Full task lifecycle | Register handler | Enqueue → Worker processes → Ack | Task in results, not in queue |
| `TestLifecycle_EnqueueToRetryToCompletion` | Retry then success | Handler fails once then succeeds | Enqueue, wait | Attempts=2 in results |
| `TestLifecycle_EnqueueToDLQ` | Task fails to DLQ | Handler always fails, maxAttempts=3 | Enqueue, wait for 3 failures | Task in DLQ |
| `TestLifecycle_DLQToRequeueToCompletion` | Full DLQ recovery | Task in DLQ, fix handler | Requeue, wait | Task completed |
| `TestLifecycle_ScheduleToCompletion` | Scheduled task lifecycle | Schedule due now, handler registered | Start scheduler + worker | Task enqueued and completed |
| `TestLifecycle_EphemeralTask` | Ephemeral task not saved | Enqueue ephemeral task | Process | Task deleted, NOT in results |
| `TestLifecycle_PriorityOrdering` | High priority first | Enqueue low=-50, high=50 | Process | High processed first |
| `TestLifecycle_ScheduledTask_FutureExecution` | Task waits for scheduled time | Enqueue scheduledAt=+5s | Start worker | Task processed after 5s |
| `TestLifecycle_ExpiredTask` | Expired task moves to DLQ | Enqueue expiresAt=past | Dequeue | Task in DLQ immediately |

---

## 6. Concurrency & Race Condition Tests

### 6.1 Concurrent Workers

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestConcurrency_MultipleWorkers_SameQueue` | No duplicate processing | Enqueue 100 tasks | Start 5 workers | Each task processed exactly once |
| `TestConcurrency_SkipLocked` | SKIP LOCKED works | Enqueue 10 tasks | 3 workers dequeue simultaneously | No conflicts, all tasks processed |
| `TestConcurrency_HighThroughput` | High volume test | Enqueue 1000 tasks | 10 workers, concurrency=5 each | All tasks completed |
| `TestConcurrency_WorkerInternalConcurrency` | Worker goroutines | Enqueue 20 tasks | 1 worker, concurrency=10 | Processes 10 in parallel |

### 6.2 Race Conditions

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestRace_ConcurrentEnqueue_SameKey` | Only one succeeds with same key | - | 10 goroutines enqueue same key | Exactly 1 succeeds, 9 fail |
| `TestRace_ConcurrentDequeue` | No double processing | 50 tasks | 10 workers race to dequeue | Each task processed once |
| `TestRace_EnqueueDuringProcessing` | Enqueue while processing | Worker running | Enqueue new tasks | Both operations succeed |
| `TestRace_StopDuringProcessing` | Stop while processing | Task being processed | Call Stop() | Task completes or visibility restored |
| `TestRace_VisibilityTimeout_Reprocess` | No reprocess during handling | Task dequeued, processing | Wait for visibility timeout | Same task not reprocessed |

### 6.3 Scheduler Concurrency

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestRace_MultipleSchedulers` | Multiple schedulers | Schedule due | Start 3 schedulers | Task enqueued exactly once |
| `TestRace_SchedulerDuringRegister` | Register during run | Scheduler running | Call RegisterSchedules | Safe registration |

---

## 7. FIFO Ordering Tests

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestFIFO_SingleGroup_Ordering` | Tasks in group processed in order | Enqueue tasks 1,2,3,4,5 to group-a | Start worker | Processed in order 1→2→3→4→5 |
| `TestFIFO_MultipleGroups_Independent` | Groups are independent | Enqueue to group-a and group-b | Start worker | Each group ordered independently |
| `TestFIFO_AdvisoryLock_Serialization` | Advisory lock ensures serial | Enqueue 5 tasks to group-a | 3 workers | Only 1 worker processes at a time |
| `TestFIFO_LockRelease_OnCompletion` | Lock released after task | Enqueue 2 tasks to group | Process first | Second becomes processable |
| `TestFIFO_LockRelease_OnFailure` | Lock released on failure | Enqueue task to group, fails | Process | Retry can acquire lock |
| `TestFIFO_WorkerWithGroupFilter` | Worker filters by group | Tasks in group-a, group-b | Worker(taskGroup="group-a") | Only group-a processed |
| `TestFIFO_Priority_WithinGroup` | Priority ordering within group | Enqueue to group with priorities | Process | Higher priority first |

---

## 8. Dead Letter Queue Tests

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestDLQ_MaxAttemptsExceeded` | Task to DLQ after max attempts | Enqueue maxAttempts=2, handler fails | Process twice | Task in DLQ |
| `TestDLQ_Expired` | Expired task to DLQ | Enqueue expiresAt=past | Dequeue | Task in DLQ with expiry reason |
| `TestDLQ_Reason_Error` | Error reason captured | Handler returns error | Process | dlq_reason contains error |
| `TestDLQ_Reason_Panic` | Panic reason captured | Handler panics | Process | dlq_reason contains panic info |
| `TestDLQ_Stats` | DLQ count in stats | 3 tasks in DLQ | Stats() | InDLQ=3 |
| `TestDLQ_NotDequeued` | DLQ tasks not dequeued | Tasks in DLQ | Dequeue | Returns empty |
| `TestDLQ_RequeueSuccess` | Requeue from DLQ | Task in DLQ | RequeueFromDLQ, process | Task completed |
| `TestDLQ_RequeueResetsAttempts` | Requeue resets attempts | Task in DLQ (attempts=3) | RequeueFromDLQ | Attempts reset to 0 |

---

## 9. Edge Cases & Error Handling

### 9.1 Payload Edge Cases

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEdge_NilPayload` | Nil payload | - | Enqueue with nil | Success |
| `TestEdge_EmptyMapPayload` | Empty map payload | - | Enqueue with {} | Success |
| `TestEdge_LargePayload` | Large payload (1MB) | - | Enqueue large JSON | Success |
| `TestEdge_NestedPayload` | Deeply nested JSON | - | Enqueue nested structure | Correctly stored/retrieved |
| `TestEdge_UnicodePayload` | Unicode in payload | - | Enqueue with unicode | Preserved correctly |
| `TestEdge_SpecialChars_OperationID` | Special chars in operationID | - | Enqueue "op/test:1" | Works correctly |

### 9.2 Timing Edge Cases

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEdge_ScheduledAt_Past` | Past scheduledAt | - | Enqueue scheduledAt=-1h | Immediately available |
| `TestEdge_ScheduledAt_Now` | Now scheduledAt | - | Enqueue scheduledAt=now | Immediately available |
| `TestEdge_ExpiresAt_Past` | Already expired | - | Enqueue expiresAt=past | To DLQ on dequeue |
| `TestEdge_VisibilityTimeout_Short` | Very short visibility | Worker with visibility=1s | Dequeue, sleep 2s | Task becomes visible |
| `TestEdge_ProcessTimeout_ExpiresAt` | ExpiresAt as timeout | Enqueue expiresAt=+1s, handler sleeps | Process | Timeout from expiresAt |

### 9.3 Numeric Edge Cases

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEdge_Priority_Min` | priority = -100 | - | Enqueue | Success |
| `TestEdge_Priority_Max` | priority = 100 | - | Enqueue | Success |
| `TestEdge_MaxAttempts_One` | maxAttempts = 1 | - | Enqueue, fail once | Immediately to DLQ |
| `TestEdge_MaxAttempts_Large` | maxAttempts = 100 | - | Enqueue | Success, allows 100 retries |

### 9.4 State Edge Cases

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEdge_EmptyQueue_Dequeue` | Dequeue empty queue | Clean DB | Dequeue | Empty result, no error |
| `TestEdge_EmptyQueue_Stats` | Stats empty queue | Clean DB | Stats | All zeros |
| `TestEdge_NoSchedules_Start` | Scheduler with no schedules | No schedules | Start scheduler | Runs, does nothing |
| `TestEdge_NoTasks_Worker` | Worker with no registered tasks | Tasks in queue, no handlers | Start worker | Tasks nacked as unregistered |
| `TestEdge_AllTasksInDLQ` | All tasks in DLQ | 5 tasks all in DLQ | Dequeue | Empty result |
| `TestEdge_AllTasksScheduled` | All tasks scheduled future | 5 tasks scheduledAt=+1h | Dequeue | Empty result |
| `TestEdge_AllTasksInFlight` | All tasks in-flight | 5 tasks, visibility_at=+1h | Dequeue | Empty result |

### 9.5 Database Edge Cases

| Test Case | Description | Setup | Action | Expected Result |
|-----------|-------------|-------|--------|-----------------|
| `TestEdge_Transaction_Rollback` | Rollback doesn't persist | Start transaction | Enqueue, rollback | No task in DB |
| `TestEdge_Transaction_Commit` | Commit persists | Start transaction | Enqueue, commit | Task in DB |
| `TestEdge_ConcurrentMigration` | Safe concurrent migration | - | 2 goroutines call Migrate | Both succeed (IF NOT EXISTS) |

---

## 10. Performance Tests

| Test Case | Description | Metric | Expected |
|-----------|-------------|--------|----------|
| `TestPerf_EnqueueThroughput` | Enqueue 10,000 tasks | ops/sec | > 1000 ops/sec |
| `TestPerf_EnqueueBatch_vs_Single` | Compare batch vs single | time | Batch significantly faster |
| `TestPerf_DequeueThroughput` | Dequeue 10,000 tasks | ops/sec | > 500 ops/sec |
| `TestPerf_ProcessThroughput` | Process 10,000 no-op tasks | ops/sec | > 500 ops/sec |
| `TestPerf_ConcurrentWorkers` | 10 workers, 10,000 tasks | total time | Near-linear scaling |
| `TestPerf_LargeQueue` | Dequeue with 100,000 tasks | dequeue time | Consistent (index used) |
| `TestPerf_IndexUsage` | Verify indexes used | EXPLAIN | No sequential scans |
| `TestPerf_FIFO_LockContention` | High contention on groups | lock wait | Acceptable wait times |

---

## Test Infrastructure

### Setup Functions

```go
// NewTestDB creates test database connection
func NewTestDB(t *testing.T) *bun.DB

// CleanupDB clears all taskmill tables
func CleanupDB(t *testing.T, db *bun.DB)

// MigrateTestDB runs migrations
func MigrateTestDB(t *testing.T, db *bun.DB)

// NewTestEnqueuer creates enqueuer for queue
func NewTestEnqueuer(t *testing.T, queueName string) enqueuer.Enqueuer

// NewTestWorker creates worker with options
func NewTestWorker(t *testing.T, db *bun.DB, queueName string, opts ...worker.Option) worker.Worker

// NewTestScheduler creates scheduler
func NewTestScheduler(t *testing.T, db *bun.DB, queueName string, opts ...scheduler.Option) scheduler.Scheduler

// NewTestConsole creates console
func NewTestConsole(t *testing.T, db *bun.DB) console.Console
```

### Wait Helpers

```go
// WaitForTaskProcessed waits for task completion
func WaitForTaskProcessed(t *testing.T, db *bun.DB, taskID int64, timeout time.Duration)

// WaitForQueueEmpty waits for queue to be empty
func WaitForQueueEmpty(t *testing.T, db *bun.DB, queueName string, timeout time.Duration)

// WaitForDLQCount waits for DLQ to reach count
func WaitForDLQCount(t *testing.T, db *bun.DB, queueName string, count int, timeout time.Duration)

// WaitForResultsCount waits for results to reach count
func WaitForResultsCount(t *testing.T, db *bun.DB, queueName string, count int, timeout time.Duration)
```

### Assert Helpers

```go
// AssertTaskInQueue verifies task in active queue
func AssertTaskInQueue(t *testing.T, db *bun.DB, taskID int64)

// AssertTaskInDLQ verifies task in DLQ
func AssertTaskInDLQ(t *testing.T, db *bun.DB, taskID int64)

// AssertTaskInResults verifies task completed
func AssertTaskInResults(t *testing.T, db *bun.DB, taskID int64)

// AssertTaskNotExists verifies task deleted
func AssertTaskNotExists(t *testing.T, db *bun.DB, taskID int64)

// AssertQueueStats verifies queue statistics
func AssertQueueStats(t *testing.T, db *bun.DB, queueName string, expected console.QueueStats)
```

### Mock Tasks

```go
// SuccessTask always succeeds
type SuccessTask struct {
    OperationIDValue string
    ExecutedCount    atomic.Int32
    LastPayload      atomic.Value
}

// FailingTask always fails with error
type FailingTask struct {
    OperationIDValue string
    Error            error
    FailCount        atomic.Int32
}

// PanicTask always panics
type PanicTask struct {
    OperationIDValue string
    PanicValue       any
}

// SlowTask takes duration to execute
type SlowTask struct {
    OperationIDValue string
    Duration         time.Duration
}

// ConditionalTask fails N times then succeeds
type ConditionalTask struct {
    OperationIDValue string
    FailTimes        int
    currentCount     atomic.Int32
}

// OrderTrackingTask records execution order
type OrderTrackingTask struct {
    OperationIDValue string
    Order            *[]int64  // shared slice to track order
    mu               sync.Mutex
}
```

---

## Test Organization

```
taskmill/
├── console/
│   └── console_test.go      # Console integration tests
├── enqueuer/
│   └── enqueuer_test.go     # Enqueuer integration tests
├── scheduler/
│   └── scheduler_test.go    # Scheduler integration tests
├── worker/
│   └── worker_test.go       # Worker integration tests
├── integration_test.go      # Cross-component tests
├── race_test.go             # Race condition tests (run with -race)
├── fifo_test.go             # FIFO ordering tests
├── dlq_test.go              # DLQ-specific tests
├── edge_test.go             # Edge cases
├── bench_test.go            # Performance benchmarks
└── testutil/
    ├── db.go                # Database utilities
    ├── wait.go              # Wait helpers
    ├── assert.go            # Custom assertions
    └── mocks.go             # Mock task implementations
```

---

## Running Tests

```bash
# All integration tests (requires PostgreSQL)
DB_HOST=localhost DB_USER=postgres DB_PASSWORD=postgres DB_NAME=taskmill_test \
  go test ./...

# With race detection
go test -race ./...

# Specific test file
go test -v ./worker/worker_test.go

# Specific test
go test -v -run TestWorker_ProcessTask_Success ./...

# Benchmarks
go test -bench=. -benchmem ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose with timeout
go test -v -timeout 5m ./...
```

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `DB_USER` | PostgreSQL user | postgres |
| `DB_PASSWORD` | PostgreSQL password | postgres |
| `DB_NAME` | Test database name | taskmill_test |
| `DB_SSLMODE` | SSL mode | disable |

---

## Notes

1. **Database Required**: All tests require a running PostgreSQL instance
2. **Isolation**: Each test should clean up after itself using `t.Cleanup()`
3. **Parallelism**: Use `t.Parallel()` for independent tests
4. **Timeouts**: Use reasonable timeouts (5-30s) to catch hangs
5. **Race Detection**: Run with `-race` regularly
6. **Cleanup**: Tests should leave database clean for next test
