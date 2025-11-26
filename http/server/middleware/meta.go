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
//
// This middleware collects information from the request such as trace ID, IP address,
// user agent, and other HTTP headers, and injects them into the request context using
// the meta package. It also sets service information and prepares keys for user
// identification that will be populated by authentication middlewares.
func NewMetaInjectMW(serviceName, serviceVersion string) server.Middleware {
	return server.Middleware{
		Priority: 700,
		Handler: func(c *fiber.Ctx) error {
			traceID := getTraceID(c.UserContext())

			metaData := map[meta.ContextKey]string{
				meta.TraceID:        traceID,
				meta.ServiceName:    serviceName,
				meta.ServiceVersion: serviceVersion,
				meta.AcceptLanguage: c.Get("accept-language"),
				meta.XTzOffset:      c.Get("x-tz-offset"),
			}

			ctx := c.UserContext()
			for k, v := range metaData {
				ctx = context.WithValue(ctx, k, v)
			}

			c.SetUserContext(ctx)

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
