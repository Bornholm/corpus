package minio

import (
	"context"
	"fmt"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	testminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestWatch(t *testing.T) {
	ctx := context.Background()

	const (
		minioUsername = "miniousername"
		minioPassword = "miniopassword"
	)

	minioContainer, err := testminio.Run(
		ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		testminio.WithUsername(minioUsername),
		testminio.WithPassword(minioPassword),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(minioContainer); err != nil {
			t.Fatalf("failed to terminate container: %+v", errors.WithStack(err))
		}
	}()
	if err != nil {
		t.Fatalf("failed to start container: %+v", errors.WithStack(err))
	}

	endpoint, err := minioContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("could not retrieve connection string: %+v", errors.WithStack(err))
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioUsername, minioPassword, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create minio client: %+v", errors.WithStack(err))
	}

	const (
		bucketName = "corpus"
	)

	if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("failed to create minio bucket: %+v", errors.WithStack(err))
	}

	dsn := fmt.Sprintf("minio://%s:%s@%s?bucket=%s&secure=false", minioUsername, minioPassword, endpoint, bucketName)

	testsuite.TestWatch(t, dsn)
}
