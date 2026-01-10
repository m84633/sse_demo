package memory

import (
	"sync"

	"go.uber.org/zap"
	"sse_demo/internal/model"
)

type Store struct {
	mu      sync.Mutex
	nextID  int64
	records []model.Notification
	log     *zap.Logger
}

func New(logger *zap.Logger) *Store {
	return &Store{nextID: 1, log: logger}
}
