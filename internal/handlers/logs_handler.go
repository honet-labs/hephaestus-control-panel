package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type LogsHandler struct {
	authService *services.AuthService
}

func NewLogsHandler(authService *services.AuthService) *LogsHandler {
	return &LogsHandler{authService: authService}
}

func (h *LogsHandler) GetRecentLogs(c *gin.Context) {
	logs := logger.GetRecentLogs()
	module := c.Query("module")
	level := c.Query("level")
	limit := 0
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	var filtered []logger.LogEntry
	for _, entry := range logs {
		if module != "" && !strings.Contains(strings.ToLower(entry.Module), strings.ToLower(module)) {
			continue
		}
		if level != "" && level != "ALL" && !strings.EqualFold(entry.Level, level) {
			continue
		}
		filtered = append(filtered, entry)
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    filtered,
	})
}

// WebSocket Live Log Stream Endpoint: /ws/logs
func (h *LogsHandler) StreamLogsWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("Sec-WebSocket-Protocol")
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication token required"})
		return
	}

	if h.authService != nil {
		user, err := h.authService.ValidateSession(c.Request.Context(), token)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid or expired session token"})
			return
		}
	}

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
