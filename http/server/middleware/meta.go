// Package middleware provides HTTP server middleware components.
package middleware

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/meta"
	"go.opentelemetry.io/otel/trace"
)

// NewMetaInjectMW creates a middleware that injects metadata into the request context.
// It also sets the X-Request-ID header to the trace ID.
// Uses global service info from meta.SetServiceInfo().
func NewMetaInjectMW() server.Middleware {
	return server.Middleware{
		Priority: 700,
		Handler: func(c *fiber.Ctx) error {
			ctx := c.UserContext()
			traceID := getTraceID(ctx)

			ctx = context.WithValue(ctx, meta.TraceID, traceID)
			ctx = context.WithValue(ctx, meta.ServiceName, meta.GetServiceName())
			ctx = context.WithValue(ctx, meta.ServiceVersion, meta.GetServiceVersion())

			c.SetUserContext(ctx)
			c.Set("X-Request-ID", traceID)
			return c.Next()
		},
	}
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
