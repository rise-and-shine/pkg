package taskmill

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/code19m/errx"
	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/alert"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/rise-and-shine/pkg/ucdef"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
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

// NewWorker creates a new Worker instance.
func NewWorker(config *WorkerConfig) (Worker, error) {
	config.setDefaults()

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &worker{
		db:                config.DB,
		serviceName:       config.ServiceName,
		serviceVersion:    config.ServiceVersion,
		queue:             config.Queue,
		queueName:         config.QueueName,
		messageGroupID:    config.MessageGroupID,
		concurrency:       config.Concurrency,
		pollInterval:      config.PollInterval,
		visibilityTimeout: config.VisibilityTimeout,
		batchSize:         config.BatchSize,
		processTimeout:    config.ProcessTimeout,
		tasksMap:          make(map[string]ucdef.AsyncTask[any]),
		stopCh:            make(chan struct{}),
		stoppedCh:         make(chan struct{}),
		logger:            logger.Named("taskmill.worker"),
	}, nil
}

const (
	extendedContextTimeout = 3 * time.Second
)

type worker struct {
	db *bun.DB

	serviceName    string
	serviceVersion string

	queue          pgqueue.Queue
	queueName      string
	messageGroupID *string

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
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerLoop(ctx)
		}()
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
		return errx.New("[taskmill]: worker shutdown timeout exceeded")
	}
}

func (w *worker) workerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-w.stopCh:
			return

		default:
			messages, err := w.dequeueMessages(ctx)
			if err != nil {
				w.logger.With("error", err).Error("[taskmill]: worker failed to dequeue messages")
				time.Sleep(w.pollInterval)
				continue
			}

			if len(messages) == 0 {
				time.Sleep(w.pollInterval)
				continue
			}

			for _, msg := range messages {
				// ignore error, since it's handled in the process chain
				chain := w.buildProcessChain()
				_ = chain(ctx, msg)
			}
		}
	}
}

func (w *worker) dequeueMessages(ctx context.Context) ([]pgqueue.Message, error) {
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // intentional

	params := pgqueue.DequeueParams{
		QueueName:         w.queueName,
		MessageGroupID:    w.messageGroupID,
		VisibilityTimeout: w.visibilityTimeout,
		BatchSize:         w.batchSize,
	}

	messages, err := w.queue.Dequeue(ctx, &tx, params)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return messages, nil
}

type handleFunc func(context.Context, pgqueue.Message) error

func (w *worker) processMessage(ctx context.Context, message pgqueue.Message) error {
	operationID, ok := message.Payload["_operation_id"].(string)
	if !ok {
		return errx.New("[taskmill]: task message missing _operation_id",
			errx.WithCode(CodeInvalidPayload),
			errx.WithDetails(errx.D{"message": message}))
	}

	taskPayload, ok := message.Payload["payload"]
	if !ok {
		return errx.New("[taskmill]: task message missing payload",
			errx.WithCode(CodeInvalidPayload),
			errx.WithDetails(errx.D{"message": message}))
	}

	w.mu.RLock()
	task, exists := w.tasksMap[operationID]
	w.mu.RUnlock()

	if !exists {
		return errx.New("[taskmill]: task not registered",
			errx.WithCode(CodeTaskNotRegistered),
			errx.WithDetails(errx.D{"operation_id": operationID}))
	}

	err := executeWithRecovery(ctx, task, taskPayload)
	if err != nil {
		nackerr := w.nackMessage(ctx, message, errxToMap(err))
		if nackerr != nil {
			w.logger.With(
				"operation_id", operationID,
				"message_id", message.ID,
			).Error("[taskmill]: worker failed to nack message: " + nackerr.Error())
		}
	} else {
		ackerr := w.ackMessage(ctx, message)
		if ackerr != nil {
			w.logger.With(
				"operation_id", operationID,
				"message_id", message.ID,
			).Error("[taskmill]: worker failed to ack message: " + ackerr.Error())
		}
	}

	return err
}

func (w *worker) ackMessage(ctx context.Context, message pgqueue.Message) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit

	err = w.queue.Ack(ctx, &tx, message.ID)
	if err != nil {
		return errx.Wrap(err)
	}

	err = tx.Commit()
	return errx.Wrap(err)
}

func (w *worker) nackMessage(ctx context.Context, message pgqueue.Message, reason map[string]any) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is no-op after commit

	err = w.queue.Nack(ctx, &tx, message.ID, reason)
	if err != nil {
		return errx.Wrap(err)
	}

	err = tx.Commit()
	return errx.Wrap(err)
}

