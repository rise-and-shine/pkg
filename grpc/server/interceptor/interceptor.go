// Package interceptor provides the interceptor for the server.
//
// # This package provides the interceptor for the server
//
// There are 3 interceptors:
// - NewErrorWrap: convert gRPC errors to the internal ErrorX format
// - NewMetaForward: forwards metadata from the context to the outgoing request
// - NewLogger: logs gRPC requests

package interceptor
