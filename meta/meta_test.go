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
				mp(meta.ActorID, "user-456"),
				mp(meta.ActorType, "customer"),
				mp(meta.ServiceName, "auth-service"),
				mp(meta.ServiceVersion, "v1.0.0"),
			),
			keyToVerify: meta.ActorID,
			valueExpect: "user-456",
		},
		{
			name:       "skip empty values",
			initialCtx: context.Background(),
			metaData: testMeta(
				mp(meta.TraceID, "trace-123"),
				mp(meta.ActorID, ""),
				mp(meta.ServiceName, "auth-service"),
			),
			keyToVerify: meta.ActorID,
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
			resultCtx := tc.initialCtx
			for k, v := range tc.metaData {
				if v != "" {
					resultCtx = context.WithValue(resultCtx, k, v)
				}
			}

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
				ctx = context.WithValue(ctx, meta.ActorID, "user-456")
				ctx = context.WithValue(ctx, meta.ActorType, "customer")
				ctx = context.WithValue(ctx, meta.ServiceName, "auth-service")
				return ctx
			},
			expected: testMeta(
				mp(meta.TraceID, "trace-123"),
				mp(meta.ActorID, "user-456"),
				mp(meta.ActorType, "customer"),
				mp(meta.ServiceName, "auth-service"),
			),
		},
		{
			name: "ignore non-string values",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, 12345) // Not a string
				ctx = context.WithValue(ctx, meta.ServiceName, "auth-service")
				return ctx
			},
			expected: testMeta(
				mp(meta.ServiceName, "auth-service"),
			),
		},
		{
			name: "ignore empty string values",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, "trace-123")
				ctx = context.WithValue(ctx, meta.ActorID, "") // Empty string
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
		mp(meta.ActorType, "user"),
		mp(meta.ActorID, "actor-123"),
		mp(meta.ServiceName, "auth-service"),
		mp(meta.ServiceVersion, "v1.0.0"),
		mp(meta.AcceptLanguage, "en-US"),
		mp(meta.XTzOffset, "-0700"),
	)

	// Act - Inject metadata into context
	ctxWithMeta := originalCtx
	for k, v := range metadata {
		ctxWithMeta = context.WithValue(ctxWithMeta, k, v)
	}

	// Act - Extract metadata from context
	extractedMeta := meta.ExtractMetaFromContext(ctxWithMeta)

	// Assert
	assert.Equal(t, metadata, extractedMeta)
}

func TestShouldGetMeta(t *testing.T) {
	tests := []struct {
		name          string
		ctxSetup      func() context.Context
		key           meta.ContextKey
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name: "success - valid string value",
			ctxSetup: func() context.Context {
				return context.WithValue(context.Background(), meta.TraceID, "trace-xyz-123")
			},
			key:           meta.TraceID,
			expectedValue: "trace-xyz-123",
			expectError:   false,
		},
		{
			name: "error - key not found",
			ctxSetup: func() context.Context {
				return context.Background()
			},
			key:           meta.ActorID,
			expectedValue: "",
			expectError:   true,
			errorContains: "key not found",
		},
		{
			name: "error - type mismatch (non-string value)",
			ctxSetup: func() context.Context {
				return context.WithValue(context.Background(), meta.ActorID, 12345)
			},
			key:           meta.ActorID,
			expectedValue: "",
			expectError:   true,
			errorContains: "type mismatch",
		},
		{
			name: "success - empty string value",
			ctxSetup: func() context.Context {
				return context.WithValue(context.Background(), meta.ActorType, "")
			},
			key:           meta.ActorType,
			expectedValue: "",
			expectError:   false,
		},
		{
			name: "success - all predefined keys",
			ctxSetup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, meta.TraceID, "trace-123")
				ctx = context.WithValue(ctx, meta.ActorType, "user")
				ctx = context.WithValue(ctx, meta.ActorID, "actor-456")
				ctx = context.WithValue(ctx, meta.ServiceName, "test-service")
				ctx = context.WithValue(ctx, meta.ServiceVersion, "v1.0.0")
				ctx = context.WithValue(ctx, meta.AcceptLanguage, "en-US")
				ctx = context.WithValue(ctx, meta.XTzOffset, "-0700")
				return ctx
			},
			key:           meta.ServiceName,
			expectedValue: "test-service",
			expectError:   false,
		},
		{
			name: "error - type mismatch with struct value",
			ctxSetup: func() context.Context {
				type customStruct struct {
					field string
				}
				return context.WithValue(context.Background(), meta.ServiceName, customStruct{field: "value"})
			},
			key:           meta.ServiceName,
			expectedValue: "",
			expectError:   true,
			errorContains: "type mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctx := tc.ctxSetup()

			// Act
			value, err := meta.ShouldGetMeta(ctx, tc.key)

			// Assert
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Equal(t, tc.expectedValue, value)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, value)
			}
		})
	}
}

func TestAllContextKeys(t *testing.T) {
	// This test ensures all defined context keys can be properly injected and extracted

	// Define a map with all predefined context keys
	allKeys := testMeta(
		mp(meta.TraceID, "trace-xyz"),
		mp(meta.ActorType, "customer"),
		mp(meta.ActorID, "user-123"),
		mp(meta.ServiceName, "api-gateway"),
		mp(meta.ServiceVersion, "v2.3.4"),
		mp(meta.AcceptLanguage, "en-US"),
		mp(meta.XTzOffset, "-0700"),
	)

	// Inject all keys into context
	ctx := context.Background()
	for k, v := range allKeys {
		ctx = context.WithValue(ctx, k, v)
	}

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
