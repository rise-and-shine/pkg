// Package meta provides functionality for managing request metadata through context.
package meta

import "context"

// ContextKey is a type for keys used in context values for metadata.
type ContextKey string

const (
	// TraceID represents a unique identifier for tracing requests across services.
	TraceID ContextKey = "trace_id"

	// RequestUserID identifies the user making the request.
	RequestUserID ContextKey = "request_user_id"

	// RequestUserType indicates the type of the user making the request.
	RequestUserType ContextKey = "request_user_type"

	// RequestUserRole indicates the current role of the user making the request.
	RequestUserRole ContextKey = "request_user_role"

	// IPAddress contains the client's IP address.
	IPAddress ContextKey = "ip_address"

	// UserAgent contains the user agent string from the request.
	UserAgent ContextKey = "user_agent"

	// RemoteAddr contains the network address that sent the request.
	RemoteAddr ContextKey = "remote_addr"

	// Referer contains the address of the previous web page from which a link was followed.
	Referer ContextKey = "referer"

	// ServiceName identifies the name of current running service.
	ServiceName ContextKey = "service_name"

	// ServiceVersion indicates the version of the service.
	ServiceVersion ContextKey = "service_version"

	// AcceptLanguage indicates the natural language and locale that the client prefers.
	AcceptLanguage ContextKey = "accept-language"

	// XClientAppName identifies the client application name.
	XClientAppName ContextKey = "x-client-app-name"

	// XClientAppOS identifies the operating system of the client.
	XClientAppOS ContextKey = "x-client-app-os"

	// XClientAppVersion indicates the version of the client application.
	XClientAppVersion ContextKey = "x-client-app-version"

	// XTzOffset contains the timezone offset from the client.
	XTzOffset ContextKey = "x-tz-offset"
)

// InjectMetaToContext adds metadata from the provided map to the context.
// It only adds values that are not empty strings and returns a new context
// with the added values.
func InjectMetaToContext(ctx context.Context, data map[ContextKey]string) context.Context {
	for k, v := range data {
		if v != "" {
			ctx = context.WithValue(ctx, k, v) //nolint:fatcontext // allow due to finite number of keys
		}
	}
	return ctx
}

// ExtractMetaFromContext extracts all metadata from the provided context.
// It retrieves values for all predefined context keys and returns them in a map.
// Only non-empty string values are included in the returned map.
func ExtractMetaFromContext(ctx context.Context) map[ContextKey]string {
	data := make(map[ContextKey]string)
	for _, k := range []ContextKey{
		TraceID,
		RequestUserID,
		RequestUserType,
		RequestUserRole,
		IPAddress,
		UserAgent,
		RemoteAddr,
		Referer,
		ServiceName,
		ServiceVersion,
		AcceptLanguage,
		XClientAppName,
		XClientAppOS,
		XClientAppVersion,
		XTzOffset,
	} {
		if v, ok := ctx.Value(k).(string); ok && v != "" {
			data[k] = v
		}
	}
	return data
}
