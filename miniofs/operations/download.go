package operations

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type Downloader struct {
	Client *minio.Client
	Bucket string
}

func NewDownloader(client *minio.Client, bucket string) *Downloader {
	return &Downloader{
		Client: client,
		Bucket: bucket,
	}
}

func (d *Downloader) Download(ctx context.Context, fileName string) (io.Reader, error) {
	obj, err := d.Client.GetObject(ctx, d.Bucket, fileName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
