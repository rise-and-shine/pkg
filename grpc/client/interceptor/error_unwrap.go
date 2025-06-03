package interceptor

import (
	"context"
	"github.com/code19m/errx"
	"google.golang.org/grpc"
)

// NewErrorUnwrap creates a new interceptor that unwraps a gRPC errors
// If the error is not a grpc error, it returns the original error
//
// Returns:
//
//	-grpc.UnaryClientInterceptor: an interceptor func that can be used with gRPC clients
func NewErrorUnwrap() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		ok, e := errx.FromGRPCError(err)
		if !ok {
			return err
		}

		return e
	}
}
