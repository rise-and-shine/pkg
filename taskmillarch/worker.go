package taskmill

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/pgqueue"
	"github.com/rise-and-shine/pkg/ucdef"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Worker handles task registration, enqueueing, and execution.
type Worker interface {
	// RegisterTask registers an AsyncTask handler with the worker.
	// The task name is derived from task.OperationID().
	RegisterTask(task ucdef.AsyncTask[any]) error

	// Enqueue enqueues a task for execution.
	// Returns the message ID from pgqueue.
	Enqueue(ctx context.Context, taskName string, payload any, opts ...EnqueueOption) (int64, error)

	// EnqueueTx enqueues a task within an existing transaction.
	EnqueueTx(ctx context.Context, tx *bun.Tx, taskName string, payload any, opts ...EnqueueOption) (int64, error)

	// Start begins the worker loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the worker.
	Stop() error
}

// registeredTask holds a task handler with type-erased execution.
type registeredTask struct {
	name    string
	handler func(ctx context.Context, payload any) error
}

// worker is the concrete implementation of Worker.
type worker struct {
	cfg       WorkerConfig
	logger    logger.Logger
	tasks     map[string]*registeredTask
	tasksMu   sync.RWMutex
	stopCh    chan struct{}
	stoppedCh chan struct{}
	wg        sync.WaitGroup
}

// NewWorker creates a new Worker instance.
func NewWorker(cfg WorkerConfig) (Worker, error) {
	applyWorkerDefaults(&cfg)

	if err := validateWorkerConfig(&cfg); err != nil {
		return nil, errx.Wrap(err)
	}

	return &worker{
		cfg:       cfg,
		logger:    cfg.Logger.Named("taskmill.worker"),
		tasks:     make(map[string]*registeredTask),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}, nil
}

// RegisterTask registers an AsyncTask handler.
func (w *worker) RegisterTask(task ucdef.AsyncTask[any]) error {
	name := task.OperationID()

	w.tasksMu.Lock()
	defer w.tasksMu.Unlock()

	if _, exists := w.tasks[name]; exists {
		return errx.New("[taskmill.worker]: task already registered", errx.WithDetails(errx.D{
			"task_name": name,
		}))
	}

	w.tasks[name] = &registeredTask{
		name: name,
		handler: func(ctx context.Context, payload any) error {
			return task.Execute(ctx, payload)
		},
	}

	w.logger.With("task_name", name).Info("[taskmill.worker] task registered")
	return nil
}

// Enqueue enqueues a task for execution.
func (w *worker) Enqueue(ctx context.Context, taskName string, payload any, opts ...EnqueueOption) (int64, error) {
	var messageID int64

	err := w.cfg.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		id, err := w.EnqueueTx(ctx, &tx, taskName, payload, opts...)
		if err != nil {
			return err
		}
		messageID = id
		return nil
	})

	if err != nil {
		return 0, errx.Wrap(err)
	}

	return messageID, nil
}

// EnqueueTx enqueues a task within an existing transaction.
func (w *worker) EnqueueTx(
	ctx context.Context,
	tx *bun.Tx,
	taskName string,
	payload any,
	opts ...EnqueueOption,
) (int64, error) {
	// Verify task is registered
	w.tasksMu.RLock()
	_, exists := w.tasks[taskName]
	w.tasksMu.RUnlock()

	if !exists {
		return 0, errx.New("[taskmill.worker]: task not registered", errx.WithDetails(errx.D{
			"task_name": taskName,
		}))
	}

	// Build options
	options := &enqueueOptions{
		queueName:   w.cfg.QueueName,
		priority:    0,
		maxAttempts: w.cfg.DefaultMaxAttempts,
		scheduledAt: time.Now(),
	}
	for _, opt := range opts {
		opt(options)
	}

	// Serialize payload
	serialized, err := serializePayload(taskName, payload)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	// Generate idempotency key
	idempotencyKey := generateIdempotencyKey(taskName, payload)

	// Create message
	message := pgqueue.SingleMessage{
		Payload:        serialized,
		IdempotencyKey: idempotencyKey,
		MessageGroupID: options.messageGroupID,
		Priority:       options.priority,
		ScheduledAt:    options.scheduledAt,
		MaxAttempts:    options.maxAttempts,
		ExpiresAt:      options.expiresAt,
	}

	// Enqueue
	messageIDs, err := w.cfg.Queue.EnqueueBatchTx(ctx, tx, options.queueName, []pgqueue.SingleMessage{message})
	if err != nil {
		return 0, errx.Wrap(err)
	}

	if len(messageIDs) == 0 {
		return 0, errx.New("[taskmill.worker]: no message IDs returned")
	}

	w.logger.With(
		"task_name", taskName,
		"message_id", messageIDs[0],
		"queue_name", options.queueName,
	).Debug("[taskmill.worker] task enqueued")

	return messageIDs[0], nil
}

