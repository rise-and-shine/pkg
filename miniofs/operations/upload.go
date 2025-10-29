package operations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/code19m/errx"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/rise-and-shine/pkg/miniofs/processor"
	"github.com/rise-and-shine/pkg/miniofs/types"
)

type Uploader struct {
	Client    *minio.Client
	Bucket    string
	Processor *processor.ImageProcessor
}

func NewUploader(client *minio.Client, bucket string, proc *processor.ImageProcessor) *Uploader {
	return &Uploader{
		Client:    client,
		Bucket:    bucket,
		Processor: proc,
	}
}

func (u *Uploader) UploadImage(ctx context.Context,file io.Reader,fileName string) (string, error) {

	uniqueName := generateFileName(fileName)

	data, err := io.ReadAll(file)
	if err != nil {
		return "", errx.Wrap(err)
	}

	uploadInfo, err := u.Client.PutObject(
		ctx,
		u.Bucket,
		uniqueName,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: detectContentType(fileName)},
	)
	if err != nil {
		return "", errx.Wrap(err)
	}

	return uploadInfo.Location, nil
}

func (u *Uploader) UploadFile(ctx context.Context, file io.Reader, fileName string) (string, error) {
	fileName = generateFileName(fileName)
	_, err := u.Client.PutObject(ctx, u.Bucket, fileName, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func generateImageName(originalName string, size types.ImageSize) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	base := strings.TrimSuffix(originalName, ext)
	id := uuid.New().String()
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf(
		"%s_%s_%s_%s%s",
		base,
		id,
		date,
		size,
		ext,
	) // Example: "image_1234567890_2023-01-01_original.jpg"
}

func generateFileName(originalName string) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	base := strings.TrimSuffix(originalName, ext)
	id := uuid.New().String()
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s_%s_%s%s", base, id, date, ext) // Example: "image_1234567890_2023-01-01.jpg"
}

func detectContentType(fileName string) string {
    ext := strings.ToLower(filepath.Ext(fileName))
    switch ext {
    case ".jpg", ".jpeg":
        return "image/jpeg"
    case ".png":
        return "image/png"
    case ".gif":
        return "image/gif"
    case ".bmp":
        return "image/bmp"
    case ".webp":
        return "image/webp"
    case ".pdf":
        return "application/pdf"
    case ".txt":
        return "text/plain"
    case ".csv":
        return "text/csv"
    case ".json":
        return "application/json"
    default:
        return "application/octet-stream" // noma’lum fayllar uchun
    }
}
