package ftp

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWatch(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Test skipped in CI environment because of FTP incompatibility with containerized environment")
		return
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image: "docker.io/garethflowers/ftp-server",
		Cmd:   []string{},
		Env: map[string]string{
			"FTP_PASS": "corpus",
			"FTP_USER": "corpus",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			// FTP passive mode, using host network
			hc.Resources = container.Resources{
				Ulimits: []*units.Ulimit{
					{
						Name: "nofile",
						Hard: 65536,
						Soft: 65536,
					},
				},
			}
			hc.NetworkMode = container.NetworkMode("host")
		},
		WaitingFor: wait.ForLog("chpasswd: password for 'corpus' changed"),
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

	port, err := container.MappedPort(ctx, "21")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dsn := fmt.Sprintf("ftp://corpus:corpus@127.0.0.1:%v?timeout=10s", port.Port())

	testsuite.TestWatch(t, dsn)
}
