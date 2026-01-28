// Package middleware provides HTTP server middleware components.
package middleware

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.23.1"
	"go.opentelemetry.io/otel/trace"
)

// NewTracingMW creates a middleware that provides OpenTelemetry tracing for HTTP requests.
//
// This middleware starts a new span for each incoming request, propagates it through the
// request context, and adds relevant HTTP attributes to the span. It sets the span name
// based on the HTTP method and route path, and records errors if they occur during
// request processing.
func NewTracingMW() server.Middleware {
	return server.Middleware{
		Priority: 900,
		Handler: func(c *fiber.Ctx) error {
			defaultSpanName := fmt.Sprintf("%s %s", c.Method(), "/")

			opts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindServer),
			}

			ctx, span := otel.Tracer("http-server").Start(c.UserContext(), defaultSpanName, opts...)
			defer span.End()

			setContext(ctx, c)

			err := c.Next()

			routerPattern := c.Route().Path
			if routerPattern != "" && routerPattern != "/" {
				span.SetName(fmt.Sprintf("%s %s", c.Method(), routerPattern))
			}

			span.SetAttributes(
				semconv.HTTPMethodKey.String(c.Method()),
				semconv.HTTPRouteKey.String(routerPattern),
				semconv.HTTPURLKey.String(c.OriginalURL()),
				semconv.HTTPStatusCodeKey.Int(c.Response().StatusCode()),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		},
	}
}

func setContext(ctx context.Context, c *fiber.Ctx) {
	traceID := tracing.GetStartingTraceID(ctx)
	ctx = context.WithValue(ctx, meta.TraceID, traceID)
	ctx = context.WithValue(ctx, meta.IPAddress, c.IP())
	ctx = context.WithValue(ctx, meta.UserAgent, c.Get("user-agent"))
	ctx = context.WithValue(ctx, meta.AcceptLanguage, c.Get("accept-language"))

	c.Set("X-Trace-ID", traceID)
	c.SetUserContext(ctx)
}
