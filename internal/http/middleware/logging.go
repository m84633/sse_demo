package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		if raw != "" {
			path = fmt.Sprintf("%s?%s", path, raw)
		}
		latency := time.Since(start)
		status := c.Writer.Status()
		errs := c.Errors.ByType(gin.ErrorTypePrivate)

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}

		if len(errs) > 0 {
			logger.Error("request failed", append(fields, zap.String("errors", errs.String()))...)
			return
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("request completed", fields...)
		case status >= http.StatusBadRequest:
			logger.Warn("request completed", fields...)
		default:
			logger.Info("request completed", fields...)
		}
	}
}
