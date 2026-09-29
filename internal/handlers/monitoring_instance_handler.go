package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type MonitoringInstanceHandler struct {
	service *services.MonitoringInstanceService
}

func NewMonitoringInstanceHandler(service *services.MonitoringInstanceService) *MonitoringInstanceHandler {
	return &MonitoringInstanceHandler{service: service}
}

// ListInstances handles GET /api/v1/monitoring/instances
func (h *MonitoringInstanceHandler) ListInstances(c *gin.Context) {
	userID, userRole := getUserContext(c)
	group := c.Query("group")
	tag := c.Query("tag")
	fetchMetrics := c.DefaultQuery("metrics", "true") == "true"

	instances, err := h.service.ListInstances(c.Request.Context(), userID, userRole, group, tag, fetchMetrics)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list monitoring instances",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    instances,
	})
}

// GetGroups handles GET /api/v1/monitoring/instances/groups
func (h *MonitoringInstanceHandler) GetGroups(c *gin.Context) {
	userID, userRole := getUserContext(c)
	groups, err := h.service.GetDistinctGroups(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve groups",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
	})
}

// GetInstance handles GET /api/v1/monitoring/instances/:id
func (h *MonitoringInstanceHandler) GetInstance(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	inst, err := h.service.GetInstance(c.Request.Context(), id, userID, userRole, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Instance not found or access denied",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    inst,
	})
}

// CreateInstance handles POST /api/v1/monitoring/instances
func (h *MonitoringInstanceHandler) CreateInstance(c *gin.Context) {
	userID, _ := getUserContext(c)
	var req domain.CreateMonitoringInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	inst, err := h.service.CreateInstance(c.Request.Context(), &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create monitoring instance",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Monitoring instance created successfully",
		"data":    inst,
	})
}

// UpdateInstance handles PUT /api/v1/monitoring/instances/:id
func (h *MonitoringInstanceHandler) UpdateInstance(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	var req domain.UpdateMonitoringInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	inst, err := h.service.UpdateInstance(c.Request.Context(), id, &req, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update monitoring instance",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Monitoring instance updated successfully",
		"data":    inst,
	})
}

// DeleteInstance handles DELETE /api/v1/monitoring/instances/:id
func (h *MonitoringInstanceHandler) DeleteInstance(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	if err := h.service.DeleteInstance(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete monitoring instance",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Monitoring instance deleted successfully",
	})
}

// ToggleAlert handles PUT /api/v1/monitoring/instances/:id/alert
func (h *MonitoringInstanceHandler) ToggleAlert(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	var body struct {
		AlertEnabled bool `json:"alertEnabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if err := h.service.ToggleAlert(c.Request.Context(), id, body.AlertEnabled, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Alert status updated successfully",
	})
}

// SyncFromRemoteHosts handles POST /api/v1/monitoring/instances/sync-remote-hosts
func (h *MonitoringInstanceHandler) SyncFromRemoteHosts(c *gin.Context) {
	userID, userRole := getUserContext(c)
	var req domain.SyncRemoteHostsRequest
	_ = c.ShouldBindJSON(&req) // optional body

	synced, err := h.service.SyncFromRemoteHosts(c.Request.Context(), &req, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to sync remote hosts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Remote hosts synchronized successfully",
		"data":    synced,
		"count":   len(synced),
	})
}

// GetInstanceHistory handles GET /api/v1/monitoring/instances/:id/history
func (h *MonitoringInstanceHandler) GetInstanceHistory(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")
	timeRange := c.DefaultQuery("range", "24h")

	history, err := h.service.GetInstanceHistory(c.Request.Context(), id, timeRange, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch metric history",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
	})
}

// GetContainerHistory handles GET /api/v1/monitoring/containers/history
func (h *MonitoringInstanceHandler) GetContainerHistory(c *gin.Context) {
	containerID := c.Query("containerId")
	containerName := c.Query("containerName")
	hostname := c.Query("hostname")
	ipAddress := c.Query("ipAddress")
	timeRange := c.DefaultQuery("range", "24h")

	if containerID == "" && containerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Either containerId or containerName is required",
		})
		return
	}

	history, err := h.service.GetContainerHistory(c.Request.Context(), containerID, containerName, hostname, ipAddress, timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch container metric history",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
	})
}

// ListShares handles GET /api/v1/monitoring/instances/:id/shares
func (h *MonitoringInstanceHandler) ListShares(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	shares, err := h.service.ListShares(c.Request.Context(), id, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list shares",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    shares,
	})
}

// AddShare handles POST /api/v1/monitoring/instances/:id/shares
func (h *MonitoringInstanceHandler) AddShare(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")

	var req domain.ShareMonitoringInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid share request"})
		return
	}

	if err := h.service.AddShare(c.Request.Context(), id, &req, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Instance shared successfully",
	})
}

// DeleteShare handles DELETE /api/v1/monitoring/instances/:id/shares/:userId
func (h *MonitoringInstanceHandler) DeleteShare(c *gin.Context) {
	currentUserID, userRole := getUserContext(c)
	id := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	if err := h.service.DeleteShare(c.Request.Context(), id, targetUserID, currentUserID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Share revoked successfully",
	})
}

// GetEngineStatus handles GET /api/v1/monitoring/instances/engine/status
func (h *MonitoringInstanceHandler) GetEngineStatus(c *gin.Context) {
	status := h.service.GetEngineStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// SetEngineInterval handles POST /api/v1/monitoring/instances/engine/interval
func (h *MonitoringInstanceHandler) SetEngineInterval(c *gin.Context) {
	var body struct {
		Interval string `json:"interval" binding:"required"` // "30s", "1m", "5m"
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Interval required (e.g. '30s', '1m', '5m')"})
		return
	}

	var dur time.Duration
	switch strings.ToLower(strings.TrimSpace(body.Interval)) {
	case "30s", "30":
		dur = 30 * time.Second
	case "1m", "60s", "60":
		dur = 1 * time.Minute
	case "5m", "300s", "300":
		dur = 5 * time.Minute
	default:
		parsed, err := time.ParseDuration(body.Interval)
		if err != nil || parsed < 5*time.Second {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid interval. Supported: '30s', '1m', '5m'"})
			return
		}
		dur = parsed
	}

	h.service.SetPollInterval(dur)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Auto-refresh polling queue interval set to %v", dur),
		"data":    h.service.GetEngineStatus(),
	})
}

// PollNow handles POST /api/v1/monitoring/instances/poll-now
func (h *MonitoringInstanceHandler) PollNow(c *gin.Context) {
	h.service.TriggerImmediatePoll()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Prometheus metric polling queue triggered immediately",
		"data":    h.service.GetEngineStatus(),
	})
}

// ListContainers handles GET /api/v1/monitoring/containers
func (h *MonitoringInstanceHandler) ListContainers(c *gin.Context) {
	host := c.Query("host")
	containers, err := h.service.ListDockerContainers(c.Request.Context(), host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list docker containers",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    containers,
	})
}


