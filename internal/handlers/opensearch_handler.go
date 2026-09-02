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

func (h *OpenSearchHandler) TestConnection(c *gin.Context) {
	var req struct {
		Host      string `json:"host" binding:"required"`
		Port      int    `json:"port" binding:"required"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		UseSSL    bool   `json:"useSsl"`
		VerifySSL bool   `json:"verifySsl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid connection parameters"})
		return
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
