package middleware

import (
	"time"

	"go-hephaestus/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLoggerMiddleware logs HTTP requests and assigns a unique Request ID
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Header("X-Request-ID", reqID)
		c.Set("requestId", reqID)

		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := map[string]interface{}{
			"requestId": reqID,
			"status":    status,
			"method":    c.Request.Method,
			"path":      path,
			"ip":        c.ClientIP(),
			"latencyMs": float64(latency.Microseconds()) / 1000.0,
		}

		if status >= 500 {
			logger.Error("HTTP", "Server Error Response", nil, fields)
		} else if status >= 400 {
			logger.Warn("HTTP", "Client Warning Response", fields)
		} else {
			logger.Debug("HTTP", "Request Processed", fields)
		}
	}
}
