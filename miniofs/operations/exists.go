package operations

import (
	"context"
	"github.com/minio/minio-go/v7"
)

type Exists struct {
	Client *minio.Client
	Bucket string
}

// NewExists returns a new instance of Exists checker
func NewExists(client *minio.Client, bucket string) *Exists {
	return &Exists{
		Client: client,
		Bucket: bucket,
	}
}

func (e *Exists) Exists(ctx context.Context, fileName string) (bool, error) {
	_, err := e.Client.StatObject(ctx, e.Bucket, fileName, minio.StatObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" || resp.Code == "NotFound" {
			return false, nil // Fayl topilmadi
		}
		return false, err // Boshqa xatolik
	}
	return true, nil // Fayl mavjud
}
