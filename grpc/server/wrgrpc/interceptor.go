package wrgrpc

import "google.golang.org/grpc"

// Interceptor represents a gRPC interceptor with priority-based ordering.
//
// Each interceptor has a priority value that determines its position in the
// interceptor chain. Higher priority values cause the interceptor to be executed
// earlier in the chain. This allows for controlling the execution order of
// interceptors regardless of the order they are added to the server. interceptor has a priority and a handler
type Interceptor struct {
	Priority int
	Handler  grpc.UnaryServerInterceptor
}

// ByOrder implements sort.Interface for []Interceptor based on
// the Priority field
type ByOrder []Interceptor

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	return a[i].Priority > a[j].Priority
}
