package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/sse"
)

type repoMock struct {
	mock.Mock
}

func (m *repoMock) CreateNotification(ctx context.Context, n model.Notification) (model.Notification, error) {
	args := m.Called(ctx, n)
	return args.Get(0).(model.Notification), args.Error(1)
}

func (m *repoMock) ListNotifications(ctx context.Context, room string, limit int) ([]model.Notification, error) {
	args := m.Called(ctx, room, limit)
	return args.Get(0).([]model.Notification), args.Error(1)
}

type ackMock struct {
	acked   int
	nacked  int
	requeue bool
}

func (a *ackMock) Ack(_ uint64, _ bool) error {
	a.acked++
	return nil
}

func (a *ackMock) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacked++
	a.requeue = requeue
	return nil
}

func (a *ackMock) Reject(_ uint64, _ bool) error {
	return nil
}

func TestConsumerHandleMessage(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		repo := &repoMock{}
		svc := notify.NewService(repo, sse.NewHub(), zap.NewNop())
		consumer := &Consumer{svc: svc, logger: zap.NewNop()}
		ack := &ackMock{}

		msg := amqp.Delivery{
			Body:         []byte("{bad json"),
			Acknowledger: ack,
		}

		err := consumer.handleMessage(context.Background(), msg)
		require.NoError(t, err)
		require.Equal(t, 1, ack.acked)
		require.Equal(t, 0, ack.nacked)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("missing fields", func(t *testing.T) {
		repo := &repoMock{}
		svc := notify.NewService(repo, sse.NewHub(), zap.NewNop())
		consumer := &Consumer{svc: svc, logger: zap.NewNop()}
		ack := &ackMock{}

		msg := amqp.Delivery{
			Body:         []byte(`{"room":"room-1"}`),
			Acknowledger: ack,
		}

		err := consumer.handleMessage(context.Background(), msg)
		require.NoError(t, err)
		require.Equal(t, 1, ack.acked)
		require.Equal(t, 0, ack.nacked)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("invalid type", func(t *testing.T) {
		repo := &repoMock{}
		svc := notify.NewService(repo, sse.NewHub(), zap.NewNop())
		consumer := &Consumer{svc: svc, logger: zap.NewNop()}
		ack := &ackMock{}

		msg := amqp.Delivery{
			Body:         []byte(`{"room":"room-1","type":"bad","title":"t","body":"b"}`),
			Acknowledger: ack,
		}

		err := consumer.handleMessage(context.Background(), msg)
		require.NoError(t, err)
		require.Equal(t, 1, ack.acked)
		require.Equal(t, 0, ack.nacked)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("store error -> nack", func(t *testing.T) {
		storeErr := errors.New("store failed")
		repo := &repoMock{}
		repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{}, storeErr).Once()
		svc := notify.NewService(repo, sse.NewHub(), zap.NewNop())
		consumer := &Consumer{svc: svc, logger: zap.NewNop()}
		ack := &ackMock{}

		msg := amqp.Delivery{
			Body:         []byte(`{"room":"room-1","type":"info","title":"t","body":"b"}`),
			Acknowledger: ack,
		}

		err := consumer.handleMessage(context.Background(), msg)
		require.NoError(t, err)
		require.Equal(t, 0, ack.acked)
		require.Equal(t, 1, ack.nacked)
		require.True(t, ack.requeue)
		repo.AssertExpectations(t)
	})

	t.Run("success -> ack", func(t *testing.T) {
		repo := &repoMock{}
		repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{
			ID:    1,
			Room:  "room-1",
			Type:  domain.NotificationTypeInfo,
			Title: "t",
			Body:  "b",
		}, nil).Once()
		svc := notify.NewService(repo, sse.NewHub(), zap.NewNop())
		consumer := &Consumer{svc: svc, logger: zap.NewNop()}
		ack := &ackMock{}

		payload, err := json.Marshal(map[string]string{
			"room":  "room-1",
			"type":  domain.NotificationTypeInfo,
			"title": "t",
			"body":  "b",
		})
		require.NoError(t, err)

		msg := amqp.Delivery{
			Body:         payload,
			Acknowledger: ack,
		}

		err = consumer.handleMessage(context.Background(), msg)
		require.NoError(t, err)
		require.Equal(t, 1, ack.acked)
		require.Equal(t, 0, ack.nacked)
		repo.AssertExpectations(t)
	})
}
