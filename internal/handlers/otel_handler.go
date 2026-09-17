package handlers

import (
	"net/http"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type OTelHandler struct {
	otelService *services.OTelService
	otelRepo    *repository.OTelRepository
}

func NewOTelHandler(otelService *services.OTelService, otelRepo *repository.OTelRepository) *OTelHandler {
	return &OTelHandler{
		otelService: otelService,
		otelRepo:    otelRepo,
	}
}

// ListHosts returns all configured OpenTelemetry hosts
func (h *OTelHandler) ListHosts(c *gin.Context) {
	hosts, err := h.otelRepo.List(c.Request.Context())
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

	if err := h.otelRepo.Save(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save host: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host profile saved successfully", "data": cfg})
}

// DeleteHost removes an OpenTelemetry host configuration
func (h *OTelHandler) DeleteHost(c *gin.Context) {
	id := c.Param("id")
	if err := h.otelRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host deleted successfully"})
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
		if req.SSHHost != "" {
			cfg.SSHHost = req.SSHHost
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
