package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type OpenSearchHandler struct {
	openSearchService *services.OpenSearchService
	shareRepo         *repository.ConnectionShareRepository
}

func NewOpenSearchHandler(openSearchService *services.OpenSearchService) *OpenSearchHandler {
	return &OpenSearchHandler{
		openSearchService: openSearchService,
		shareRepo:         repository.NewConnectionShareRepository(),
	}
}

func (h *OpenSearchHandler) getRequestContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	configID := strings.TrimSpace(c.Query("configId"))
	if configID == "" {
		configID = strings.TrimSpace(c.GetHeader("X-OpenSearch-Config-Id"))
	}
	if configID != "" {
		return services.ContextWithConfigID(ctx, configID)
	}
	return ctx
}

func (h *OpenSearchHandler) GetHealth(c *gin.Context) {
	health, err := h.openSearchService.GetClusterHealth(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": health})
}

func (h *OpenSearchHandler) GetNodesStats(c *gin.Context) {
	stats, err := h.openSearchService.GetNodesStats(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *OpenSearchHandler) GetNodesInfo(c *gin.Context) {
	info, err := h.openSearchService.GetNodesInfo(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": info})
}

func (h *OpenSearchHandler) GetIndices(c *gin.Context) {
	indices, err := h.openSearchService.GetIndices(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": indices})
}

func (h *OpenSearchHandler) GetShards(c *gin.Context) {
	shards, err := h.openSearchService.GetShards(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shards})
}

func (h *OpenSearchHandler) GetRecovery(c *gin.Context) {
	recovery, err := h.openSearchService.GetRecovery(h.getRequestContext(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": recovery})
}

func (h *OpenSearchHandler) GetConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := strings.TrimSpace(c.Query("configId"))
	if configID == "" {
		configID = strings.TrimSpace(c.GetHeader("X-OpenSearch-Config-Id"))
	}
	var cfg *domain.OpenSearchConfig
	var err error
	if configID != "" {
		cfg, err = h.openSearchService.GetConfigByID(c.Request.Context(), configID)
	} else {
		cfg, err = h.openSearchService.GetActiveConfigForUser(c.Request.Context(), userID, userRole)
	}
	if err != nil || cfg == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}
	safeCfg := *cfg
	if safeCfg.Password != "" {
		safeCfg.Password = "••••••••"
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safeCfg})
}

func (h *OpenSearchHandler) ActivateConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		id = strings.TrimSpace(c.Query("id"))
	}
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = c.ShouldBindJSON(&req)
		id = strings.TrimSpace(req.ID)
	}
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}

	cfg, err := h.openSearchService.SetActiveConfig(c.Request.Context(), id, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	safeCfg := *cfg
	if safeCfg.Password != "" {
		safeCfg.Password = "••••••••"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("OpenSearch cluster connection '%s' activated successfully", safeCfg.Name),
		"data":    safeCfg,
	})
}

func (h *OpenSearchHandler) ListConfigs(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configs, err := h.openSearchService.ListConfigs(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	for i := range configs {
		if configs[i].Password != "" {
			configs[i].Password = "••••••••"
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": configs})
}

func (h *OpenSearchHandler) SaveConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	var req domain.OpenSearchConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	req.Host = strings.TrimSpace(req.Host)
	if req.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Cluster Host / IP is required"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "OpenSearch Cluster"
	}
	if req.Port <= 0 {
		req.Port = 9200
	}

	saved, err := h.openSearchService.SaveConfig(c.Request.Context(), req, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	safeSaved := *saved
	if safeSaved.Password != "" {
		safeSaved.Password = "••••••••"
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safeSaved})
}

func (h *OpenSearchHandler) DeleteConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Param("id")
	if id == "" {
		id = c.Query("id")
	}
	if err := h.openSearchService.DeleteConfig(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "OpenSearch configuration deleted successfully."})
}

// Sharing Handlers
func (h *OpenSearchHandler) ListShares(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}
	shares, err := h.shareRepo.ListShares(c.Request.Context(), "opensearch_shares", "config_id", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *OpenSearchHandler) AddShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"` // "read" or "manage"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body: userId is required"})
		return
	}

	// Verify current user can manage
	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "opensearch_configs", "opensearch_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to share this configuration"})
		return
	}

	if err := h.shareRepo.AddShare(c.Request.Context(), "opensearch_shares", "config_id", configID, req.UserID, req.Permission, currentUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access granted successfully"})
}

func (h *OpenSearchHandler) DeleteShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil || configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Valid configuration ID and user ID are required"})
		return
	}

	// Verify current user can manage
	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "opensearch_configs", "opensearch_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to modify shares for this configuration"})
		return
	}

	if err := h.shareRepo.DeleteShare(c.Request.Context(), "opensearch_shares", "config_id", configID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access revoked successfully"})
}

