package http

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"sse_demo/internal/http/controller"
	"sse_demo/internal/http/middleware"
)

func NewRouter(handler *controller.Handler, logger *zap.Logger) *gin.Engine {
	router := gin.New()
	router.Use(middleware.ZapLogger(logger), middleware.ZapRecovery(logger))

	router.GET("/health", func(c *gin.Context) {
		c.Status(200)
	})
	router.StaticFile("/", "./public/index.html")
	router.POST("/notifications", handler.CreateNotification)
	router.POST("/notifications/publish", handler.PublishNotification)
	router.GET("/sse/:room", handler.SSE)

	return router
}
