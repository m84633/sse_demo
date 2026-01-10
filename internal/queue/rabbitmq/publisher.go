package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/queue"
)

type noopPublisher struct{}

func (n *noopPublisher) Publish(ctx context.Context, payload []byte, routingKey string) error {
	_ = ctx
	_ = payload
	_ = routingKey
	return nil
}

type Publisher struct {
	url    string
	logger *zap.Logger
	exchange string
}

func NewPublisher(cfg *config.Config, logger *zap.Logger) queue.Publisher {
	if cfg.RabbitMQURL == "" {
		return &noopPublisher{}
	}
	return &Publisher{url: cfg.RabbitMQURL, logger: logger, exchange: cfg.RabbitExchange}
}

func (p *Publisher) Publish(ctx context.Context, payload []byte, routingKey string) error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("rabbitmq dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("rabbitmq exchange declare: %w", err)
	}

	if err := ch.PublishWithContext(ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         payload,
		},
	); err != nil {
		p.logger.Error("rabbitmq publish failed", zap.Error(err))
		return err
	}

	return nil
}
