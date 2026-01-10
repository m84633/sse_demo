//go:build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	httpserver "sse_demo/internal/http"
	"sse_demo/internal/http/controller"
	"sse_demo/internal/queue/rabbitmq"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/sse"
	"sse_demo/internal/store/memory"
)

func TestPublishFlow(t *testing.T) {
	ginTestMode()

	ctx := context.Background()
	amqpURL, cleanup := setupRabbitMQContainer(t, ctx)
	defer cleanup()

	cfg := &config.Config{
		HTTPAddr:             ":0",
		SSEHeartbeat:         5 * time.Second,
		HistoryLimit:         0,
		RabbitMQURL:          amqpURL,
		RabbitExchange:       "notifications",
		RabbitQueue:          "notifications.sse",
		RabbitRoutingKey:     "notification.*",
		RabbitConsumerTag:    "sse-consumer",
		RabbitPublishPrefix:  "notification",
	}

	logger := zap.NewNop()
	repo := memory.New(logger)
	hub := sse.NewHub()
	svc := notify.NewService(repo, hub, logger)
	publisher := rabbitmq.NewPublisher(cfg, logger)
	consumer := rabbitmq.NewConsumer(cfg, svc, logger)

	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Start(consumeCtx)
	}()

	require.NoError(t, waitForConsumer(ctx, amqpURL, cfg.RabbitQueue, 5*time.Second))

	handler := controller.NewHandler(cfg, svc, hub, logger, publisher)
	router := httpserver.NewRouter(handler, logger)

	hubCtx, hubCancel := context.WithCancel(ctx)
	defer hubCancel()
	go hub.Run(hubCtx)

	server := httptest.NewServer(router)
	defer server.Close()

	sseResp, err := http.Get(server.URL + "/sse/room-1?limit=0")
	require.NoError(t, err)
	defer sseResp.Body.Close()
	require.Equal(t, http.StatusOK, sseResp.StatusCode)

	payload := map[string]string{
		"room":  "room-1",
		"type":  domain.NotificationTypeInfo,
		"title": "hello",
		"body":  "world",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	postResp, err := http.Post(server.URL+"/notifications/publish", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer postResp.Body.Close()
	require.Equal(t, http.StatusAccepted, postResp.StatusCode)

	data, err := readSSEData(sseResp.Body, 5*time.Second)
	require.NoError(t, err)
	require.Contains(t, data, "\"room\":\"room-1\"")

	cancel()
	select {
	case <-time.After(3 * time.Second):
		t.Fatalf("consumer did not stop")
	case <-errCh:
	}
}

func waitForConsumer(ctx context.Context, amqpURL, queue string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, err := amqp.Dial(amqpURL)
			if err != nil {
				continue
			}
			ch, err := conn.Channel()
			if err != nil {
				_ = conn.Close()
				continue
			}
			q, err := ch.QueueInspect(queue)
			_ = ch.Close()
			_ = conn.Close()
			if err != nil {
				continue
			}
			if q.Consumers > 0 {
				return nil
			}
		}
	}
}

func setupRabbitMQContainer(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()

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
