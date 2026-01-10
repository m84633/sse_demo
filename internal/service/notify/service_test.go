package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
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

func TestServiceCreate(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		repo := &repoMock{}
		hub := sse.NewHub()
		svc := NewService(repo, hub, zap.NewNop())

		_, err := svc.Create(context.Background(), model.Notification{
			Room:  "room-1",
			Type:  "bad",
			Title: "title",
			Body:  "body",
		})
		require.ErrorIs(t, err, domain.ErrInvalidNotificationType)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("store error", func(t *testing.T) {
		storeErr := errors.New("store failed")
		repo := &repoMock{}
		repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{}, storeErr).Once()
		hub := sse.NewHub()
		svc := NewService(repo, hub, zap.NewNop())

		_, err := svc.Create(context.Background(), model.Notification{
			Room:  "room-1",
			Type:  domain.NotificationTypeInfo,
			Title: "title",
			Body:  "body",
		})
		require.ErrorIs(t, err, storeErr)
		repo.AssertExpectations(t)
	})

	t.Run("broadcasts", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := sse.NewHub()
		go hub.Run(ctx)

		client := &sse.Client{
			Room: "room-1",
			Ch:   make(chan model.Notification, 1),
		}
		hub.Register(client)
		defer hub.Unregister(client)

		repo := &repoMock{}
		repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{
			ID:    42,
			Room:  "room-1",
			Type:  domain.NotificationTypeInfo,
			Title: "title",
			Body:  "body",
		}, nil).Once()
		svc := NewService(repo, hub, zap.NewNop())

		created, err := svc.Create(context.Background(), model.Notification{
			Room:  "room-1",
			Type:  domain.NotificationTypeInfo,
			Title: "title",
			Body:  "body",
		})
		require.NoError(t, err)
		require.Equal(t, int64(42), created.ID)
		repo.AssertExpectations(t)

		select {
		case got := <-client.Ch:
			require.Equal(t, int64(42), got.ID)
			require.Equal(t, "room-1", got.Room)
			require.Equal(t, domain.NotificationTypeInfo, got.Type)
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected broadcast to client")
		}
	})
}

func TestServiceListHistory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := []model.Notification{{ID: 1, Room: "room-1", Type: domain.NotificationTypeInfo}}
		repo := &repoMock{}
		repo.On("ListNotifications", mock.Anything, "room-1", 10).Return(expected, nil).Once()
		hub := sse.NewHub()
		svc := NewService(repo, hub, zap.NewNop())

		got, err := svc.ListHistory(context.Background(), "room-1", 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, expected[0].ID, got[0].ID)
		repo.AssertExpectations(t)
	})

	t.Run("store error", func(t *testing.T) {
		storeErr := errors.New("list failed")
		repo := &repoMock{}
		repo.On("ListNotifications", mock.Anything, "room-1", 10).Return([]model.Notification(nil), storeErr).Once()
		hub := sse.NewHub()
		svc := NewService(repo, hub, zap.NewNop())

		_, err := svc.ListHistory(context.Background(), "room-1", 10)
		require.ErrorIs(t, err, storeErr)
		repo.AssertExpectations(t)
	})
}
