package miniofs

import (
	"context"
	"io"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/miniofs/client"
	"github.com/rise-and-shine/pkg/miniofs/operations"
	"github.com/rise-and-shine/pkg/miniofs/processor"
)

// Service implements the StorageService interface
type Service struct {
	processor  *processor.ImageProcessor
	uploader   *operations.Uploader
	downloader *operations.Downloader
	deleter    *operations.Deleter
	exists     *operations.Exists
	presigner  *operations.PresignedURL
}

// NewService creates and initializes a new Service instance for interacting with the storage backend.
//
// It sets up all necessary components such as the MinIO client, image processor, uploader,
// downloader, deleter, existence checker, and presigner based on the provided configuration.
//
// Parameters:
//
//		cfg - configuration struct containing MinIO connection details and bucket name.
//		Required fields: Endpoint, AccessKeyID, SecretAccessKey
//	 Optional fields: BucketName (default: "images"), UseSSL (default: false)
//
// Returns:
//
//	*Service - a pointer to the fully initialized Service instance.
//	error    - an error wrapped with additional context if initialization fails.
func NewService(cfg client.Config) (*Service, error) {
	cl, err := client.New(cfg)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	proc := processor.New()

	return &Service{
		processor:  proc,
		uploader:   operations.NewUploader(cl.Minio, cfg.BucketName, proc),
		downloader: operations.NewDownloader(cl.Minio, cfg.BucketName),
		deleter:    operations.NewDeleter(cl.Minio, cfg.BucketName),
		exists:     operations.NewExists(cl.Minio, cfg.BucketName),
		presigner:  operations.NewPresignedURL(cl.Minio, cfg.BucketName, proc),
	}, nil
}

func (s *Service) UploadImage(ctx context.Context,file io.Reader,fileName string) (string, error) {
	url, err := s.uploader.UploadImage(ctx, file, fileName)
	if err != nil {
		return "", errx.Wrap(err)
	}
	return url, nil
}

func (s *Service) UploadFile(ctx context.Context, file io.Reader, fileName string) (string, error) {
	url, err := s.uploader.UploadFile(ctx, file, fileName)
	if err != nil {
		return "", errx.Wrap(err)
	}
	return url, nil
}

func (s *Service) Download(ctx context.Context, fileName string) (io.Reader, error) {
	info, err := s.downloader.Download(ctx, fileName)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	return info, nil
}

func (s *Service) Delete(ctx context.Context, fileName string) error {
	err := s.deleter.Delete(ctx, fileName)
	return errx.Wrap(err)
}

func (s *Service) Exists(ctx context.Context, fileName string) (bool, error) {
	exists, err := s.exists.Exists(ctx, fileName)
	if err != nil {
		return false, errx.Wrap(err)
	}
	return exists, nil
}

func (s *Service) PresignedURL(ctx context.Context, object string, expiry time.Duration) (string, error) {
	url, err := s.presigner.PresignedURL(ctx, object, expiry)
	if err != nil {
		return "", errx.Wrap(err)
	}
	return url, nil
}
