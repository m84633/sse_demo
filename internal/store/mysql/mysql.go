package mysql

import (
	"go.uber.org/zap"
	"sse_demo/internal/db"
)

type Store struct {
	queries *db.Queries
	log     *zap.Logger
}

func New(queries *db.Queries, logger *zap.Logger) *Store {
	return &Store{queries: queries, log: logger}
}
