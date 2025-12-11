// Package miniowr provides a MinIO implementation of the filestore.FileStore interface.
package miniowr

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/code19m/errx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rise-and-shine/pkg/filestore"
)

// Client implements the filestore.FileStore interface using MinIO.
type Client struct {
	client *minio.Client
	bucket string
}

// New creates a new MinIO filestore client.
func New(cfg Config) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &Client{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// Upload uploads a file to the specified path.
// Content type is detected from the file content.
func (c *Client) Upload(ctx context.Context, path string, reader io.Reader) (*filestore.FileInfo, error) {
	// Read content into buffer to detect content type and get size
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	contentType := http.DetectContentType(data)
	size := int64(len(data))

	info, err := c.client.PutObject(ctx, c.bucket, path, bytes.NewReader(data), size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &filestore.FileInfo{
		Path:         path,
		Size:         info.Size,
		ContentType:  contentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}, nil
}

// Get retrieves a file and its metadata from the specified path.
func (c *Client) Get(ctx context.Context, path string) (*filestore.File, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, errx.Wrap(err)
	}

	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, errx.Wrap(c.wrapMinioError(err))
	}

	return &filestore.File{
		Content: obj,
		Info: filestore.FileInfo{
			Path:         path,
			Size:         stat.Size,
			ContentType:  stat.ContentType,
			ETag:         stat.ETag,
			LastModified: stat.LastModified,
		},
	}, nil
}

// Delete removes a file at the specified path.
func (c *Client) Delete(ctx context.Context, path string) error {
	err := c.client.RemoveObject(ctx, c.bucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		return errx.Wrap(c.wrapMinioError(err))
	}
	return nil
}

// Exists checks if a file exists at the specified path.
func (c *Client) Exists(ctx context.Context, path string) (bool, error) {
	_, err := c.client.StatObject(ctx, c.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == codeNoSuchKey {
			return false, nil
		}
		return false, errx.Wrap(err)
	}
	return true, nil
}

// wrapMinioError converts MinIO errors to filestore error codes.
func (c *Client) wrapMinioError(err error) error {
	errResp := minio.ToErrorResponse(err)
	if errResp.Code == codeNoSuchKey {
		return errx.New("file not found", errx.WithCode(filestore.CodeFileNotFound))
	}
	return errx.Wrap(err)
}

const (
	codeNoSuchKey = "NoSuchKey"
)
