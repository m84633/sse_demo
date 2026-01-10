package memory

import (
	"context"
	"time"

	"sse_demo/internal/model"
)

func (s *Store) CreateNotification(_ context.Context, notification model.Notification) (model.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	notification.ID = s.nextID
	s.nextID++
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now().UTC()
	}
	s.records = append(s.records, notification)
	return notification, nil
}

func (s *Store) ListNotifications(_ context.Context, room string, limit int) ([]model.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []model.Notification
	for i := len(s.records) - 1; i >= 0; i-- {
		record := s.records[i]
		if record.Room != room {
			continue
		}
		result = append(result, record)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}
