package repository

import (
	"context"

	"sse_demo/internal/model"
)

type NotificationRepository interface {
	CreateNotification(ctx context.Context, notification model.Notification) (model.Notification, error)
	ListNotifications(ctx context.Context, room string, limit int) ([]model.Notification, error)
}
