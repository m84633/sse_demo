package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	"sse_demo/internal/http/dto"
	"sse_demo/internal/http/resp"
	"sse_demo/internal/model"
	"sse_demo/internal/queue"
	"sse_demo/internal/repository"
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

type publisherMock struct {
	mock.Mock
}

func (m *publisherMock) Publish(ctx context.Context, payload []byte, routingKey string) error {
	args := m.Called(ctx, payload, routingKey)
	return args.Error(0)
}

func setupRouter(t *testing.T, repo repository.NotificationRepository, publisher queue.Publisher) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		RabbitPublishPrefix: "notification",
		HistoryLimit:        10,
	}
	hub := sse.NewHub()
	svc := notify.NewService(repo, hub, zap.NewNop())
	handler := NewHandler(cfg, svc, hub, zap.NewNop(), publisher)

	router := gin.New()
	router.POST("/notifications", handler.CreateNotification)
	router.POST("/notifications/publish", handler.PublishNotification)
	return router
}

func performJSONRequest(t *testing.T, router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateNotificationController(t *testing.T) {
	t.Run("missing fields", func(t *testing.T) {
		repo := &repoMock{}
		router := setupRouter(t, repo, &publisherMock{})

		rec := performJSONRequest(t, router, http.MethodPost, "/notifications", map[string]string{
			"room": "room-1",
		})

		require.Equal(t, http.StatusBadRequest, rec.Code)
		var respBody dto.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &respBody))
		require.Equal(t, resp.CodeBadRequest, respBody.Code)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("invalid type", func(t *testing.T) {
		repo := &repoMock{}
		router := setupRouter(t, repo, &publisherMock{})

		rec := performJSONRequest(t, router, http.MethodPost, "/notifications", map[string]string{
			"room":  "room-1",
			"type":  "bad",
			"title": "title",
			"body":  "body",
		})

		require.Equal(t, http.StatusBadRequest, rec.Code)
		var respBody dto.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &respBody))
		require.Equal(t, resp.CodeBadRequest, respBody.Code)
		repo.AssertNotCalled(t, "CreateNotification", mock.Anything, mock.Anything)
	})

	t.Run("success", func(t *testing.T) {
		repo := &repoMock{}
		repo.On("CreateNotification", mock.Anything, mock.Anything).Return(model.Notification{
			ID:    99,
			Room:  "room-1",
			Type:  domain.NotificationTypeInfo,
			Title: "title",
			Body:  "body",
		}, nil).Once()
		router := setupRouter(t, repo, &publisherMock{})

		rec := performJSONRequest(t, router, http.MethodPost, "/notifications", map[string]string{
			"room":  "room-1",
			"type":  domain.NotificationTypeInfo,
			"title": "title",
			"body":  "body",
		})

		require.Equal(t, http.StatusCreated, rec.Code)
		var respBody model.Notification
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &respBody))
		require.Equal(t, int64(99), respBody.ID)
		require.Equal(t, domain.NotificationTypeInfo, respBody.Type)
		repo.AssertExpectations(t)
	})
}

func TestPublishNotificationController(t *testing.T) {
	t.Run("publish success", func(t *testing.T) {
		repo := &repoMock{}
		pub := &publisherMock{}
		pub.On("Publish", mock.Anything, mock.Anything, "notification."+domain.NotificationTypeInfo).Return(nil).Once()
		router := setupRouter(t, repo, pub)

		rec := performJSONRequest(t, router, http.MethodPost, "/notifications/publish", map[string]string{
			"room":  "room-1",
			"type":  domain.NotificationTypeInfo,
			"title": "title",
			"body":  "body",
		})

		require.Equal(t, http.StatusAccepted, rec.Code)
		pub.AssertExpectations(t)
		repo.AssertExpectations(t)

		var payload map[string]string
		call := pub.Calls[0]
		require.Len(t, call.Arguments, 3)
		body := call.Arguments.Get(1).([]byte)
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, "room-1", payload["room"])
		require.Equal(t, domain.NotificationTypeInfo, payload["type"])
	})

	t.Run("publish error", func(t *testing.T) {
		repo := &repoMock{}
		pub := &publisherMock{}
		pub.On("Publish", mock.Anything, mock.Anything, "notification."+domain.NotificationTypeInfo).Return(errors.New("publish failed")).Once()
		router := setupRouter(t, repo, pub)

		rec := performJSONRequest(t, router, http.MethodPost, "/notifications/publish", map[string]string{
			"room":  "room-1",
			"type":  domain.NotificationTypeInfo,
			"title": "title",
			"body":  "body",
		})

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		var respBody dto.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &respBody))
		require.Equal(t, resp.CodeInternalError, respBody.Code)
		pub.AssertExpectations(t)
		repo.AssertExpectations(t)
	})
}
