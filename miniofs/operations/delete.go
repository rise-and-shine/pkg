package operations

import (
	"context"

	"github.com/minio/minio-go/v7"
)

type Deleter struct {
	Client *minio.Client
	Bucket string
}

func NewDeleter(client *minio.Client, bucket string) *Deleter {
	return &Deleter{
		Client: client,
		Bucket: bucket,
	}
}

// Delete a file from the storage
func (d *Deleter) Delete(ctx context.Context, fileName string) error {
	return d.Client.RemoveObject(ctx, d.Bucket, fileName, minio.RemoveObjectOptions{})
}