// Start begins the worker loop.
func (w *worker) Start(ctx context.Context) error {
	w.logger.With(
		"concurrency", w.cfg.Concurrency,
		"queue", w.cfg.QueueName,
		"batch_size", w.cfg.BatchSize,
	).Info("[taskmill.worker] starting")

	for i := range w.cfg.Concurrency {
		w.wg.Add(1)
		go w.workerLoop(ctx, i)
	}

	w.wg.Wait()
	close(w.stoppedCh)

	w.logger.Info("[taskmill.worker] stopped")
	return nil
}

// Stop gracefully shuts down the worker.
func (w *worker) Stop() error {
	w.logger.Info("[taskmill.worker] stopping")
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(w.cfg.ShutdownTimeout):
		return errx.New("[taskmill.worker]: shutdown timeout exceeded")
	}
}

// workerLoop is the main loop for each worker goroutine.
func (w *worker) workerLoop(ctx context.Context, workerID int) {
	defer w.wg.Done()

	log := w.logger.With("worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			messages, err := w.dequeueMessages(ctx)
			if err != nil {
				log.Errorx(err)
				time.Sleep(w.cfg.PollInterval)
				continue
			}

			if len(messages) == 0 {
				time.Sleep(w.cfg.PollInterval)
				continue
			}

			for _, msg := range messages {
				w.processMessage(ctx, msg, log)
			}
		}
	}
}

// dequeueMessages dequeues messages from the queue.
func (w *worker) dequeueMessages(ctx context.Context) ([]pgqueue.Message, error) {
	var messages []pgqueue.Message

	err := w.cfg.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		params := pgqueue.DequeueParams{
			QueueName:         w.cfg.QueueName,
			MessageGroupID:    w.cfg.MessageGroupID,
			VisibilityTimeout: w.cfg.VisibilityTimeout,
			BatchSize:         w.cfg.BatchSize,
		}

		var err error
		messages, err = w.cfg.Queue.DequeueTx(ctx, &tx, params)
		return err
	})

	return messages, err
}

// processMessage processes a single message.
func (w *worker) processMessage(ctx context.Context, msg pgqueue.Message, log logger.Logger) {
	taskName := fmt.Sprintf("%v", msg.Payload["_task_name"])
	startedAt := time.Now()

	// Build handler chain
	handler := w.buildHandlerChain(taskName, msg.ID, msg.Attempts, msg.MaxAttempts, startedAt)

	// Execute in transaction
	err := w.cfg.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := handler(ctx, msg); err != nil {
			// Nack on error
			return w.cfg.Queue.NackTx(ctx, &tx, msg.ID, map[string]any{
				"error":   err.Error(),
				"attempt": msg.Attempts,
			})
		}
		// Ack on success
		return w.cfg.Queue.AckTx(ctx, &tx, msg.ID)
	})

	if err != nil {
		log.With("message_id", msg.ID, "task_name", taskName).Errorx(err)
	}
}

// handlerFunc is the task handler signature.
type handlerFunc func(context.Context, pgqueue.Message) error

// buildHandlerChain builds the middleware chain.
func (w *worker) buildHandlerChain(
	taskName string,
	messageID int64,
	attempt, maxAttempts int,
	startedAt time.Time,
) handlerFunc {
	handler := w.executeTask

	handler = w.wrapLogging(taskName, messageID, attempt, maxAttempts, startedAt, handler)
	handler = w.wrapAlerting(taskName, messageID, handler)
	handler = w.wrapMetaInjection(taskName, handler)
	handler = w.wrapTimeout(handler)
	handler = w.wrapTracing(taskName, messageID, attempt, handler)
	handler = w.wrapRecovery(taskName, messageID, handler)

	return handler
}

// executeTask executes the registered task handler.
func (w *worker) executeTask(ctx context.Context, msg pgqueue.Message) error {
	taskName, ok := msg.Payload["_task_name"].(string)
	if !ok {
		return errx.New("[taskmill.worker]: message missing _task_name")
	}

	w.tasksMu.RLock()
	task, exists := w.tasks[taskName]
	w.tasksMu.RUnlock()

	if !exists {
		return errx.New("[taskmill.worker]: task not registered", errx.WithDetails(errx.D{
			"task_name": taskName,
		}))
	}

	// Deserialize payload (remove internal fields)
	payload := make(map[string]any)
	for k, v := range msg.Payload {
		if k != "_task_name" {
			payload[k] = v
		}
	}

	return task.handler(ctx, payload)
}

