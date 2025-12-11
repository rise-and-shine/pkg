package filestore

// Error codes for filestore operations.
const (
	// CodeFileNotFound is returned when a file does not exist at the specified path.
	CodeFileNotFound = "FILE_NOT_FOUND"

	// CodeUnsupportedContentType is returned when the file's content type is not supported.
	CodeUnsupportedContentType = "UNSUPPORTED_CONTENT_TYPE"

	// CodeFileTooLarge is returned when the file exceeds the maximum allowed size.
	CodeFileTooLarge = "FILE_TOO_LARGE"
)
