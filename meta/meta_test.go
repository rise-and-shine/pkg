// Package meta_test contains tests for the meta package.
package meta_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rise-and-shine/pkg/meta"
)

// testMeta creates a metadata map for testing purposes.
// It handles the exhaustive linter directive in a single place.
func testMeta(pairs ...metaPair) map[meta.ContextKey]string {
	result := make(map[meta.ContextKey]string)
	for _, pair := range pairs {
		result[pair.key] = pair.value
	}
	return result
}

// metaPair represents a key-value pair for testing metadata.
type metaPair struct {
	key   meta.ContextKey
	value string
}

// mp is a convenience function to create a metaPair.
func mp(key meta.ContextKey, value string) metaPair {
	return metaPair{key: key, value: value}
}

func TestInjectMetaToContext(t *testing.T) {
	tests := []struct {
		name        string
		initialCtx  context.Context
		metaData    map[meta.ContextKey]string
		keyToVerify meta.ContextKey
		valueExpect string
		nilValue    bool
	}{
		{
			name:       "inject single value",
			initialCtx: context.Background(),
			metaData: testMeta(
				mp(meta.TraceID, "abc-123"),
			),
			keyToVerify: meta.TraceID,
			valueExpect: "abc-123",
		},
		{
			name:       "inject multiple values",
			initialCtx: context.Background(),
			metaData: testMeta(
				mp(meta.TraceID, "trace-123"),
				mp(meta.RequestUserID, "user-456"),
				mp(meta.IPAddress, "192.168.1.1"),
				mp(meta.ServiceName, "auth-service"),
				mp(meta.ServiceVersion, "v1.0.0"),
			),
			keyToVerify: meta.RequestUserID,
			valueExpect: "user-456",
		},
		{
			name:       "skip empty values",
			initialCtx: context.Background(),
			metaData: testMeta(
				mp(meta.TraceID, "trace-123"),
				mp(meta.RequestUserID, ""),
				mp(meta.IPAddress, "192.168.1.1"),
			),
			keyToVerify: meta.RequestUserID,
			nilValue:    true,
		},
		{
			name:       "overwrite existing value",
			initialCtx: context.WithValue(context.Background(), meta.TraceID, "old-trace-id"),
			metaData: testMeta(
				mp(meta.TraceID, "new-trace-id"),
			),
			keyToVerify: meta.TraceID,
			valueExpect: "new-trace-id",
		},
		{
			name:        "empty map",
			initialCtx:  context.Background(),
			metaData:    testMeta(),
			keyToVerify: meta.TraceID,
			nilValue:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			resultCtx := meta.InjectMetaToContext(tc.initialCtx, tc.metaData)

			// Assert
			if tc.nilValue {
				assert.Nil(t, resultCtx.Value(tc.keyToVerify))
			} else {
				assert.Equal(t, tc.valueExpect, resultCtx.Value(tc.keyToVerify))
			}

			// Check that initial context is not modified
			if val := tc.initialCtx.Value(tc.keyToVerify); val != nil {
				// Only check if the initial context had a value
				if originalVal, ok := val.(string); ok && originalVal != tc.valueExpect {
					// The initial context had a different value than the result context
					assert.NotEqual(t, originalVal, resultCtx.Value(tc.keyToVerify))
				}
			}
		})
	}
}

func TestExtractMetaFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctxSetup func() context.Context
		expected map[meta.ContextKey]string
	}{
		{
			name: "extract single value",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				return context.WithValue(ctx, meta.TraceID, "abc-123")
			},
			expected: testMeta(
				mp(meta.TraceID, "abc-123"),
			),
		},
		{
			name: "extract multiple values",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, "trace-123")
				ctx = context.WithValue(ctx, meta.RequestUserID, "user-456")
				ctx = context.WithValue(ctx, meta.IPAddress, "192.168.1.1")
				ctx = context.WithValue(ctx, meta.ServiceName, "auth-service")
				return ctx
			},
			expected: testMeta(
				mp(meta.TraceID, "trace-123"),
				mp(meta.RequestUserID, "user-456"),
				mp(meta.IPAddress, "192.168.1.1"),
				mp(meta.ServiceName, "auth-service"),
			),
		},
		{
			name: "ignore non-string values",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, 12345) // Not a string
				ctx = context.WithValue(ctx, meta.IPAddress, "192.168.1.1")
				return ctx
			},
			expected: testMeta(
				mp(meta.IPAddress, "192.168.1.1"),
			),
		},
		{
			name: "ignore empty string values",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, "trace-123")
				ctx = context.WithValue(ctx, meta.RequestUserID, "") // Empty string
				return ctx
			},
			expected: testMeta(
				mp(meta.TraceID, "trace-123"),
			),
		},
		{
			name:     "empty context",
			ctxSetup: context.Background,
			expected: testMeta(),
		},
		{
			name: "with custom context key not in predefined list",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				customKey := meta.ContextKey("custom_key")
				ctx = context.WithValue(ctx, customKey, "custom_value")
				ctx = context.WithValue(ctx, meta.TraceID, "trace-123")
				return ctx
			},
			expected: testMeta(
				mp(meta.TraceID, "trace-123"),
				// custom_key should not be extracted as it's not in the predefined list
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctx := tc.ctxSetup()

			// Act
			result := meta.ExtractMetaFromContext(ctx)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// This test checks that metadata can be injected into a context and then extracted correctly

	// Arrange
	originalCtx := context.Background()
	metadata := testMeta(
		mp(meta.TraceID, "trace-123"),
		mp(meta.RequestUserID, "user-456"),
		mp(meta.IPAddress, "192.168.1.1"),
		mp(meta.ServiceName, "auth-service"),
		mp(meta.ServiceVersion, "v1.0.0"),
		mp(meta.XClientAppName, "web-client"),
		mp(meta.XClientAppOS, "macos"),
		mp(meta.XClientAppVersion, "2.1.0"),
		mp(meta.RequestUserType, "admin"),
	)

	// Act - Inject metadata into context
	ctxWithMeta := meta.InjectMetaToContext(originalCtx, metadata)

	// Act - Extract metadata from context
	extractedMeta := meta.ExtractMetaFromContext(ctxWithMeta)

	// Assert
	assert.Equal(t, metadata, extractedMeta)
}

func TestAllContextKeys(t *testing.T) {
	// This test ensures all defined context keys can be properly injected and extracted

	// Define a map with all predefined context keys
	allKeys := testMeta(
		mp(meta.TraceID, "trace-xyz"),
		mp(meta.RequestUserID, "user-123"),
		mp(meta.RequestUserType, "customer"),
		mp(meta.RequestUserRole, "admin"),
		mp(meta.IPAddress, "10.0.0.1"),
		mp(meta.UserAgent, "Mozilla/5.0"),
		mp(meta.RemoteAddr, "10.0.0.2:8080"),
		mp(meta.Referer, "https://example.com"),
		mp(meta.ServiceName, "api-gateway"),
		mp(meta.ServiceVersion, "v2.3.4"),
		mp(meta.AcceptLanguage, "en-US"),
		mp(meta.XClientAppName, "mobile-app"),
		mp(meta.XClientAppOS, "ios"),
		mp(meta.XClientAppVersion, "3.0.1"),
		mp(meta.XTzOffset, "-0700"),
	)

	// Inject all keys into context
	ctx := meta.InjectMetaToContext(context.Background(), allKeys)

	// Extract all keys from context
	extracted := meta.ExtractMetaFromContext(ctx)

	// Verify all keys were properly injected and extracted
	assert.Len(t, extracted, len(allKeys))
	for k, v := range allKeys {
		extractedVal, ok := extracted[k]
		assert.True(t, ok, "Key %s not found in extracted metadata", k)
		assert.Equal(t, v, extractedVal, "Value mismatch for key %s", k)
	}
}
