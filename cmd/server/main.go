package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := InitializeApp()
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	logger := app.Logger()
	defer func() {
		_ = logger.Sync()
	}()

	go func() {
		if err := app.Run(ctx); err != nil {
			logger.Error("app stopped", zap.Error(err))
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
