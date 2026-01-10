//go:build integration

package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
)

func TestPublisherIntegration(t *testing.T) {
	ctx := context.Background()
	amqpURL, cleanup := setupRabbitMQContainer(t, ctx)
	defer cleanup()

	cfg := &config.Config{
		RabbitMQURL:       amqpURL,
		RabbitExchange:    "notifications",
		RabbitQueue:       "notifications.sse",
		RabbitRoutingKey:  "notification.*",
		RabbitConsumerTag: "sse-consumer",
	}

	publisher := NewPublisher(cfg, zap.NewNop())

	conn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	err = ch.ExchangeDeclare(cfg.RabbitExchange, "topic", true, false, false, false, nil)
	require.NoError(t, err)
	_, err = ch.QueueDeclare(cfg.RabbitQueue, true, false, false, false, nil)
	require.NoError(t, err)
	err = ch.QueueBind(cfg.RabbitQueue, cfg.RabbitRoutingKey, cfg.RabbitExchange, false, nil)
	require.NoError(t, err)

	deliveries, err := ch.Consume(cfg.RabbitQueue, "publisher-test", true, false, false, false, nil)
	require.NoError(t, err)

	payload := map[string]string{
		"room":  "room-1",
		"type":  domain.NotificationTypeInfo,
		"title": "title",
		"body":  "body",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	err = publisher.Publish(ctx, body, "notification."+domain.NotificationTypeInfo)
	require.NoError(t, err)

	select {
	case msg := <-deliveries:
		var got map[string]string
		require.NoError(t, json.Unmarshal(msg.Body, &got))
		require.Equal(t, payload["room"], got["room"])
		require.Equal(t, payload["type"], got["type"])
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for published message")
	}
}

// setupRabbitMQContainer is defined in testhelpers_integration.go
