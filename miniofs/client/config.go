package client

import (
	"time"

	"github.com/rise-and-shine/pkg/miniofs/types"
)

// Config holds the configuration parameters needed to connect to the MinIO storage backend.
//
// Fields:
//
//	Endpoint        - The MinIO server URL or IP address (e.g., "play.min.io:9000").
//	AccessKeyID     - The access key ID for authenticating with the MinIO server.
//	SecretAccessKey - The secret access key for authenticating with the MinIO server.
//	BucketName      - The name of the bucket to operate on.
//	UseSSL          - Whether to use SSL/TLS for the connection (true to enable HTTPS).
//	MaxFileSize     - The maximum allowed file size for uploads, in bytes.
//	Timeout         - The timeout duration for operations (e.g., 30 * time.Second).
//	AllowedMIME    - A list of allowed MIME types for file uploads.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool

	MaxFileSize int64 // in bytes
	Timeout     time.Duration
}

func (cfg *Config) Validate() error {
	// Required
	if cfg.Endpoint == "" {
		return types.ErrRequiredConfigEndpoint
	}
	if cfg.AccessKeyID == "" {
		return types.ErrRequiredConfigKeyID
	}
	if cfg.SecretAccessKey == "" {
		return types.ErrRequiredConfigSecret
	}

	// Default bucket name
	if cfg.BucketName == "" {
		cfg.BucketName = "images"
	}

	// Check file size to default value
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024 // 10 MB
	}

	// Check time to zero time
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return nil
}
