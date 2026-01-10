package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func ginTestMode() {
	gin.SetMode(gin.TestMode)
}

type noopPublisher struct{}

func (n *noopPublisher) Publish(ctx context.Context, payload []byte, routingKey string) error {
	_ = ctx
	_ = payload
	_ = routingKey
	return nil
}

func TestSSEFlow(t *testing.T) {
	ginTestMode()

	cfg := &config.Config{
		HTTPAddr:     ":0",
		SSEHeartbeat: 5 * time.Second,
		HistoryLimit: 0,
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

	sseResp, err := http.Get(server.URL + "/sse/room-1?limit=0")
	require.NoError(t, err)
	defer func() { _ = sseResp.Body.Close() }()
	require.Equal(t, http.StatusOK, sseResp.StatusCode)

	payload := map[string]string{
		"room":  "room-1",
		"type":  domain.NotificationTypeInfo,
		"title": "hello",
		"body":  "world",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	postResp, err := http.Post(server.URL+"/notifications", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = postResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, postResp.StatusCode)

	data, err := readSSEData(sseResp.Body, 2*time.Second)
	require.NoError(t, err)

	var got model.Notification
	require.NoError(t, json.Unmarshal([]byte(data), &got))
	require.Equal(t, payload["room"], got.Room)
	require.Equal(t, payload["type"], got.Type)
	require.Equal(t, payload["title"], got.Title)
	require.Equal(t, payload["body"], got.Body)
}

func readSSEData(body io.Reader, timeout time.Duration) (string, error) {
	reader := bufio.NewReader(body)
	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		var dataLines []string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- result{"", err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if len(dataLines) > 0 {
					ch <- result{strings.Join(dataLines, "\n"), nil}
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
	}()

	select {
	case res := <-ch:
		return res.data, res.err
	case <-time.After(timeout):
		return "", context.DeadlineExceeded
	}
}
