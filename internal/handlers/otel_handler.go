package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type OTelHandler struct {
	otelService *services.OTelService
	otelRepo    *repository.OTelRepository
	shareRepo   *repository.ConnectionShareRepository
}

func NewOTelHandler(otelService *services.OTelService, otelRepo *repository.OTelRepository) *OTelHandler {
	return &OTelHandler{
		otelService: otelService,
		otelRepo:    otelRepo,
		shareRepo:   repository.NewConnectionShareRepository(),
	}
}

// ListHosts returns all configured OpenTelemetry hosts
func (h *OTelHandler) ListHosts(c *gin.Context) {
	if c.Query("checkStatus") == "true" {
		_ = h.otelService.CheckAllHostsStatus(c.Request.Context())
	}

	userID, userRole := getUserContext(c)
	hosts, err := h.otelRepo.List(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if hosts == nil {
		hosts = []domain.OpenTelemetryConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": hosts})
}

// GetHost returns a single host by ID
func (h *OTelHandler) GetHost(c *gin.Context) {
	id := c.Param("id")
	host, err := h.otelRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Host not found"})
		return
	}
	// Mask passwords in response
	if host.SSHPassword != nil && *host.SSHPassword != "" {
		masked := "********"
		host.SSHPassword = &masked
	}
	if host.SSHKey != nil && *host.SSHKey != "" {
		masked := "********"
		host.SSHKey = &masked
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": host})
}

// SaveHost creates or updates an OpenTelemetry host configuration
func (h *OTelHandler) SaveHost(c *gin.Context) {
	var cfg domain.OpenTelemetryConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON payload: " + err.Error()})
		return
	}

	urlID := c.Param("id")
	if urlID != "" {
		cfg.ID = urlID
	}

	if strings.TrimSpace(cfg.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Host profile name is required"})
		return
	}
	if strings.TrimSpace(cfg.SSHHost) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH Host IP or hostname is required"})
		return
	}
	if strings.TrimSpace(cfg.SSHUser) == "" {
		cfg.SSHUser = "root"
	}
	if cfg.SSHPort <= 0 {
		cfg.SSHPort = 22
	}
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		cfg.ConfigPath = "/etc/otelcol-contrib/config.yaml"
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = "otelcol-contrib"
	}
	if strings.TrimSpace(cfg.ReloadMode) == "" {
		cfg.ReloadMode = "restart"
	}

	userID, userRole := getUserContext(c)
	if err := h.otelRepo.Save(c.Request.Context(), cfg, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save host: " + err.Error()})
		return
	}

	// Trigger immediate background status check so status updates from unknown
	go func(hostID string) {
		_, _ = h.otelService.GetHostStatus(context.Background(), hostID)
	}(cfg.ID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host profile saved successfully", "data": cfg})
}

// DeleteHost removes an OpenTelemetry host configuration
func (h *OTelHandler) DeleteHost(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")
	if err := h.otelRepo.Delete(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host deleted successfully"})
}

// Sharing Handlers
func (h *OTelHandler) ListShares(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Host ID is required"})
		return
	}
	shares, err := h.shareRepo.ListShares(c.Request.Context(), "opentelemetry_shares", "config_id", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *OTelHandler) AddShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Host ID is required"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body: userId is required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "opentelemetry_configs", "opentelemetry_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to share this host profile"})
		return
	}

	if err := h.shareRepo.AddShare(c.Request.Context(), "opentelemetry_shares", "config_id", configID, req.UserID, req.Permission, currentUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access granted successfully"})
}

func (h *OTelHandler) DeleteShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil || configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Valid host ID and user ID are required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "opentelemetry_configs", "opentelemetry_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to modify shares for this host profile"})
		return
	}

	if err := h.shareRepo.DeleteShare(c.Request.Context(), "opentelemetry_shares", "config_id", configID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access revoked successfully"})
}

func (h *OTelHandler) UpdateVisibility(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	var req struct {
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Visibility (private/public) is required"})
		return
	}

	hasAccess, isOwner, _, err := h.shareRepo.CheckAccess(c.Request.Context(), "opentelemetry_configs", "opentelemetry_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only the connection owner can change its visibility"})
		return
	}

	if err := h.shareRepo.UpdateVisibility(c.Request.Context(), "opentelemetry_configs", configID, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Visibility updated to %s", req.Visibility)})
}

