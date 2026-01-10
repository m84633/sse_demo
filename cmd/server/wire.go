//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"sse_demo/internal/app"
	"sse_demo/internal/config"
	"sse_demo/internal/http"
	"sse_demo/internal/http/controller"
	"sse_demo/internal/logging"
	"sse_demo/internal/service/notify"
	"sse_demo/internal/queue/rabbitmq"
	"sse_demo/internal/sse"
	"sse_demo/internal/store"
)

func InitializeApp() (*app.App, error) {
	wire.Build(
		config.New,
		logging.New,
		store.NewStore,
		sse.NewHub,
		notify.NewService,
		controller.NewHandler,
		http.NewRouter,
		rabbitmq.NewConsumer,
		rabbitmq.NewPublisher,
		app.NewApp,
	)
	return &app.App{}, nil
}
