package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Apply also when the binary is served directly, without the Nginx container.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
			// Bound request bodies even when clients bypass the Nginx proxy.
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20)
		}
		c.Next()
	}
}
