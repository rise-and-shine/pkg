// Package tracing provides distributed tracing capabilities using OpenTelemetry.
// It allows for initializing a global tracer that can be used throughout an application
// to create and manage spans, and export them to a configured OTLP endpoint.
package tracing

import (
	"context"
	"net"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/spf13/cast"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.23.1"
	"go.opentelemetry.io/otel/trace/noop"
)

// InitGlobalTracer initializes a global OpenTelemetry tracer provider and OTLP exporter.
// It takes a Config (exporter host/port, sample rate, tags)
// It returns a shutdown function for the tracer provider and exporter (intended to be called with defer)
// and an error if initialization fails.
//
// If cfg.Disable is true, a no-op tracer is used.
// cfg.SampleRate controls the trace sampling fraction.
// cfg.Tags are added as resource attributes to spans.
func InitGlobalTracer(cfg Config) (func() error, error) {
	if cfg.Disable {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func() error { return nil }, nil
	}

	exporterAddr := net.JoinHostPort(cfg.ExporterHost, cast.ToString(cfg.ExporterPort))

	grpcTraceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(exporterAddr),
		otlptracegrpc.WithReconnectionPeriod(reconnectionPeriod),
	)

	exporter, err := otlptrace.New(
		context.Background(),
		grpcTraceClient,
	)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	processor := trace.NewBatchSpanProcessor(exporter)

	attrs := make([]attribute.KeyValue, 0, len(cfg.Tags))
	for k, v := range cfg.Tags {
		attrs = append(attrs, attribute.String(k, v))
	}
	attrs = append(attrs, semconv.ServiceNameKey.String(meta.ServiceName()))
	attrs = append(attrs, semconv.ServiceVersionKey.String(meta.ServiceVersion()))

	tp := trace.NewTracerProvider(
		trace.WithSampler(
			trace.ParentBased(trace.TraceIDRatioBased(cfg.SampleRate)),
		),
		trace.WithSpanProcessor(processor),
		trace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attrs...,
			),
		),
	)

	// set global propagator and tracer provider
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	otel.SetTracerProvider(tp)

	return shutdownFunc(tp), nil
}

func shutdownFunc(tp *trace.TracerProvider) func() error {
	return func() error {
		const shutdownTimeout = 5 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := tp.ForceFlush(ctx)
		if err != nil {
			return errx.Wrap(err)
		}

		err = tp.Shutdown(ctx)
		return errx.Wrap(err)
	}
}
