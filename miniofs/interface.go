package miniofs

import (
	"context"
	"io"
	"time"
)

// StorageService defines the contract for working with a minio storage system.
// It provides functionality for uploading, downloading, deleting, and checking file existence,
// as well as generating time-limited access URLs.
type StorageService interface {
	// UploadImage uploads an image file to the storage.
	// It automatically resizes the image into multiple predefined sizes
	// (e.g., original, medium, small) and returns a map of image sizes to their
	// respective stored file names or keys.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   file     - an io.Reader representing the image file to upload
	//   fileName - the base name to use for stored files
	//
	// Returns:
	//   map[types.ImageSize]string - a map of image sizes to their file names
	//   error                      - an error if the upload or resize fails
	UploadImage(ctx context.Context, file io.Reader, fileName string) (string, error)

	// UploadFile uploads a file to the storage.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   file     - an io.Reader representing the file to upload
	//   fileName - the base name to use for the stored file
	//
	// Returns:
	//   string - the name of the uploaded file
	//   error  - an error if the upload fails
	UploadFile(ctx context.Context, file io.Reader, fileName string) (string, error)

	// Download retrieves a file from the storage system.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   fileName - the name of the file to download
	//
	// Returns:
	//   io.Reader - a reader containing the file's data
	//   error     - an error if the file cannot be found or read
	Download(ctx context.Context, fileName string) (io.Reader, error)

	// Delete removes a file from the storage system.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   fileName - the name of the file to delete
	//
	// Returns:
	//   error - an error if the file cannot be deleted
	Delete(ctx context.Context, fileName string) error

	// Exists checks if a file exists in the storage system.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   fileName - the name of the file to check
	//
	// Returns:
	//   bool   - true if the file exists, false otherwise
	//   error  - an error if the check fails
	Exists(ctx context.Context, fileName string) (bool, error)

	// PresignedURL generates a presigned URL for a file in the storage system.
	//
	// Parameters:
	//   ctx      - the context for cancellation and deadlines
	//   bucket   - the name of the bucket containing the file
	//   object   - the name of the file
	//   expiry   - the duration for which the URL is valid
	//
	// Returns:
	//   string   - the presigned URL
	//   error    - an error if the URL generation fails
	PresignedURL(ctx context.Context, object string, expiry time.Duration) (string, error)
}
