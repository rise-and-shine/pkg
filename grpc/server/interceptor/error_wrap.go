package interceptor

import (
	"context"
	"github.com/code19m/errx"
	"google.golang.org/grpc"
)

// NewErrorWrap creates a new server interceptor that wraps a gRPC errors
//
// Returns:
//
//	-grpc.UnaryServerInterceptor: an interceptor func that can be used with gRPC servers
func NewErrorWrap(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		resp, err = handler(ctx, req)
		err = errx.ToGRPCError(err, errx.WithTracePrefix(serviceName))
		return resp, err
	}
}
