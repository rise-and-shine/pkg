package interceptor

import (
	"context"
	"github.com/code19m/pkg/grpc/server/wrgrpc"
	"github.com/code19m/pkg/meta"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewMetaInject creates a new gRPC server interceptor that injects metadata from incoming
// gRPC requests into the context. This ensures that important information like trace IDs,
// user information, and client application details are accessible in the request handlers.
//
// The interceptor handles several key tasks:
//   - Extracts or generates a trace ID for the request
//   - Extracts client metadata from incoming gRPC metadata
//   - Injects service name and version into the context
func NewMetaInject(serviceName, serviceVersion string) wrgrpc.Interceptor {
	return wrgrpc.Interceptor{
		Priority: 100,
		Handler: func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (resp any, err error) {

			// inject traceId into contex
			traceID := getTraceId(ctx)
			ctx = context.WithValue(ctx, meta.TraceID, traceID)

			// extract metadata and inject to context
			md, ok := metadata.FromIncomingContext(ctx)
			if ok {
				for _, k := range []meta.ContextKey{
					meta.RequestUserID,
					meta.RequestUserType,
					meta.IPAddress,
					meta.AcceptLanguage,
					meta.XClientAppName,
					meta.XClientAppOS,
					meta.XClientAppVersion,
					meta.XTzOffset,
				} {
					if value := md.Get(string(k)); len(value) > 0 {
						ctx = context.WithValue(ctx, k, value[0])
					}
				}
			}

			// inject service name and version into context
			ctx = context.WithValue(ctx, meta.ServiceName, serviceName)
			ctx = context.WithValue(ctx, meta.ServiceVersion, serviceVersion)

			return handler(ctx, req)
		},
	}
}

// getTraceId obtains a trace ID from the request context through one of three methods:
// 1. From an existing OpenTelemetry span in the context
// 2. From gRPC metadata headers
// 3. By generating a new UUID if no trace ID is found
//
// This ensures that every request has a trace ID for distributed tracing and logging.
//
// Parameters:
//   - ctx: The request context
//
// Returns:
//   - string: A valid trace ID string
func getTraceId(ctx context.Context) string {

	// get from span context
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	// if not found, try get from metadata
	if traceID == "" {
		traceID = getTraceIdFromMetadata(ctx)
	}

	// if not found, generate a new UUID
	if traceID == "" {
		traceID = uuid.New().String()
	}

	return traceID
}

// getTraceIdFromMetadata extracts a trace ID from gRPC metadata if present.
//
// Parameters:
//   - ctx: The request context containing gRPC metadata
//
// Returns:
//   - string: The trace ID from metadata or an empty string if not found
func getTraceIdFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if traceID := md.Get(string(meta.TraceID)); len(traceID) > 0 {
			return traceID[0]
		}
	}
	return ""
}
