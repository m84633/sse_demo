package notify

import (
	"context"

	"go.uber.org/zap"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
	"sse_demo/internal/repository"
	"sse_demo/internal/sse"
)

type Service struct {
	store repository.NotificationRepository
	hub   *sse.Hub
	log   *zap.Logger
}

func NewService(store repository.NotificationRepository, hub *sse.Hub, logger *zap.Logger) *Service {
	return &Service{store: store, hub: hub, log: logger}
}

func (s *Service) Create(ctx context.Context, notification model.Notification) (model.Notification, error) {
	if !domain.IsValidNotificationType(notification.Type) {
		return model.Notification{}, domain.ErrInvalidNotificationType
	}
	created, err := s.store.CreateNotification(ctx, notification)
	if err != nil {
		s.log.Error("store create notification failed",
			zap.String("room", notification.Room),
			zap.String("type", notification.Type),
			zap.String("title", notification.Title),
			zap.Error(err),
		)
		return model.Notification{}, err
	}
	s.hub.Broadcast(created)
	return created, nil
}

func (s *Service) ListHistory(ctx context.Context, room string, limit int) ([]model.Notification, error) {
	history, err := s.store.ListNotifications(ctx, room, limit)
	if err != nil {
		s.log.Error("store list notifications failed", zap.String("room", room), zap.Int("limit", limit), zap.Error(err))
		return nil, err
	}
	return history, nil
}
