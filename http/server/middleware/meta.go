// Package middleware provides HTTP server middleware components.
package middleware

import (
	"context"

	"github.com/code19m/pkg/http/server"
	"github.com/code19m/pkg/meta"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
				meta.TraceID:           traceID,
				meta.IPAddress:         c.IP(),
				meta.UserAgent:         c.Get(fiber.HeaderUserAgent),
				meta.RemoteAddr:        c.Context().RemoteAddr().String(),
				meta.Referer:           c.Get(fiber.HeaderReferer),
				meta.ServiceName:       serviceName,
				meta.ServiceVersion:    serviceVersion,
				meta.AcceptLanguage:    c.Get(string(meta.AcceptLanguage)),
				meta.XClientAppName:    c.Get(string(meta.XClientAppName)),
				meta.XClientAppOS:      c.Get(string(meta.XClientAppOS)),
				meta.XClientAppVersion: c.Get(string(meta.XClientAppVersion)),
				meta.XTzOffset:         c.Get(string(meta.XTzOffset)),

				// missing keys. Those are will be set by authentication middlewares
				meta.RequestUserID:   "",
				meta.RequestUserType: "",
				meta.RequestUserRole: "",
			}

			ctx := meta.InjectMetaToContext(c.UserContext(), metaData)
			c.SetUserContext(ctx)

			return c.Next()
		},
	}
}

// getTraceID extracts the trace ID from the current span in the context.
// If no trace ID is available, it generates a new UUID to use as a trace ID.
func getTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	if traceID == "" {
		traceID = uuid.NewString()
	}

	return traceID
}
