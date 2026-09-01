package handlers

import (
	"net/http"

	"go-hephaestus/internal/logger"

	"github.com/gin-gonic/gin"
)

type LogsHandler struct{}

func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

func (h *LogsHandler) GetRecentLogs(c *gin.Context) {
	logs := logger.GetRecentLogs()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

// WebSocket Live Log Stream Endpoint: /ws/logs
func (h *LogsHandler) StreamLogsWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	logChan, unsubscribe := logger.Subscribe()
	defer unsubscribe()

	// Send initial recent logs
	recent := logger.GetRecentLogs()
	for _, entry := range recent {
		if err := ws.WriteJSON(entry); err != nil {
			return
		}
	}

	// Stream live logs as they happen
	for entry := range logChan {
		if err := ws.WriteJSON(entry); err != nil {
			return
		}
	}
}
