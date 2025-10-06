package client

import (
	"context"

	"github.com/code19m/errx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	Minio  *minio.Client
	Bucket string
}

// New creates and initializes a new MinIO Client instance based on the provided configuration.
//
// It validates the required configuration fields (Endpoint, AccessKeyID, SecretAccessKey),
// sets a default bucket name if none is provided, and attempts to establish a connection
// with the MinIO server. The function also checks if the specified bucket exists.
//
// Parameters:
//
//	cfg - configuration struct containing connection details and bucket name.
//
// Returns:
//
//	*Client - a pointer to the initialized Client containing the MinIO client and bucket name.
//	error   - an error wrapped with additional context if any step fails.
func New(cfg Config) (*Client, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, errx.New("invalid configuration", errx.WithDetails(errx.D{"error": err}))
	}

	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, errx.New("failed to create MinIO client", errx.WithDetails(errx.D{"error": err}))
	}

	ctx := context.Background()
	exists, err := cli.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, errx.New("failed to check bucket existence", errx.WithDetails(errx.D{"error": err}))
	}
	if !exists {
		err = cli.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, errx.New("failed to create bucket", errx.WithDetails(errx.D{"error": err}))
		}
	}

	return &Client{
		Minio:  cli,
		Bucket: cfg.BucketName,
	}, nil
}
