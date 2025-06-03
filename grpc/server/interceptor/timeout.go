package interceptor

import (
	"context"
	"github.com/code19m/pkg/grpc/server/wrgrpc"
	"google.golang.org/grpc"
	"time"
)

// NewTimeout creates an interceptor that enforces a maximum execution time for gRPC handlers.
// If the handler does not complete within the specified duration, the context will be canceled,
// which should propagate to any ongoing operations within the handler or downstream services.
//
// The timeout interceptor has a priority of 800, making it one of the first interceptors to be
// executed in the chain.
func NewTimeout(duration time.Duration) wrgrpc.Interceptor {
	return wrgrpc.Interceptor{
		Priority: 800,
		Handler: func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, duration)
			defer cancel()
			return handler(ctx, req)
		},
	}
}
