//go:build integration

package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/sse"
)

func TestConsumerIntegration(t *testing.T) {
	ctx := context.Background()
	amqpURL, cleanup := setupRabbitMQContainer(t, ctx)
	defer cleanup()

	cfg := &config.Config{
		RabbitMQURL:         amqpURL,
		RabbitExchange:      "notifications",
		RabbitQueue:         "notifications.sse",
		RabbitRoutingKey:    "notification.*",
		RabbitConsumerTag:   "sse-consumer",
		RabbitPublishPrefix: "notification",
	}

	repo := &repoMock{}
	done := make(chan struct{})
	repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{
		ID:    1,
		Room:  "room-1",
		Type:  domain.NotificationTypeInfo,
		Title: "t",
		Body:  "b",
	}, nil).Run(func(args mock.Arguments) {
		select {
		case <-done:
		default:
			close(done)
		}
	}).Once()

	hub := sse.NewHub()
	svc := notify.NewService(repo, hub, zap.NewNop())
	consumer := NewConsumer(cfg, svc, zap.NewNop())

	consumeCtx, cancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Start(consumeCtx)
	}()

	require.NoError(t, waitForConsumer(ctx, amqpURL, cfg.RabbitQueue, 5*time.Second))

	publishNotification(t, amqpURL, cfg.RabbitExchange, "notification."+domain.NotificationTypeInfo, map[string]string{
		"room":  "room-1",
		"type":  domain.NotificationTypeInfo,
		"title": "t",
		"body":  "b",
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for consumer")
	}

	cancel()
	select {
	case <-time.After(3 * time.Second):
		t.Fatalf("consumer did not stop")
	case <-errCh:
	}

	repo.AssertExpectations(t)
}

// setupRabbitMQContainer is defined in testhelpers_integration.go

func publishNotification(t *testing.T, amqpURL, exchange, routingKey string, payload map[string]string) {
	t.Helper()

	conn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	err = ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
	require.NoError(t, err)

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	err = ch.Publish(exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	require.NoError(t, err)
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
