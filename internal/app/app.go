package app

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"sse_demo/internal/config"
	"sse_demo/internal/queue"
	"sse_demo/internal/sse"
)

type App struct {
	cfg      *config.Config
	hub      *sse.Hub
	consumer queue.Consumer
	server   *http.Server
	logger   *zap.Logger
	wg       sync.WaitGroup
}

func NewApp(cfg *config.Config, hub *sse.Hub, consumer queue.Consumer, router *gin.Engine, logger *zap.Logger) *App {
	return &App{
		cfg:      cfg,
		hub:      hub,
		consumer: consumer,
		server: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: router,
		},
		logger: logger,
	}
}

func (a *App) Run(ctx context.Context) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.hub.Run(ctx)
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.consumer.Start(ctx); err != nil && ctx.Err() == nil {
			a.logger.Error("consumer stopped", zap.Error(err))
		}
	}()
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("graceful shutdown started")
	shutdownErr := a.server.Shutdown(ctx)

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("graceful shutdown completed")
		return shutdownErr
	case <-ctx.Done():
		if shutdownErr != nil {
			return shutdownErr
		}
		return ctx.Err()
	}
}

func (a *App) Logger() *zap.Logger {
	return a.logger
}