// TestHost verifies SSH connectivity and agent status
func (h *OTelHandler) TestHost(c *gin.Context) {
	var req struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		SSHHost     string  `json:"sshHost"`
		SSHPort     int     `json:"sshPort"`
		SSHUser     string  `json:"sshUser"`
		SSHAuth     string  `json:"sshAuth"`
		SSHPassword *string `json:"sshPassword"`
		SSHKey      *string `json:"sshKey"`
		ServiceName string  `json:"serviceName"`
		ConfigPath  string  `json:"configPath"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	var cfg domain.OpenTelemetryConfig
	if req.ID != "" && (req.SSHHost == "" || (req.SSHPassword != nil && *req.SSHPassword == "********")) {
		existing, err := h.otelRepo.GetByID(c.Request.Context(), req.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Host not found: " + err.Error()})
			return
		}
		cfg = *existing
		if req.SSHHost != "" && req.SSHHost != existing.SSHHost {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Supply fresh credentials when testing a different SSH destination"})
			return
		}
		if req.ServiceName != "" {
			cfg.ServiceName = req.ServiceName
		}
	} else {
		cfg = domain.OpenTelemetryConfig{
			ID:          req.ID,
			Name:        req.Name,
			SSHHost:     req.SSHHost,
			SSHPort:     req.SSHPort,
			SSHUser:     req.SSHUser,
			SSHAuth:     req.SSHAuth,
			SSHPassword: req.SSHPassword,
			SSHKey:      req.SSHKey,
			ServiceName: req.ServiceName,
			ConfigPath:  req.ConfigPath,
		}
	}

	if cfg.SSHPort <= 0 {
		cfg.SSHPort = 22
	}
	if cfg.SSHUser == "" {
		cfg.SSHUser = "root"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "otelcol-contrib"
	}

	ok, msg, status := h.otelService.TestHostConnection(c.Request.Context(), &cfg)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": msg, "status": status})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": msg, "status": status})
}

// GetHostStatus returns live systemd service status and logs for the host
func (h *OTelHandler) GetHostStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.otelService.GetHostStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

// GetConfigFile reads the remote OpenTelemetry configuration YAML
func (h *OTelHandler) GetConfigFile(c *gin.Context) {
	id := c.Param("id")
	content, err := h.otelService.GetConfigFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": content})
}

// SaveConfigFile validates, backups, writes config and optionally restarts the agent
func (h *OTelHandler) SaveConfigFile(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Content      string `json:"content" binding:"required"`
		Summary      string `json:"summary"`
		RestartAfter *bool  `json:"restartAfter"`
		Restart      *bool  `json:"restart"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration content is required"})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "system"
	}

	// Default to true so "Deploy & Restart Agent" always triggers service restart
	shouldRestart := true
	if req.RestartAfter != nil {
		shouldRestart = *req.RestartAfter
	} else if req.Restart != nil {
		shouldRestart = *req.Restart
	}

	res, err := h.otelService.SaveConfigFile(c.Request.Context(), id, req.Content, username, req.Summary, shouldRestart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": res.Message, "data": res})
}

// ValidateConfig checks OpenTelemetry configuration syntax and schema on the remote host
func (h *OTelHandler) ValidateConfig(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Content string `json:"content"`
	}
	_ = c.ShouldBindJSON(&req)

	res, err := h.otelService.ValidateConfig(c.Request.Context(), id, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

// RestartService triggers a service reload or restart
func (h *OTelHandler) RestartService(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req)

	mode := req.Mode
	if mode == "" {
		mode = "restart"
	}

	res, err := h.otelService.RestartService(c.Request.Context(), id, mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": res.Success, "data": res})
}

// GetPresets returns bundled OpenTelemetry pipeline templates
func (h *OTelHandler) GetPresets(c *gin.Context) {
	presets := h.otelService.GetPresets()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": presets})
}

// ListHistory returns version history for a given host config
func (h *OTelHandler) ListHistory(c *gin.Context) {
	id := c.Param("id")
	history, err := h.otelRepo.ListHistory(c.Request.Context(), id, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if history == nil {
		history = []domain.OpenTelemetryConfigHistory{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": history})
}
