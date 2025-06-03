package interceptor

import (
	"context"
	"github.com/code19m/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewMetaForward creates a new interceptor that forwards the metadata from the context
// to the gRPC client
//
// Returns:
//
//	-grpc.UnaryClientInterceptor: an interceptor func that can be used with gRPC clients
func NewMetaForward() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctxData := meta.ExtractMetaFromContext(ctx)

		kv := make([]string, 0)

		for _, k := range []meta.ContextKey{
			meta.TraceID,
			meta.RequestUserID,
			meta.RequestUserType,
			meta.IPAddress,
			meta.AcceptLanguage,
			meta.XClientAppName,
			meta.XClientAppOS,
			meta.XClientAppVersion,
			meta.XTzOffset,
		} {
			if v, ok := ctxData[k]; ok && v != "" {
				kv = append(kv, string(k), v)
			}
		}

		ctx = metadata.AppendToOutgoingContext(ctx, kv...)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
