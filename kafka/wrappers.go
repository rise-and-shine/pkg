package kafka

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/avast/retry-go/v4"
	"github.com/code19m/errx"
	"github.com/code19m/pkg/kafka/otelsarama"
	"github.com/code19m/pkg/meta"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"runtime"
	"time"
)

// handlerWithRecovery is a wrapper around the handler to add recovery support
func (c *consumer) handlerWithRecovery(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stackTrace := make([]byte, 4096) // 4KB
				stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

				c.logger.
					Named("recovery").
					WithContext(ctx).
					With("stack_trace", string(stackTrace)).
					With("panic_values", fmt.Sprintf("%v", r)).
					Error("panic recovered in recovery handler")

				err = errx.New("panic recovered in recovery handler", errx.WithDetails(errx.D{
					"stack_trace":  string(stackTrace),
					"panic_values": fmt.Sprintf("%v", r),
				}))
			}
		}()
		return next(ctx, msg)
	}
}

// handlerWithTracing is a wrapper around the handler to add tracing support
func (c *consumer) handlerWithTracing(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		// extract tracing info from headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, otelsarama.NewConsumerMessageCarrier(msg))

		// start a new span
		ctx, span := otel.Tracer("").Start(ctx, fmt.Sprintf("kafka.%s.consume", msg.Topic),
			trace.WithAttributes(
				semconv.MessagingSystem("kafka"),
				semconv.MessagingKafkaConsumerGroup(c.cfg.GroupID),
				semconv.MessagingOperationProcess,
				semconv.MessagingMessageID(string(msg.Key)),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)

		defer func() {
			// end the span
			if err != nil {
				span.RecordError(err)
			}
			span.End()
		}()

		// call the next handler
		return next(ctx, msg)
	}
}

// handlerWithTimeout is a wrapper around the handler to add timeout support
func (c *consumer) handlerWithTimeout(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		ctx, cancel := context.WithTimeout(ctx, c.cfg.HandlerTimeout)
		defer cancel()
		return next(ctx, msg)
	}
}

// handlerWithMetaInjection is a wrapper around the handler to add meta injectionHandlerWithRecovery
func (c *consumer) handlerWithMetaInjection(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		// get from span context
		span := trace.SpanFromContext(ctx)
		traceId := span.SpanContext().TraceID().String()

		// if not found, generate a new one
		if traceId == "" {
			traceId = uuid.NewString()
		}

		metaData := map[meta.ContextKey]string{
			meta.TraceID:        traceId,
			meta.ServiceName:    c.cfg.ServiceName,
			meta.ServiceVersion: c.cfg.ServiceVersion,
		}

		// add meta to context for downstream handlers
		ctx = meta.InjectMetaToContext(ctx, metaData)

		return next(ctx, msg)
	}
}

// handlerWithAlerting is a wrapper around the handler to add alerting
func (c *consumer) handlerWithAlerting(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		logger := c.logger.Named("alerting").WithContext(ctx)

		err := next(ctx, msg)
		if err == nil {
			return nil
		}

		e := errx.AsErrorX(err)

		operation := fmt.Sprintf("consumer topic -> %s", msg.Topic)
		details := make(map[string]string)
		metaCtx := meta.ExtractMetaFromContext(ctx)
		for k, v := range metaCtx {
			details[string(k)] = v
		}
		details["error_type"] = e.Trace()

		sendErr := c.alertProvider.SendError(ctx, e.Code(), err.Error(), operation, details)
		if sendErr != nil {
			logger.With("send_error", sendErr).Warn("failed to send error alert")
		}

		return err
	}
}

// handleWithLogging is a wrapper around the handler to add logging
func (c *consumer) handlerWithLogging(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		logger := c.logger.Named("access_logger").WithContext(ctx)

		start := time.Now()

		// extra recovery for catching panic in earler staps of the handler
		withRecovery := c.handlerWithRecovery(next)
		err = withRecovery(ctx, msg)

		duration := time.Since(start)

		headers := lo.SliceToMap(msg.Headers, func(h *sarama.RecordHeader) (string, string) {
			return string(h.Key), string(h.Value)
		})

		logger = logger.With(
			"trace_id", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"duration", duration.String(),
			"headers", headers,
		)

		logMsg := "consumed incoming kafka message"
		if err != nil {
			logger = logger.With("error", getErrObject(err))
			logger.Error(logMsg)
		}
		logger.Info(logMsg)

		return err
	}
}

// handlerWithRecovery is a wrapper around the handler to add recovery
// TODO: Handle errors more gracefully. For example: Use dead letter queue
func (c *consumer) handlerWithErrorHandling(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		// make any error as internal
		return errx.Wrap(next(ctx, msg), errx.WithType(errx.T_Internal))
	}
}

// handlerWithRetry is a wrapper around the handler to add retry support with backoff and jitter
func (c *consumer) handlerWithRetry(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		if c.cfg.RetryDisabled {
			return next(ctx, msg)
		}

		logger := c.logger.Named("retry").WithContext(ctx)

		// configure retry with backoff and jitter
		err = retry.Do(
			func() error {
				return next(ctx, msg)
			},
			retry.Attempts(uint(c.cfg.RetryCount)),
			retry.Delay(c.cfg.RetryDelay),
			retry.MaxJitter(10),
			retry.LastErrorOnly(true), // only return the last error
			retry.OnRetry(func(n uint, err error) {
				logger.
					With("error", getErrObject(err)).
					With("attempt", n+1).
					With("max_attempts", c.cfg.RetryCount).
					With("retrying kafka message")
			}),
			retry.Context(ctx), // response to context cancellation
		)

		return err
	}
}

func getErrObject(err error) any {
	errx := errx.AsErrorX(err)
	return map[string]any{
		"code":    errx.Code(),
		"message": errx.Error(),
		"type":    errx.Type().String(),
		"trace":   errx.Trace(),
		"fields":  errx.Fields(),
		"details": errx.Details(),
	}
}
