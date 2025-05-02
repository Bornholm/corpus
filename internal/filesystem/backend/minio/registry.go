package minio

import (
	"net/url"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendFactory("minio", FromDSN)
}

type Config struct {
	Endpoint string
	Bucket   string
	Options  minio.Options
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	conf := &Config{}

	configurations := []ConfigureFunc{
		configureBucket,
		configureCredentials,
		configureRegion,
		configureEndpoint,
	}

	for _, configure := range configurations {
		if err := configure(dsn, conf); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	client, err := minio.New(conf.Endpoint, &conf.Options)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backend := New(client, conf.Bucket, dsn.Path)

	return backend, nil
}

type ConfigureFunc func(dsn *url.URL, conf *Config) error

const (
	paramToken = "token"
)

func configureCredentials(dsn *url.URL, conf *Config) error {
	query := dsn.Query()

	if dsn.User != nil {
		id := dsn.User.Username()
		secret, _ := dsn.User.Password()
		token := query.Get(paramToken)

		dsn.User = nil
		query.Del(paramToken)

		conf.Options.Creds = credentials.NewStaticV4(id, secret, token)
	}

	dsn.RawQuery = query.Encode()

	return nil
}

const (
	paramBucket = "bucket"
)

func configureBucket(dsn *url.URL, conf *Config) error {
	query := dsn.Query()

	var bucket string
	if query.Has(paramBucket) {
		bucket = query.Get(paramBucket)
		query.Del(paramBucket)
		dsn.RawQuery = query.Encode()
	} else {
		bucket = "default"
	}

	conf.Bucket = bucket

	return nil
}

const (
	paramRegion = "region"
)

func configureRegion(dsn *url.URL, conf *Config) error {
	query := dsn.Query()

	var region string
	if query.Has(paramRegion) {
		region = query.Get(paramRegion)
		query.Del(paramRegion)
		dsn.RawQuery = query.Encode()
	} else {
		region = "us-east-1"
	}

	conf.Options.Region = region

	return nil
}

const (
	paramSecure = "secure"
)

func configureEndpoint(dsn *url.URL, conf *Config) error {
	query := dsn.Query()

	conf.Endpoint = dsn.Host

	if query.Get(paramSecure) == "true" {
		query.Del(paramSecure)
		dsn.RawQuery = query.Encode()
		conf.Options.Secure = true
	}

	return nil
}
