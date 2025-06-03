package wrgrpc

import (
	"github.com/spf13/cast"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"sort"
)

// grpcServer is a wrapper around the standard gRPC server that implements
// a consistent server interface with sensible defaults and additional features
type grpcServer struct {
	cfg        Config
	server     *grpc.Server
	lestenAddr string
}

// NewGRPC creates a new gRPC server with the specified configuration and interceptors
//
// Parameters:
//
//	cfg: the configuration for the gRPC server
//	interceptors: the interceptors to apply to the gRPC server
//
// Returns:
//
//	-grpcServer: a new gRPC server
func NewGRPC(cfg Config, interceptors []Interceptor) *grpcServer {
	unaryInterceptors := []grpc.UnaryServerInterceptor{}

	sort.Sort(ByOrder(interceptors))
	for _, interceptor := range interceptors {
		if interceptor.Handler != nil {
			unaryInterceptors = append(unaryInterceptors, interceptor.Handler)
		}
	}

	options := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(MaxReceiveMessageLength),
		grpc.MaxSendMsgSize(MaxSendMessageLength),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	if len(unaryInterceptors) > 0 {
		options = append(options, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}

	src := &grpcServer{
		cfg:        cfg,
		server:     grpc.NewServer(options...),
		lestenAddr: net.JoinHostPort(cfg.Host, cast.ToString(cfg.Port)),
	}

	return src
}

// Register registers grpc services implementations with the gRPC server
//
// This method allows for registering grpc services implementations with the gRPC server
// through a callback function
//
// Parameters:
//
//	registerFunc: a function that registers the grpc services implementations with the gRPC server
func (s *grpcServer) Register(registerFunc func(server *grpc.Server)) {
	registerFunc(s.server)
	if s.cfg.Reflection {
		reflection.Register(s.server)
	}
}

// startGRPC starts the gRPC server, binding it to the specified address
// this is a internal method used by the Start method
func (s *grpcServer) startGRPC() error {
	socket, err := net.Listen("tcp", s.lestenAddr)
	if err != nil {
		return err
	}
	return s.server.Serve(socket)
}

// Start starts the gRPC server on the configured address
//
// Returns:
//
//	error: an error if the server fails to start
func (s *grpcServer) Start() error {
	return s.startGRPC()
}

// Addr returns the address on which the gRPC server is listening
//
// Returns:
//
//	string: the address on which the gRPC server is listening
func (s *grpcServer) Addr() string {
	return s.lestenAddr
}

// Stop gracefully stops the gRPC server
//
// This method stops the gRPC server gracefully
func (s *grpcServer) Stop() {
	s.server.GracefulStop()
}
