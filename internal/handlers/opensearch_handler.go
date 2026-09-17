package handlers

import (
	"net/http"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type OpenSearchHandler struct {
	openSearchService *services.OpenSearchService
}

func NewOpenSearchHandler(openSearchService *services.OpenSearchService) *OpenSearchHandler {
	return &OpenSearchHandler{openSearchService: openSearchService}
}

func (h *OpenSearchHandler) GetHealth(c *gin.Context) {
	health, err := h.openSearchService.GetClusterHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": health})
}

func (h *OpenSearchHandler) GetNodesStats(c *gin.Context) {
	stats, err := h.openSearchService.GetNodesStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *OpenSearchHandler) GetNodesInfo(c *gin.Context) {
	info, err := h.openSearchService.GetNodesInfo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": info})
}

func (h *OpenSearchHandler) GetIndices(c *gin.Context) {
	indices, err := h.openSearchService.GetIndices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": indices})
}

func (h *OpenSearchHandler) GetShards(c *gin.Context) {
	shards, err := h.openSearchService.GetShards(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shards})
}

func (h *OpenSearchHandler) GetRecovery(c *gin.Context) {
	recovery, err := h.openSearchService.GetRecovery(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": recovery})
}

func (h *OpenSearchHandler) GetConfig(c *gin.Context) {
	cfg, err := h.openSearchService.GetActiveConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}
	safeCfg := *cfg
	if safeCfg.Password != "" {
		safeCfg.Password = "••••••••"
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safeCfg})
}

func (h *OpenSearchHandler) SaveConfig(c *gin.Context) {
	var req domain.OpenSearchConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	saved, err := h.openSearchService.SaveConfig(c.Request.Context(), req)
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
	id := c.Param("id")
	if id == "" {
		id = c.Query("id")
	}
	if err := h.openSearchService.DeleteConfig(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "OpenSearch configuration deleted successfully."})
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

	// Resolve saved credentials if password is empty or masked
	if services.IsMaskedOrEmptyPassword(req.Password) {
		var saved *domain.OpenSearchConfig
		if req.ID != "" && req.ID != "opensearch-active" {
			saved, _ = h.openSearchService.GetConfigByID(c.Request.Context(), req.ID)
		}
		if saved == nil {
			saved, _ = h.openSearchService.GetActiveConfig(c.Request.Context())
		}
		if saved != nil {
			// If testing existing ID, or if host was omitted, or if host matches the saved configuration
			if req.ID != "" || req.Host == "" || req.Host == saved.Host {
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
