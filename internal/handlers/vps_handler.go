package handlers

import (
	"net/http"
	"strconv"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type VpsHandler struct {
	vpsService *services.VpsService
}

func NewVpsHandler(vpsService *services.VpsService) *VpsHandler {
	return &VpsHandler{vpsService: vpsService}
}

func (h *VpsHandler) GetMetrics(c *gin.Context) {
	hostID := c.Param("id")
	metrics, err := h.vpsService.GetMetrics(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": metrics})
}

func (h *VpsHandler) GetProcesses(c *gin.Context) {
	hostID := c.Param("id")
	procs, err := h.vpsService.GetProcesses(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": procs})
}

func (h *VpsHandler) KillProcess(c *gin.Context) {
	hostID := c.Param("id")
	pidStr := c.Param("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid PID"})
		return
	}

	if err := h.vpsService.KillProcess(c.Request.Context(), hostID, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Process terminated successfully."})
}

func (h *VpsHandler) GetServices(c *gin.Context) {
	hostID := c.Param("id")
	svcs, err := h.vpsService.GetServices(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": svcs})
}

func (h *VpsHandler) GetNetworkInfo(c *gin.Context) {
	hostID := c.Param("id")
	netInfo, err := h.vpsService.GetNetworkInfo(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": netInfo})
}

func (h *VpsHandler) ControlService(c *gin.Context) {
	hostID := c.Param("id")
	var req struct {
		ServiceName string `json:"serviceName" binding:"required"`
		Action      string `json:"action" binding:"required"` // status, start, stop, restart, reload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "serviceName and action are required"})
		return
	}

	out, err := h.vpsService.ControlService(c.Request.Context(), hostID, req.ServiceName, req.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "output": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": out})
}

type GrokHandler struct {
	grokService *services.GrokService
}

func NewGrokHandler(grokService *services.GrokService) *GrokHandler {
	return &GrokHandler{grokService: grokService}
}

func (h *GrokHandler) Test(c *gin.Context) {
	var req struct {
		Pattern string `json:"pattern" binding:"required"`
		Text    string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pattern and text are required"})
		return
	}

	res := h.grokService.TestPattern(req.Pattern, req.Text)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

func (h *GrokHandler) GetPatterns(c *gin.Context) {
	patterns := h.grokService.GetPresetPatterns()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": patterns})
}

type DataPrepperHandler struct {
	dpService *services.DataPrepperService
}

func NewDataPrepperHandler(dpService *services.DataPrepperService) *DataPrepperHandler {
	return &DataPrepperHandler{dpService: dpService}
}

func (h *DataPrepperHandler) ListPipelines(c *gin.Context) {
	list, err := h.dpService.ListPipelines(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *DataPrepperHandler) ValidateYAML(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "content is required"})
		return
	}

	valid, msg := h.dpService.ValidateYAML(req.Content)
	c.JSON(http.StatusOK, gin.H{"success": valid, "message": msg})
}
