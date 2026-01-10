package store

import (
	"database/sql"
	"sse_demo/internal/db"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/repository"
	"sse_demo/internal/store/memory"
	"sse_demo/internal/store/mysql"
)

func NewStore(cfg *config.Config, logger *zap.Logger) (repository.NotificationRepository, error) {
	if cfg.MySQLDSN == "" {
		return memory.New(logger), nil
	}
	sqlDB, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		logger.Error("mysql open failed", zap.Error(err))
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		logger.Error("mysql ping failed", zap.Error(err))
		return nil, err
	}
	queries := db.New(sqlDB)
	return mysql.New(queries, logger), nil
}
