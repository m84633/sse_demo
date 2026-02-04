package mysql

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
	"sse_demo/internal/db"
	"sse_demo/internal/model"
)

func (s *Store) CreateNotification(ctx context.Context, notification model.Notification) (model.Notification, error) {
	ctx, span := otel.Tracer("mysql").Start(ctx, "mysql.create_notification")
	defer span.End()

	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now().UTC()
	}
	result, err := s.queries.CreateNotification(ctx, db.CreateNotificationParams{
		Room:  notification.Room,
		Type:  notification.Type,
		Title: notification.Title,
		Body:  notification.Body,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "create notification failed")
		s.log.Error("sql create notification failed",
			zap.String("room", notification.Room),
			zap.String("type", notification.Type),
			zap.String("title", notification.Title),
			zap.Error(err),
		)
		return model.Notification{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "last insert id failed")
		s.log.Error("sql last insert id failed", zap.Error(err))
		return model.Notification{}, err
	}
	notification.ID = id
	return notification, nil
}

func (s *Store) ListNotifications(ctx context.Context, room string, limit int) ([]model.Notification, error) {
	ctx, span := otel.Tracer("mysql").Start(ctx, "mysql.list_notifications")
	defer span.End()

	rows, err := s.queries.ListNotificationsByRoom(ctx, db.ListNotificationsByRoomParams{
		Room:  room,
		Limit: int32(limit),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "list notifications failed")
		s.log.Error("sql list notifications failed", zap.String("room", room), zap.Int("limit", limit), zap.Error(err))
		return nil, err
	}

	var result []model.Notification
	for _, row := range rows {
		result = append(result, model.Notification{
			ID:        row.ID,
			Room:      row.Room,
			Type:      row.Type,
			Title:     row.Title,
			Body:      row.Body,
			CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}
