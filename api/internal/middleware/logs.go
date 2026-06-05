package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"time"
)

func LogsMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		if len(c.Errors) > 0 {
			logger.Errorw("Запрос завершился с ошибкой:",
				"path", path,
				"method", c.Request.Method,
				"status", c.Writer.Status(),
				"latency", latency,
				"user-agent", c.Request.UserAgent(),
				"ip-address", c.ClientIP(),
				"errors", fmt.Sprintf("%s", c.Errors),
			)
		} else {
			logger.Infow("Запрос совершен:",
				"path", path,
				"method", c.Request.Method,
				"status", c.Writer.Status(),
				"latency", latency,
				"user-agent", c.Request.UserAgent(),
				"ip-address", c.ClientIP(),
			)
		}
	}
}
