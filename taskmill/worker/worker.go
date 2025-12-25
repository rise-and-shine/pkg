package worker

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/code19m/errx"
	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/alert"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/rise-and-shine/pkg/ucdef"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Error codes for worker operations.
const (
	// CodeTaskNotRegistered is returned when trying to process a task that hasn't been registered.
	CodeTaskNotRegistered = "TASK_NOT_REGISTERED"

	// CodeInvalidPayload is returned when the task payload is invalid or malformed.
	CodeInvalidPayload = "INVALID_PAYLOAD"
)

type Worker interface {
	// RegisterAsyncTask registers an AsyncTask handler/usecase with the worker.
	// The task name is derived from task.OperationID().
	RegisterAsyncTask(task ucdef.AsyncTask[any])

	// Start begins the worker loop.
	// Blocks until Stop is called or context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the worker.
	Stop() error
}

// New creates a new Worker instance.
func New(db *bun.DB, queueName string, opts ...Option) (Worker, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	queue, err := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &worker{
		db:                db,
		queue:             queue,
		queueName:         queueName,
		taskGroupID:       o.taskGroupID,
		concurrency:       o.concurrency,
		pollInterval:      o.pollInterval,
		visibilityTimeout: o.visibilityTimeout,
		batchSize:         o.batchSize,
		processTimeout:    o.processTimeout,

		tasksMap:  make(map[string]ucdef.AsyncTask[any]),
		mu:        sync.RWMutex{},
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
		logger:    logger.Named("taskmill.worker"),
	}, nil
}

const (
	extendedContextTimeout = 3 * time.Second
	shutdownTimeout        = 10 * time.Second
)

type worker struct {
	db *bun.DB

	queue       pgqueue.Queue
	queueName   string
	taskGroupID *string

	concurrency       int
	pollInterval      time.Duration
	visibilityTimeout time.Duration
	batchSize         int
	processTimeout    time.Duration

	tasksMap map[string]ucdef.AsyncTask[any]
	mu       sync.RWMutex

	stopCh    chan struct{}
	stoppedCh chan struct{}

	logger logger.Logger
}

func (w *worker) RegisterAsyncTask(task ucdef.AsyncTask[any]) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.tasksMap[task.OperationID()] = task
}

func (w *worker) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	for range w.concurrency {
		wg.Go(func() {
			w.workerLoop(ctx)
		})
	}

	wg.Wait()
	close(w.stoppedCh)
	return nil
}

func (w *worker) Stop() error {
	close(w.stopCh)

	select {
	case <-w.stoppedCh:
		return nil
	case <-time.After(shutdownTimeout):
		return errx.New("[worker]: shutdown timeout exceeded")
	}
}

func (w *worker) workerLoop(ctx context.Context) {
	chain := w.buildProcessChain()

	for {
		select {
		case <-ctx.Done():
			return

		case <-w.stopCh:
			return

		default:
			tasks, err := w.dequeueTasks(ctx)
			if err != nil {
				w.logger.With("error", err).Error("[worker]: failed to dequeue tasks")
				time.Sleep(w.pollInterval)
				continue
			}

			if len(tasks) == 0 {
				time.Sleep(w.pollInterval)
				continue
			}

			for _, task := range tasks {
				// ignore error, since it's handled in the process chain
				_ = chain(ctx, task)
			}
		}
	}
}

func (w *worker) dequeueTasks(ctx context.Context) ([]pgqueue.Task, error) {
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // intentional

	params := pgqueue.DequeueParams{
		QueueName:         w.queueName,
		TaskGroupID:       w.taskGroupID,
		VisibilityTimeout: w.visibilityTimeout,
		BatchSize:         w.batchSize,
	}

	tasks, err := w.queue.Dequeue(ctx, &tx, params)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return tasks, nil
}

type handleFunc func(context.Context, pgqueue.Task) error

func (w *worker) processTask(ctx context.Context, queueTask pgqueue.Task) error {
	operationID := queueTask.OperationID
	if operationID == "" {
		return errx.New("[worker]: task missing operation_id",
			errx.WithCode(CodeInvalidPayload),
			errx.WithDetails(errx.D{"task": queueTask}))
	}

	w.mu.RLock()
	task, exists := w.tasksMap[operationID]
	w.mu.RUnlock()

	if !exists {
		return errx.New("[worker]: task not registered",
			errx.WithCode(CodeTaskNotRegistered),
			errx.WithDetails(errx.D{"operation_id": operationID}))
	}

	// Pass the payload directly (no envelope)
	err := executeWithRecovery(ctx, task, queueTask.Payload)
	if err != nil {
		nackerr := w.nackTask(ctx, queueTask, errxToMap(err))
		if nackerr != nil {
			w.logger.With(
				"operation_id", operationID,
				"task_id", queueTask.ID,
			).Error("[worker]: failed to nack task: " + nackerr.Error())
		}
	} else {
		ackerr := w.ackTask(ctx, queueTask)
		if ackerr != nil {
			w.logger.With(
				"operation_id", operationID,
				"task_id", queueTask.ID,
			).Error("[worker]: failed to ack task: " + ackerr.Error())
		}
	}

	return err
}

func (w *worker) ackTask(ctx context.Context, queueTask pgqueue.Task) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit

	err = w.queue.Ack(ctx, &tx, queueTask.ID)
	if err != nil {
		return errx.Wrap(err)
	}

	err = tx.Commit()
	return errx.Wrap(err)
}

