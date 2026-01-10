package queue

import "context"

type Consumer interface {
	Start(ctx context.Context) error
}

type Publisher interface {
	Publish(ctx context.Context, payload []byte, routingKey string) error
}