// wrapRecovery catches panics.
func (w *worker) wrapRecovery(taskName string, messageID int64, next handlerFunc) handlerFunc {
	return func(ctx context.Context, msg pgqueue.Message) (err error) {
		defer func() {
			if r := recover(); r != nil {
				w.logger.With("task_name", taskName, "message_id", messageID, "panic", r).
					Error("[taskmill.worker] panic recovered")
				err = errx.New("[taskmill.worker]: panic recovered", errx.WithDetails(errx.D{"panic": r}))
			}
		}()
		return next(ctx, msg)
	}
}

// wrapTracing adds OpenTelemetry tracing.
func (w *worker) wrapTracing(taskName string, messageID int64, attempt int, next handlerFunc) handlerFunc {
	if !w.cfg.EnableTracing {
		return next
	}

	return func(ctx context.Context, msg pgqueue.Message) error {
		ctx, span := otel.Tracer("taskmill").Start(ctx, fmt.Sprintf("TASK %s", taskName),
			trace.WithAttributes(
				attribute.String("task.name", taskName),
				attribute.Int64("task.message_id", messageID),
				attribute.String("task.queue", w.cfg.QueueName),
				attribute.Int("task.attempt", attempt),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		err := next(ctx, msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// wrapTimeout adds execution timeout.
func (w *worker) wrapTimeout(next handlerFunc) handlerFunc {
	return func(ctx context.Context, msg pgqueue.Message) error {
		ctx, cancel := context.WithTimeout(ctx, w.cfg.TaskTimeout)
		defer cancel()
		return next(ctx, msg)
	}
}

// wrapMetaInjection adds metadata to context.
func (w *worker) wrapMetaInjection(taskName string, next handlerFunc) handlerFunc {
	return func(ctx context.Context, msg pgqueue.Message) error {
		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		if traceID == "00000000000000000000000000000000" {
			traceID = fmt.Sprintf("%x-%d", time.Now().UnixNano(), msg.ID)
		}

		ctx = context.WithValue(ctx, meta.TraceID, traceID)
		ctx = context.WithValue(ctx, meta.ActorType, ucdef.TypeAsyncTask)
		ctx = context.WithValue(ctx, meta.ActorID, taskName)
		ctx = context.WithValue(ctx, meta.ServiceName, w.cfg.ServiceName)
		ctx = context.WithValue(ctx, meta.ServiceVersion, w.cfg.ServiceVersion)

		return next(ctx, msg)
	}
}

// wrapAlerting sends alerts on errors.
func (w *worker) wrapAlerting(taskName string, messageID int64, next handlerFunc) handlerFunc {
	if !w.cfg.EnableAlerting {
		return next
	}

	return func(ctx context.Context, msg pgqueue.Message) error {
		err := next(ctx, msg)
		if err != nil {
			w.logger.Named("alert").WithContext(ctx).
				With("task_name", taskName, "message_id", messageID).
				Errorx(err)
		}
		return err
	}
}

// wrapLogging logs task execution.
func (w *worker) wrapLogging(
	taskName string,
	messageID int64,
	attempt, maxAttempts int,
	startedAt time.Time,
	next handlerFunc,
) handlerFunc {
	return func(ctx context.Context, msg pgqueue.Message) error {
		err := next(ctx, msg)
		duration := time.Since(startedAt)

		log := w.logger.WithContext(ctx).With(
			"task_name", taskName,
			"message_id", messageID,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"duration_ms", duration.Milliseconds(),
		)

		if err != nil {
			log.Errorx(err)
		} else {
			log.Debug("[taskmill.worker] task completed")
		}

		return err
	}
}

// serializePayload converts payload to pgqueue format.
func serializePayload(taskName string, payload any) (map[string]any, error) {
	if payload == nil {
		return map[string]any{"_task_name": taskName}, nil
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	var result map[string]any
	if unmarshalErr := json.Unmarshal(jsonBytes, &result); unmarshalErr != nil {
		return nil, errx.Wrap(unmarshalErr)
	}

	result["_task_name"] = taskName
	return result, nil
}

// generateIdempotencyKey creates a unique key.
func generateIdempotencyKey(taskName string, payload any) string {
	jsonBytes, _ := json.Marshal(payload)
	data := fmt.Sprintf("%s:%s:%d", taskName, string(jsonBytes), time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s-%x", taskName, hash[:8])
}
