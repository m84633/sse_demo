package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	httpserver "sse_demo/internal/http"
	"sse_demo/internal/http/controller"
	"sse_demo/internal/model"
	"sse_demo/internal/queue"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/sse"
	"sse_demo/internal/store/memory"
)

func TestSSEHistoryBackfill(t *testing.T) {
	ginTestMode()

	cfg := &config.Config{
		HTTPAddr:     ":0",
		SSEHeartbeat: 5 * time.Second,
		HistoryLimit: 10,
	}
	logger := zap.NewNop()
	repo := memory.New(logger)
	hub := sse.NewHub()
	svc := notify.NewService(repo, hub, logger)
	handler := controller.NewHandler(cfg, svc, hub, logger, queue.Publisher(&noopPublisher{}))
	router := httpserver.NewRouter(handler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(router)
	defer server.Close()

	payload := map[string]string{
		"room":  "room-1",
		"type":  domain.NotificationTypeInfo,
		"title": "history",
		"body":  "before",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	postResp, err := http.Post(server.URL+"/notifications", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = postResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, postResp.StatusCode)

	sseResp, err := http.Get(server.URL + "/sse/room-1?limit=10")
	require.NoError(t, err)
	defer func() { _ = sseResp.Body.Close() }()
	require.Equal(t, http.StatusOK, sseResp.StatusCode)

	data, err := readSSEData(sseResp.Body, 2*time.Second)
	require.NoError(t, err)

	var got model.Notification
	require.NoError(t, json.Unmarshal([]byte(data), &got))
	require.Equal(t, payload["room"], got.Room)
	require.Equal(t, payload["type"], got.Type)
	require.Equal(t, payload["title"], got.Title)
	require.Equal(t, payload["body"], got.Body)
}
