package operations

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rise-and-shine/pkg/miniofs/processor"
)

type PresignedURL struct {
	Client    *minio.Client
	Bucket    string
	Processor *processor.ImageProcessor
}

func NewPresignedURL(client *minio.Client, bucket string, processor *processor.ImageProcessor) *PresignedURL {
	return &PresignedURL{
		Client:    client,
		Bucket:    bucket,
		Processor: processor,
	}
}

func (p *PresignedURL) PresignedURL(ctx context.Context, fileName string, expiry time.Duration) (string, error) {
	u, err := p.Client.PresignedPutObject(ctx, p.Bucket, fileName, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