func (h *OpenSearchHandler) UpdateVisibility(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	var req struct {
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Visibility (private/public) is required"})
		return
	}

	// Verify current user is owner or admin
	hasAccess, isOwner, _, err := h.shareRepo.CheckAccess(c.Request.Context(), "opensearch_configs", "opensearch_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only the connection owner can change its visibility"})
		return
	}

	if err := h.shareRepo.UpdateVisibility(c.Request.Context(), "opensearch_configs", configID, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Visibility updated to %s", req.Visibility)})
}

func (h *OpenSearchHandler) TestConnection(c *gin.Context) {
	var req struct {
		ID        string `json:"id"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		UseSSL    bool   `json:"useSsl"`
		VerifySSL bool   `json:"verifySsl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid connection parameters"})
		return
	}

	req.Host = strings.TrimSpace(req.Host)

	// Resolve saved credentials ONLY if editing or testing an existing connection by ID
	if req.ID != "" && services.IsMaskedOrEmptyPassword(req.Password) {
		var saved *domain.OpenSearchConfig
		if req.ID != "opensearch-active" {
			saved, _ = h.openSearchService.GetConfigByID(c.Request.Context(), req.ID)
		}
		if saved == nil {
			saved, _ = h.openSearchService.GetActiveConfig(c.Request.Context())
		}
		if saved != nil {
			if req.Host == "" {
				req.Host = saved.Host
			}
			if req.Port <= 0 {
				req.Port = saved.Port
			}
			if req.Username == "" {
				req.Username = saved.Username
			}
			req.Password = saved.Password
		}
	}

	if req.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Cluster Host / IP is required"})
		return
	}
	if req.Port <= 0 {
		req.Port = 9200
	}

	res, err := h.openSearchService.TestConnection(c.Request.Context(), req.Host, req.Port, req.Username, req.Password, req.UseSSL, req.VerifySSL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": res, "message": "Successfully connected to OpenSearch cluster."})
}

func (h *OpenSearchHandler) ExecuteQuery(c *gin.Context) {
	var req struct {
		Method   string      `json:"method"`
		Endpoint string      `json:"endpoint"`
		Body     interface{} `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request payload format: " + err.Error()})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "GET"
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		endpoint = "/_search"
	}

	var bodyBytes []byte
	if req.Body != nil {
		switch v := req.Body.(type) {
		case string:
			bodyBytes = []byte(v)
		default:
			bodyBytes, _ = json.Marshal(v)
		}
	}

	res, err := h.openSearchService.ExecuteQuery(h.getRequestContext(c), method, endpoint, bodyBytes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    res,
	})
}

type PrometheusHandler struct {
	promService *services.PrometheusService
}

func NewPrometheusHandler(promService *services.PrometheusService) *PrometheusHandler {
	return &PrometheusHandler{promService: promService}
}

func (h *PrometheusHandler) Query(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "query parameter is required"})
		return
	}

	data, err := h.promService.Query(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PrometheusHandler) Reload(c *gin.Context) {
	if err := h.promService.ReloadConfig(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Prometheus configuration reloaded."})
}

func (h *PrometheusHandler) TestConnection(c *gin.Context) {
	var cfg domain.PrometheusConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input format: " + err.Error()})
		return
	}

	ok, msg, err := h.promService.TestConnection(c.Request.Context(), cfg)
	if !ok || err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   msg,
			"message": msg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
	})
}

func (h *PrometheusHandler) GetConfig(c *gin.Context) {
	instanceID := c.Query("instanceId")
	content, cfg, err := h.promService.GetConfigFile(c.Request.Context(), instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"instanceId":   cfg.ID,
			"instanceName": cfg.Name,
			"path":         cfg.Path,
			"mode":         cfg.Mode,
			"content":      content,
		},
	})
}

func (h *PrometheusHandler) SaveConfig(c *gin.Context) {
	var req struct {
		InstanceID string `json:"instanceId"`
		YAML       string `json:"yaml"`
		Reload     bool   `json:"reload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	saveRes, err := h.promService.SaveConfigFile(c.Request.Context(), req.InstanceID, req.YAML, req.Reload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": saveRes.Message,
		"data":    saveRes,
	})
}

func (h *PrometheusHandler) ValidateConfig(c *gin.Context) {
	var req struct {
		InstanceID string `json:"instanceId"`
		YAML       string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	valid, issues := h.promService.ValidateYAML(c.Request.Context(), req.YAML, req.InstanceID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   valid,
		"issues":  issues,
	})
}