func (w *worker) buildProcessChain() handleFunc {
	p := w.processMessage

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
	return func(ctx context.Context, m pgqueue.Message) error {
		logger := w.logger.Named("access_logger").WithContext(ctx)

		start := time.Now()

		err := next(ctx, m)

		logger = logger.With(
			"message", m,
			"duration", time.Since(start).Round(time.Microsecond),
		)

		if err != nil {
			logger.Errorx(err)
		} else {
			logger.Info("[taskmill]: task processed successfully")
		}

		return err
	}
}

func (w *worker) processWithAlerting(next handleFunc) handleFunc {
	return func(ctx context.Context, m pgqueue.Message) error {
		logger := w.logger.Named("alerting").WithContext(ctx)

		err := next(ctx, m)
		if err == nil {
			return nil
		}

		e := errx.AsErrorX(err)
		operation := fmt.Sprintf("async-task: %s", m.Payload["_operation_id"])
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
				logger.With("alert_send_error", senderr).Warn("[taskmill]: failed to send error alert")
			}
		}()

		return err
	}
}

func (w *worker) processWithMetaInjection(next handleFunc) handleFunc {
	return func(ctx context.Context, m pgqueue.Message) error {
		ctx = context.WithValue(ctx, meta.TraceID, getTraceID(ctx))
		ctx = context.WithValue(ctx, meta.ServiceName, w.serviceName)
		ctx = context.WithValue(ctx, meta.ServiceVersion, w.serviceVersion)
		return next(ctx, m)
	}
}

func (w *worker) processWithTimeout(next handleFunc) handleFunc {
	return func(ctx context.Context, m pgqueue.Message) error {
		timeout := w.processTimeout
		if m.ExpiresAt != nil {
			timeout = time.Until(*m.ExpiresAt)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return next(ctx, m)
	}
}

func (w *worker) processWithTracing(next handleFunc) handleFunc {
	return func(ctx context.Context, m pgqueue.Message) error {
		// Extract trace context from message payload
		ctx = extractTraceContext(ctx, m.Payload)

		// start a new span
		ctx, span := otel.Tracer("").Start(ctx, fmt.Sprintf("PROCESS %s", m.Payload["_operation_id"]),
			trace.WithAttributes(
				semconv.MessagingSystem("postgres"),
				semconv.MessagingOperationProcess,
				semconv.MessagingMessageID(fmt.Sprintf("%d", m.ID)),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)

		// call the next handler
		err := next(ctx, m)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		span.End()
		return err
	}
}

// extractTraceContext extracts OpenTelemetry trace context from the message payload.
// If the trace context is not found or invalid, returns the original context unchanged.
func extractTraceContext(ctx context.Context, payload map[string]any) context.Context {
	traceCtxRaw, ok := payload["_trace_ctx"]
	if !ok {
		return ctx
	}

	// The trace context was stored as map[string]string
	traceCtxMap, ok := traceCtxRaw.(map[string]any)
	if !ok {
		return ctx
	}

	// Convert map[string]any to map[string]string for the propagator
	carrier := make(map[string]string)
	for k, v := range traceCtxMap {
		if str, ok := v.(string); ok {
			carrier[k] = str
		}
	}

	if len(carrier) == 0 {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}

func (w *worker) processWithRecovery(next handleFunc) handleFunc {
	return func(ctx context.Context, m pgqueue.Message) (err error) {
		defer func() {
			if r := recover(); r != nil {
				w.logger.
					With(
						"recover", r,
						"message", m,
					).
					Error("[taskmill]: worker panicked at recovery wrapper")

				alertctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extendedContextTimeout)
				operationID := fmt.Sprintf("async-task: %s", m.Payload["_operation_id"])

				go func() {
					defer cancel()
					_ = alert.SendError(
						alertctx,
						"PANIC",
						"[taskmill]: worker panicked at recovery wrapper",
						operationID,
						map[string]string{"recover": fmt.Sprintf("%v", r)},
					)
				}()

				// Return error so message gets nacked instead of acked
				err = errx.New("[taskmill]: worker panicked at recovery wrapper", errx.WithDetails(errx.D{
					"panic": fmt.Sprintf("%v", r),
				}))
			}
		}()
		return next(ctx, m)
	}
}

func executeWithRecovery(ctx context.Context, task ucdef.AsyncTask[any], payload any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := make([]byte, 4096) // 4KB
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

			err = errx.New("[taskmill]: worker panicked at task execution", errx.WithDetails(errx.D{
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
