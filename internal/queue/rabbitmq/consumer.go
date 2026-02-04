package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
	"sse_demo/internal/queue"
	"sse_demo/internal/service/notify"
)

type noopConsumer struct{}

func (n *noopConsumer) Start(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type Consumer struct {
	url    string
	svc    *notify.Service
	logger *zap.Logger
	exchange    string
	queue       string
	routingKey  string
	consumerTag string
}

func NewConsumer(cfg *config.Config, svc *notify.Service, logger *zap.Logger) queue.Consumer {
	if cfg.RabbitMQURL == "" {
		return &noopConsumer{}
	}
	return &Consumer{
		url:         cfg.RabbitMQURL,
		svc:         svc,
		logger:      logger,
		exchange:    cfg.RabbitExchange,
		queue:       cfg.RabbitQueue,
		routingKey:  cfg.RabbitRoutingKey,
		consumerTag: cfg.RabbitConsumerTag,
	}
}

func (r *Consumer) Start(ctx context.Context) error {
	ctx, span := otel.Tracer("rabbitmq").Start(ctx, "rabbitmq.consume_loop")
	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", r.exchange),
		attribute.String("messaging.destination_kind", "exchange"),
		attribute.String("messaging.rabbitmq.routing_key", r.routingKey),
	)
	defer span.End()

	conn, err := amqp.Dial(r.url)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "dial failed")
		return fmt.Errorf("rabbitmq dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "channel failed")
		return fmt.Errorf("rabbitmq channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.Qos(10, 0, false); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "qos failed")
		return fmt.Errorf("rabbitmq qos: %w", err)
	}

	if err := ch.ExchangeDeclare(
		r.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "exchange declare failed")
		return fmt.Errorf("rabbitmq exchange declare: %w", err)
	}

	queueInfo, err := ch.QueueDeclare(
		r.queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "queue declare failed")
		return fmt.Errorf("rabbitmq queue declare: %w", err)
	}

	if err := ch.QueueBind(
		queueInfo.Name,
		r.routingKey,
		r.exchange,
		false,
		nil,
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "queue bind failed")
		return fmt.Errorf("rabbitmq queue bind: %w", err)
	}

	deliveries, err := ch.Consume(
		queueInfo.Name,
		r.consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "consume failed")
		return fmt.Errorf("rabbitmq consume: %w", err)
	}

	r.logger.Info("RabbitMQ consumer started",
		zap.String("exchange", r.exchange),
		zap.String("queue", queueInfo.Name),
		zap.String("routing_key", r.routingKey),
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-deliveries:
			if !ok {
				span.SetStatus(codes.Error, "deliveries closed")
				return errors.New("rabbitmq deliveries closed")
			}
			if err := r.handleMessage(ctx, msg); err != nil {
				span.RecordError(err)
				return err
			}
		}
	}
}

type payload struct {
	Room  string `json:"room"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (r *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) error {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Headers))
	ctx, span := otel.Tracer("rabbitmq").Start(ctx, "rabbitmq.handle_message")
	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", r.exchange),
		attribute.String("messaging.destination_kind", "exchange"),
		attribute.String("messaging.rabbitmq.routing_key", msg.RoutingKey),
	)
	defer span.End()

	var p payload
	if err := json.Unmarshal(msg.Body, &p); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid json")
		r.logger.Error("rabbitmq invalid json", zap.Error(err))
		return msg.Ack(false)
	}
	if p.Room == "" || p.Type == "" || p.Title == "" || p.Body == "" {
		span.SetStatus(codes.Error, "missing required fields")
		r.logger.Warn("rabbitmq missing required fields",
			zap.String("room", p.Room),
			zap.String("type", p.Type),
			zap.String("title", p.Title),
		)
		return msg.Ack(false)
	}

	notification := model.Notification{
		Room:  p.Room,
		Type:  p.Type,
		Title: p.Title,
		Body:  p.Body,
	}

	createCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.svc.Create(createCtx, notification)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, domain.ErrInvalidNotificationType) {
			span.SetStatus(codes.Error, "invalid notification type")
			r.logger.Warn("rabbitmq invalid notification type", zap.String("type", p.Type))
			return msg.Ack(false)
		}
		span.SetStatus(codes.Error, "create notification failed")
		r.logger.Error("rabbitmq create notification failed", zap.Error(err))
		if nackErr := msg.Nack(false, true); nackErr != nil {
			r.logger.Error("rabbitmq nack failed", zap.Error(nackErr))
		}
		return nil
	}

	return msg.Ack(false)
}
