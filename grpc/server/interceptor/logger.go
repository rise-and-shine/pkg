package interceptor

import (
	"context"
	"fmt"
	"github.com/code19m/errx"
	"github.com/code19m/pkg/grpc/server/wrgrpc"
	log "github.com/code19m/pkg/logger"
	"google.golang.org/grpc"
	"runtime"
	"time"
)

// NewLogger creates a new gRPC server interceptor for logging request and response information.
// This interceptor logs the details of each gRPC request, including method name, duration,
// and error information if applicable.
//
// The logger adapts its logging level based on the error type:
//   - For internal errors: ERROR level
//   - For other errors (validation, not found, etc.): WARN level
//   - For successful requests: INFO level
func NewLogger(logger log.Logger) wrgrpc.Interceptor {
	return wrgrpc.Interceptor{
		Priority: 600,
		Handler: func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (resp any, err error) {
			logger := logger.Named("logger_server_interceptor").WithContext(ctx)

			start := time.Now()

			resp, err = handleWithRecovery(ctx, req, handler)

			duration := time.Since(start)

			logger = logger.With(
				"method", info.FullMethod,
				"duration", duration,
			)

			if err != nil {
				logger = logger.With("error", getErrorObject(err))
			}

			msg := "processed incoming gRPC unary request"
			if err != nil {
				errType := errx.GetType(err)
				if errType == errx.T_Internal {
					logger.Error(msg)
				} else {
					logger.Warn(msg)
				}
			} else {
				logger.Info(msg)
			}

			return resp, err
		},
	}
}

// getErrObject converts an error to a structured map containing detailed error information.
// This is particularly useful for structured logging of errors.
//
// Parameters:
//   - err: The error to convert to a structured map
//
// Returns:
//   - any: A map containing error details from the ErrorX interface
func getErrorObject(err error) any {
	errX := errx.AsErrorX(err)

	return map[string]any{
		"code":    errX.Code(),
		"message": errX.Error(),
		"type":    errX.Type().String(),
		"trace":   errX.Trace(),
		"fields":  errX.Fields(),
		"details": errX.Details(),
	}
}

// handleWithRecovery wraps a gRPC handler with panic recovery functionality.
// If a panic occurs during the execution of the handler, it is captured and converted
// to a proper error.
//
// Parameters:
//   - ctx: The request context
//   - req: The gRPC request object
//   - handler: The gRPC handler function to protect
//
// Returns:
//   - any: The response from the handler or nil if a panic occurred
//   - error: Any error from the handler or a new error if a panic occurred
func handleWithRecovery(ctx context.Context, req any, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := make([]byte, 4096) // 4KB
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

			err = errx.New("panic recovered at logger_interceptor", errx.WithDetails(errx.D{
				"stack_trace": string(stackTrace),
				"panic_value": fmt.Sprintf("%v", r),
			}))
		}
	}()

	return handler(ctx, req)
}
