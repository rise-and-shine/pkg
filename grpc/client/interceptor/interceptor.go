// Package interceptor provides the interceptor for the client.
//
// # This package provides the interceptor for the client
//
// There are 3 interceptors:
// - NewErrorUnwrap: convert gRPC errors to the internal ErrorX format
// - NewMetaForward: forwards metadata from the context to the outgoing request
package interceptor
