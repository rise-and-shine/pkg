package kafka

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/IBM/sarama"
	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/kafka/otelsarama"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/alert"
	"github.com/rise-and-shine/pkg/observability/tracing"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	alertTimeout = 3 * time.Second
)

// handlerWithRecovery is a wrapper around the handler to add recovery support.
func (c *Consumer) handlerWithRecovery(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stackTrace := make([]byte, 4096) // 4KB
				stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

				c.logger.
					Named("recovery").
					WithContext(ctx).
					With("stack_trace", string(stackTrace)).
					With("panic_message", r).
					Error("panic recovered in recovery handler")

				err = errx.New("panic recovered in recovery handler", errx.WithDetails(errx.D{
					"stack_trace":   string(stackTrace),
					"panic_message": r,
				}))
			}
		}()
		return next(ctx, msg)
	}
}

// handlerWithTracing is a wrapper around the handler to add tracing support.
func (c *Consumer) handlerWithTracing(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		// extract tracing info from headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, otelsarama.NewConsumerMessageCarrier(msg))

		// start a new span
		ctx, span := otel.Tracer("").Start(ctx, fmt.Sprintf("CONSUME %s", msg.Topic),
			trace.WithAttributes(
				semconv.MessagingSystem("kafka"),
				semconv.MessagingKafkaConsumerGroup(c.cfg.GroupID),
				semconv.MessagingOperationProcess,
				semconv.MessagingMessageID(string(msg.Key)),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		traceID := tracing.GetStartingTraceID(ctx)
		ctx = context.WithValue(ctx, meta.TraceID, traceID)

		// call the next handler
		err := next(ctx, msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	}
}

// handlerWithTimeout is a wrapper around the handler to add timeout support.
func (c *Consumer) handlerWithTimeout(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		ctx, cancel := context.WithTimeout(ctx, c.cfg.HandlerTimeout)
		defer cancel()

		return next(ctx, msg)
	}
}

// handlerWithAlerting is a wrapper around the handler to add alerting.
func (c *Consumer) handlerWithAlerting(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		logger := c.logger.Named("alerting").WithContext(ctx)

		err := next(ctx, msg)
		if err == nil {
			return nil
		}

		e := errx.AsErrorX(err)

		operation := fmt.Sprintf("consumer topic: %s", msg.Topic)
		details := make(map[string]string)
		details["trace_id"] = meta.Find(ctx, meta.TraceID)
		details["service_name"] = meta.ServiceName()
		details["service_version"] = meta.ServiceVersion()
		details["error_trace"] = e.Trace()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), alertTimeout)

		go func() {
			defer cancel()

			sendErr := alert.SendError(ctx, e.Code(), err.Error(), operation, details)
			if sendErr != nil {
				logger.With("alert_send_error", sendErr).Warn("failed to send error alert")
			}
		}()

		return err
	}
}

// handleWithLogging is a wrapper around the handler to add logging.
func (c *Consumer) handlerWithLogging(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		logger := c.logger.Named("access_logger").WithContext(ctx)

		start := time.Now()

		// extra recovery for catching panic in earler staps of the handler
		withRecovery := c.handlerWithRecovery(next)
		err := withRecovery(ctx, msg)

		headers := lo.SliceToMap(msg.Headers, func(h *sarama.RecordHeader) (string, string) {
			return string(h.Key), string(h.Value)
		})

		logger = logger.With(
			"service_name", meta.ServiceName(),
			"service_version", meta.ServiceVersion(),
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"duration", time.Since(start).Round(time.Microsecond),
			"headers", headers,
		)

		if err != nil {
			logger.Errorx(err)
		} else {
			logger.Info("message consumed successfully")
		}

		return err
	}
}

// handlerWithRecovery is a wrapper around the handler to add recovery.
// TODO: implement dlq.
func (c *Consumer) handlerWithErrorHandling(next HandleFunc) HandleFunc {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		return errx.Wrap(next(ctx, msg), errx.WithType(errx.T_Internal))
	}
}
