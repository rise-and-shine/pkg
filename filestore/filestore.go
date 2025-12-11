// Package filestore provides an abstraction for file storage operations.
//
// It defines a FileStore interface that can be implemented by various
// storage backends (e.g., MinIO, S3, local filesystem). The interface
// is designed to be injected into different components across project layers.
package filestore

import (
	"context"
	"io"
	"time"
)

// FileStore defines the interface for file storage operations.
// Implementations must be safe for concurrent use.
type FileStore interface {
	// Upload uploads a file to the specified path.
	// The implementation detects size and content type from the reader.
	// Returns the file info after successful upload.
	Upload(ctx context.Context, path string, reader io.Reader) (*FileInfo, error)

	// Get retrieves a file and its metadata from the specified path.
	// The caller is responsible for closing File.Content.
	Get(ctx context.Context, path string) (*File, error)

	// Delete removes a file at the specified path.
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists at the specified path.
	Exists(ctx context.Context, path string) (bool, error)
}

// File represents a stored file with its content and metadata.
type File struct {
	Content io.ReadCloser
	Info    FileInfo
}

// FileInfo contains metadata about a stored file.
type FileInfo struct {
	Path         string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}
