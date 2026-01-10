//go:build integration

package rabbitmq

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRabbitMQContainer(t require.TestingT, ctx context.Context) (string, func()) {
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-alpine",
		ExposedPorts: []string{"5672/tcp"},
		WaitingFor:   wait.ForListeningPort("5672/tcp").WithStartupTimeout(2 * time.Minute),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)

	amqpURL := "amqp://guest:guest@" + host + ":" + port.Port() + "/"

	cleanup := func() {
		_ = container.Terminate(ctx)
	}
	return amqpURL, cleanup
}
