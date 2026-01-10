package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/domain"
	"sse_demo/internal/http/dto"
	"sse_demo/internal/http/resp"
	"sse_demo/internal/model"
	"sse_demo/internal/queue"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/sse"
)

type Handler struct {
	cfg *config.Config
	svc *notify.Service
	hub *sse.Hub
	log *zap.Logger
	pub queue.Publisher
}

func NewHandler(cfg *config.Config, svc *notify.Service, hub *sse.Hub, logger *zap.Logger, publisher queue.Publisher) *Handler {
	return &Handler{cfg: cfg, svc: svc, hub: hub, log: logger, pub: publisher}
}

func (h *Handler) CreateNotification(c *gin.Context) {
	var req dto.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "invalid json"})
		return
	}
	if req.Room == "" || req.Type == "" || req.Title == "" || req.Body == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "room, type, title, body are required"})
		return
	}
	created, err := h.svc.Create(c.Request.Context(), model.Notification{
		Room:  req.Room,
		Type:  req.Type,
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidNotificationType) {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "type must be one of: info, warning, system"})
			return
		}
		h.log.Error("create notification failed",
			zap.String("room", req.Room),
			zap.String("type", req.Type),
			zap.String("title", req.Title),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Code: resp.CodeInternalError, Message: "failed to create notification"})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) PublishNotification(c *gin.Context) {
	var req dto.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "invalid json"})
		return
	}
	if req.Room == "" || req.Type == "" || req.Title == "" || req.Body == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "room, type, title, body are required"})
		return
	}
	if !domain.IsValidNotificationType(req.Type) {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "type must be one of: info, warning, system"})
		return
	}

	payload, err := json.Marshal(map[string]string{
		"room":  req.Room,
		"type":  req.Type,
		"title": req.Title,
		"body":  req.Body,
	})
	if err != nil {
		h.log.Error("publish payload marshal failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Code: resp.CodeInternalError, Message: "failed to publish notification"})
		return
	}

	prefix := h.cfg.RabbitPublishPrefix
	if prefix == "" {
		prefix = "notification"
	}
	routingKey := prefix + "." + req.Type
	if err := h.pub.Publish(c.Request.Context(), payload, routingKey); err != nil {
		h.log.Error("publish notification failed",
			zap.String("room", req.Room),
			zap.String("type", req.Type),
			zap.String("title", req.Title),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Code: resp.CodeInternalError, Message: "failed to publish notification"})
		return
	}

	c.JSON(http.StatusAccepted, dto.StatusResponse{Code: resp.CodeQueued, Message: "queued"})
}

func (h *Handler) SSE(c *gin.Context) {
	room := c.Param("room")
	if room == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: resp.CodeBadRequest, Message: "room required"})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.log.Error("streaming unsupported", zap.String("room", room))
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Code: resp.CodeInternalError, Message: "streaming unsupported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	limit := h.cfg.HistoryLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			limit = n
		}
	}

	history, err := h.svc.ListHistory(c.Request.Context(), room, limit)
	if err != nil {
		h.log.Error("list history failed", zap.String("room", room), zap.Int("limit", limit), zap.Error(err))
	} else {
		for i := len(history) - 1; i >= 0; i-- {
			if err := writeNotification(c.Writer, history[i]); err != nil {
				h.log.Error("write history notification failed", zap.String("room", room), zap.Error(err))
				return
			}
		}
		flusher.Flush()
	}

	client := &sse.Client{
		Room: room,
		Ch:   make(chan model.Notification, 16),
	}
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	heartbeat := time.NewTicker(h.cfg.SSEHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
				h.log.Error("heartbeat write failed", zap.String("room", room), zap.Error(err))
				return
			}
			flusher.Flush()
		case notification, ok := <-client.Ch:
			if !ok {
				return
			}
			if err := writeNotification(c.Writer, notification); err != nil {
				h.log.Error("write notification failed", zap.String("room", room), zap.Error(err))
				return
			}
			flusher.Flush()
		}
	}
}

func writeNotification(w http.ResponseWriter, notification model.Notification) error {
	payload, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	// SSE frame mapping:
	// - id: notification.ID (event id)
	// - event: "notification" (JS uses addEventListener("notification", ...))
	// - data: JSON payload containing room/type/title/body/created_at
	_, err = fmt.Fprintf(w, "id: %d\nevent: notification\ndata: %s\n\n", notification.ID, payload)
	return err
}
