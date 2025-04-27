package sftp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWatch(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/atmoz/sftp:alpine@sha256:2464208ceb9e9562139d36a9045ec5eea4a0954c88a8bdd603e579d1a4ec0d03",
		Cmd:          []string{"corpus:corpus:::data"},
		ExposedPorts: []string{"22/tcp"},
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
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: filepath.Join(cwd, "testdata/sftp.d"),
					Target: "/etc/sftp.d",
				},
			}
		},
		WaitingFor: wait.ForLog("listening on 0.0.0.0 port 22"),
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

	port, err := container.MappedPort(ctx, "22")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dsn := fmt.Sprintf("sftp://corpus:corpus@127.0.0.1:%v/data?hostKey=insecure-ignore&timeout=10s", port.Port())

	testsuite.TestWatch(t, dsn)
}
