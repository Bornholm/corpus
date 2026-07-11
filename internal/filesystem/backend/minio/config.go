package minio

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("minio", &MinIOConfig{}, FromConfig)
}

// MinIOConfig holds the configuration for a MinIO/S3-compatible backend.
// Named MinIOConfig to avoid collision with the internal Config type in this package.
type MinIOConfig struct {
	Endpoint  string `json:"endpoint"             jsonschema:"required,description=MinIO/S3 endpoint (host:port)"`
	AccessKey string `json:"accessKey"            jsonschema:"required,description=Access key ID"`
	SecretKey string `json:"secretKey"            jsonschema:"required,description=Secret access key"`
	Token     string `json:"token,omitempty"      jsonschema:"description=Optional session token (for temporary credentials)"`
	Bucket    string `json:"bucket,omitempty"     jsonschema:"default=default,description=Bucket name"`
	Region    string `json:"region,omitempty"     jsonschema:"default=us-east-1,description=AWS region"`
	BasePath  string `json:"basePath,omitempty"   jsonschema:"description=Base path within the bucket"`
	Secure    bool   `json:"secure,omitempty"     jsonschema:"description=Use HTTPS"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg MinIOConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse minio backend config")
	}
	if cfg.Endpoint == "" {
		return nil, errors.New("minio backend config: endpoint is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("minio backend config: accessKey and secretKey are required")
	}

	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "default"
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, cfg.Token),
		Region: region,
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not create minio client")
	}

	return New(client, bucket, cfg.BasePath), nil
}
