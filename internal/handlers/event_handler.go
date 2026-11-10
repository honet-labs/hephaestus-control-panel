package handlers

import (
	"net/http"
	"strconv"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type EventHandler struct {
	service *services.EventService
}

func NewEventHandler(service *services.EventService) *EventHandler {
	return &EventHandler{service: service}
}

// GetSummary returns the cached in-memory summary (< 1ms latency, 0 DB load)
func (h *EventHandler) GetSummary(c *gin.Context) {
	summary := h.service.GetSummary()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

// ListEvents returns paginated historical events from PostgreSQL
func (h *EventHandler) ListEvents(c *gin.Context) {
	status := c.DefaultQuery("status", "all")
	severity := c.DefaultQuery("severity", "all")
	sourceType := c.DefaultQuery("sourceType", "all")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	events, total, err := h.service.ListEvents(c.Request.Context(), status, severity, sourceType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve events",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// AcknowledgeEvent marks an active event as acknowledged
func (h *EventHandler) AcknowledgeEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Event ID is required"})
		return
	}

	if err := h.service.AcknowledgeEvent(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Event acknowledged"})
}

// ResolveEvent manually marks an event as resolved
func (h *EventHandler) ResolveEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Event ID is required"})
		return
	}

	if err := h.service.ResolveEvent(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Event resolved"})
}

// ClearResolved removes resolved events older than N days (default 7 days)
func (h *EventHandler) ClearResolved(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("olderThanDays", "1"))
	deletedCount, err := h.service.ClearResolved(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Resolved events cleared",
		"deleted": deletedCount,
	})
}
