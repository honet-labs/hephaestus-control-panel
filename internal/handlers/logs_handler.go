package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
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
	module := strings.TrimSpace(c.Query("module"))
	level := strings.TrimSpace(c.Query("level"))
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	order := c.DefaultQuery("order", "desc")

	limit := 200
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
			if limit > 2000 {
				limit = 2000
			}
		}
	}

	moduleSet := make(map[string]struct{})
	for _, entry := range logs {
		if entry.Module != "" {
			moduleSet[entry.Module] = struct{}{}
		}
	}
	var modules []string
	for m := range moduleSet {
		modules = append(modules, m)
	}
	sort.Strings(modules)

	var filtered []logger.LogEntry
	for _, entry := range logs {
		if module != "" && !strings.EqualFold(entry.Module, module) && !strings.Contains(strings.ToLower(entry.Module), strings.ToLower(module)) {
			continue
		}
		if level != "" && level != "ALL" && !strings.EqualFold(entry.Level, level) {
			continue
		}
		if query != "" {
			match := strings.Contains(strings.ToLower(entry.Message), query) ||
				strings.Contains(strings.ToLower(entry.Module), query) ||
				strings.Contains(strings.ToLower(entry.Error), query)
			if !match && entry.Fields != nil {
				for _, v := range entry.Fields {
					if strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), query) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	total := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	// Reverse if descending order requested (newest first)
	if order == "desc" {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    filtered,
		"total":   total,
		"modules": modules,
	})
}

// WebSocket Live Log Stream Endpoint: /ws/logs
func (h *LogsHandler) StreamLogsWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("Sec-WebSocket-Protocol")
	}
	if token == "" {
		if cookie, err := c.Cookie("hephaestus_session"); err == nil && cookie != "" {
			token = cookie
		}
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

		// Enforce permission: User must be admin or have "settings" feature read/manage permission
		hasPerm := user.IsAdmin() || domain.IsAdminRole(user.Role)
		if !hasPerm && user.Permissions != nil {
			if wild, ok := user.Permissions["*"]; ok && strings.EqualFold(wild, "manage") {
				hasPerm = true
			} else if p, ok := user.Permissions["settings"]; ok {
				pLower := strings.ToLower(p)
				if pLower == "read" || pLower == "manage" {
					hasPerm = true
				}
			}
		}
		if !hasPerm {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Forbidden: insufficient permissions to stream system logs"})
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
