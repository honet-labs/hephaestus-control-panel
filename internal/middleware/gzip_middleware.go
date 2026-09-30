package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		// Use BestSpeed (Level 1): gives ~75-80% compression with near-zero CPU footprint
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return gz
	},
}

type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write([]byte(s))
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write(data)
}

func (g *gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	if code == http.StatusNoContent || code == http.StatusNotModified {
		g.Header().Del("Content-Encoding")
	}
	g.ResponseWriter.WriteHeader(code)
}

// GzipMiddleware compresses HTTP responses using gzip if requested by the client
func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only compress if the client explicitly accepts gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Never compress WebSocket connections or Server-Sent Events
		if c.GetHeader("Upgrade") != "" || strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
			c.Next()
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		gz.Reset(c.Writer)
		defer gz.Close()

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		c.Writer = &gzipWriter{
			ResponseWriter: c.Writer,
			writer:         gz,
		}

		c.Next()
	}
}
