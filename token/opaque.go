package token

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewOpaqueToken generates a simple and secure opaque token.
func NewOpaqueToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		// this should never happen
		panic(fmt.Errorf("failed to generate opaque token: %w", err))
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}
