package middleware

import (
	"fmt"
	"net/http"
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
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		latencyMs := float64(latency.Microseconds()) / 1000.0

		// Don't flood live system logs with internal health checks
		if path == "/health" || path == "/nginx-health" {
			return
		}

		fullPath := path
		if rawQuery != "" {
			fullPath = path + "?" + rawQuery
		}

		statusText := http.StatusText(status)
		msg := fmt.Sprintf("%s %s -> %d %s (%.2fms) [%s]", method, fullPath, status, statusText, latencyMs, clientIP)

		fields := map[string]interface{}{
			"requestId": reqID,
			"status":    status,
			"method":    method,
			"path":      path,
			"query":     rawQuery,
			"ip":        clientIP,
			"latencyMs": latencyMs,
			"userAgent": c.Request.UserAgent(),
		}

		if len(c.Errors) > 0 {
			errStr := c.Errors.String()
			fields["error"] = errStr
			msg += " - " + errStr
		}

		if status >= 500 {
			logger.Error("HTTP", msg, nil, fields)
		} else if status >= 400 {
			logger.Warn("HTTP", msg, fields)
		} else {
			logger.Info("HTTP", msg, fields)
		}
	}
}
