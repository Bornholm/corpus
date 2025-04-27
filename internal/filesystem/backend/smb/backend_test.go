package smb

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
		Image: "docker.io/dperson/samba@sha256:66088b78a19810dd1457a8f39340e95e663c728083efa5fe7dc0d40b2478e869",
		Cmd: []string{
			"-p",
			"-u", "corpus;corpus",
			"-s", "private;/data/private;yes;no;no;corpus;corpus;corpus",
			"-w", "WORKGROUP",
		},
		ExposedPorts: []string{"139/tcp", "445/tcp"},
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
		WaitingFor: wait.ForLog("daemon 'smbd' finished starting up and ready to serve connections"),
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

	port, err := container.MappedPort(ctx, "139")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dsn := fmt.Sprintf("smb://corpus:corpus@127.0.0.1:%v?share=private", port.Port())

	testsuite.TestWatch(t, dsn)
}
