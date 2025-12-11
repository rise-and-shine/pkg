package miniowr

// Config defines the configuration options for MinIO client.
type Config struct {
	// Endpoint is the MinIO server endpoint (e.g., "localhost:9000").
	Endpoint string `yaml:"endpoint" validate:"required"`

	// AccessKey is the access key for authentication.
	AccessKey string `yaml:"access_key" validate:"required"`

	// SecretKey is the secret key for authentication.
	SecretKey string `yaml:"secret_key" validate:"required" mask:"true"`

	// Bucket is the default bucket name for file operations.
	Bucket string `yaml:"bucket" validate:"required"`

	// UseSSL enables HTTPS connection to MinIO server.
	UseSSL bool `yaml:"use_ssl" default:"false"`
}