func (w *worker) nackTask(ctx context.Context, queueTask pgqueue.Task, reason map[string]any) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit

	err = w.queue.Nack(ctx, &tx, queueTask.ID, reason)
	if err != nil {
		return errx.Wrap(err)
	}

	err = tx.Commit()
	return errx.Wrap(err)
}

func (w *worker) buildProcessChain() handleFunc {
	p := w.processTask

	// build the chain in reverse order (last wrapper execute first)
	p = w.processWithLogging(p)       // 6. logging
	p = w.processWithAlerting(p)      // 5. alerting
	p = w.processWithMetaInjection(p) // 4. meta injection
	p = w.processWithTimeout(p)       // 3. timeout
	p = w.processWithTracing(p)       // 2. tracing
	p = w.processWithRecovery(p)      // 1. recovery (outermost)

	return p
}

func (w *worker) processWithLogging(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) error {
		logger := w.logger.Named("access_logger").WithContext(ctx)

		start := time.Now()

		err := next(ctx, t)

		logger = logger.With(
			"task", t,
			"duration", time.Since(start).Round(time.Microsecond),
		)

		if err != nil {
			logger.Errorx(err)
		} else {
			logger.Info("[worker]: task processed successfully")
		}

		return err
	}
}

func (w *worker) processWithAlerting(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) error {
		logger := w.logger.Named("alerting").WithContext(ctx)

		err := next(ctx, t)
		if err == nil {
			return nil
		}

		e := errx.AsErrorX(err)
		operation := fmt.Sprintf("async-task: %s", t.OperationID)
		details := make(map[string]string)
		metaCtx := meta.ExtractMetaFromContext(ctx)
		for k, v := range metaCtx {
			details[string(k)] = v
		}
		details["error_trace"] = e.Trace()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)

		go func() {
			defer cancel()

			senderr := alert.SendError(ctx, e.Code(), err.Error(), operation, details)
			if senderr != nil {
				logger.With("alert_send_error", senderr).Warn("[worker]: failed to send error alert")
			}
		}()

		return err
	}
}

// processWithMetaInjection uses global service info from meta.SetServiceInfo().
func (w *worker) processWithMetaInjection(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) error {
		ctx = context.WithValue(ctx, meta.TraceID, getTraceID(ctx))
		ctx = context.WithValue(ctx, meta.ServiceName, meta.GetServiceName())
		ctx = context.WithValue(ctx, meta.ServiceVersion, meta.GetServiceVersion())
		return next(ctx, t)
	}
}

func (w *worker) processWithTimeout(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) error {
		timeout := w.processTimeout
		if t.ExpiresAt != nil {
			timeout = time.Until(*t.ExpiresAt)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return next(ctx, t)
	}
}

func (w *worker) processWithTracing(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) error {
		// Extract trace context from task meta
		ctx = extractTraceContext(ctx, t.Meta)

		// start a new span
		ctx, span := otel.Tracer("").Start(ctx, fmt.Sprintf("PROCESS %s", t.OperationID),
			trace.WithAttributes(
				semconv.MessagingSystem("pgqueue"),
				semconv.MessagingOperationProcess,
				semconv.MessagingMessageID(strconv.FormatInt(t.ID, 10)),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)

		// call the next handler
		err := next(ctx, t)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		span.End()
		return err
	}
}

// extractTraceContext extracts OpenTelemetry trace context from the task meta.
// If the meta is nil or empty, returns the original context unchanged.
func extractTraceContext(ctx context.Context, meta map[string]string) context.Context {
	if len(meta) == 0 {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, propagation.MapCarrier(meta))
}

func (w *worker) processWithRecovery(next handleFunc) handleFunc {
	return func(ctx context.Context, t pgqueue.Task) (err error) {
		defer func() {
			if r := recover(); r != nil {
				w.logger.
					With(
						"recover", r,
						"task", t,
					).
					Error("[worker]: panicked at recovery wrapper")

				alertctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
				operationID := fmt.Sprintf("async-task: %s", t.OperationID)

				go func() {
					defer cancel()
					_ = alert.SendError(
						alertctx,
						"PANIC",
						"[worker]: panicked at recovery wrapper",
						operationID,
						map[string]string{"recover": fmt.Sprintf("%v", r)},
					)
				}()

				// Return error so task gets nacked instead of acked
				err = errx.New("[worker]: panicked at recovery wrapper", errx.WithDetails(errx.D{
					"panic": fmt.Sprintf("%v", r),
				}))
			}
		}()
		return next(ctx, t)
	}
}

func executeWithRecovery(ctx context.Context, task ucdef.AsyncTask[any], payload any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := make([]byte, 4096) // 4KB
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

			err = errx.New("[worker]: panicked at task execution", errx.WithDetails(errx.D{
				"stack_trace":   string(stackTrace),
				"panic_message": fmt.Sprintf("%v", r),
			}))
		}
	}()
	return task.Execute(ctx, payload)
}

// getTraceID extracts the trace ID from the current span in the context.
// If no trace ID is available, it generates a new UUID to use as a trace ID.
func getTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID()

	if traceID.IsValid() {
		return traceID.String()
	}

	return fmt.Sprintf("man-%s", uuid.New().String())
}

func errxToMap(err error) map[string]any {
	e := errx.AsErrorX(err)
	return map[string]any{
		"code":    e.Code(),
		"type":    e.Type().String(),
		"message": e.Error(),
		"trace":   e.Trace(),
		"details": e.Details(),
	}
}
