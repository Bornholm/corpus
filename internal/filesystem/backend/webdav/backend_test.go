package webdav

import (
	"context"
	"fmt"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWatch(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Name:         "corpus_watch_test_webdav",
		Image:        "docker.io/bytemark/webdav:2.4@sha256:bcabbc024c511b9c63ed3345f88573e31d84c952ee493c9acb3fe345f4f80f57",
		ExposedPorts: []string{"80/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Resources = container.Resources{
				Ulimits: []*units.Ulimit{
					{
						Name: "nofile",
						Hard: 65536,
						Soft: 65536,
					},
				},
			}
		},
		Env: map[string]string{
			"USERNAME":  "corpus",
			"PASSWORD":  "corpus",
			"AUTH_TYPE": "Basic",
		},
		WaitingFor: wait.ForLog("httpd -D FOREGROUND"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("%+v", errors.WithStack(err))
		}
	}()

	port, err := container.MappedPort(ctx, "80")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dsn := fmt.Sprintf("webdav://corpus:corpus@127.0.0.1:%v", port.Port())

	testsuite.TestWatch(t, dsn)
}
