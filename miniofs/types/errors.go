package types

import "errors"

var (
	ErrResizeFailed           = errors.New("image resize failed")
	ErrRequiredConfigEndpoint = errors.New("missing required config: Endpoint")
	ErrRequiredConfigKeyID    = errors.New("missing required config: AccessKeyID")
	ErrRequiredConfigSecret   = errors.New("missing required config: SecretAccessKey")

	ErrFileTooLarge    = errors.New("file size exceeds maximum allowed size")
	ErrInvalidMIMEType = errors.New("invalid file MIME type")
	ErrInvalidFileName = errors.New("invalid file name format")
)
