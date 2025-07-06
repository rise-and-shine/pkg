// Package tracing provides distributed tracing capabilities using OpenTelemetry.
// It allows for initializing a global tracer that can be used throughout an application
// to create and manage spans, and export them to a configured OTLP endpoint.
package tracing

import (
	"context"
	"net"

	"github.com/code19m/errx"
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
// It takes a Config (exporter host/port, sample rate, tags), serviceName, and serviceVersion.
// It returns a shutdown function for the tracer provider and exporter (intended to be called with defer)
// and an error if initialization fails.
//
// If cfg.Disable is true, a no-op tracer is used.
// cfg.SampleRate controls the trace sampling fraction.
// cfg.Tags are added as resource attributes to spans.
func InitGlobalTracer(cfg Config, serviceName, serviceVersion string) (func() error, error) {
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
	attrs = append(attrs, semconv.ServiceNameKey.String(serviceName))
	attrs = append(attrs, semconv.ServiceVersionKey.String(serviceVersion))

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

	return func() error { return exporter.Shutdown(context.Background()) }, nil
}
