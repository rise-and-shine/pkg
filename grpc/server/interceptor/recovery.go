package interceptor

import (
	"context"
	"fmt"
	"github.com/code19m/errx"
	"github.com/code19m/pkg/grpc/server/wrgrpc"
	log "github.com/code19m/pkg/logger"
	"google.golang.org/grpc"
	"runtime"
)

// NewRecovery creates an interceptor that recovers from panics in gRPC handlers.
// It captures the panic, logs it with the stack trace, and converts it into a
// structured error response that can be safely returned to clients.
//
// The recovery interceptor has a priority of 900, placing it among the first
// interceptors to be executed in the chain.
func NewRecovery(logger log.Logger) wrgrpc.Interceptor {
	return wrgrpc.Interceptor{
		Priority: 900,
		Handler: func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (resp any, err error) {
			defer func() {
				if r := recover(); r != nil {
					stackTrace := make([]byte, 4096) // 4KB
					stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

					logger.
						Named("recovery_interceptor").
						WithContext(ctx).
						With("stack_trace", string(stackTrace)).
						With("panic_value", fmt.Sprintf("%v", r)).
						Error("panic recovered")

					err = errx.New("panic recovered at recovery_interceptor", errx.WithDetails(errx.D{
						"stack_trace": string(stackTrace),
						"panic_value": fmt.Sprintf("%v", r),
					}))
				}
			}()

			return handler(ctx, req)
		},
	}
}
