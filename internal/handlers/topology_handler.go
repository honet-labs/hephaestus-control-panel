package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TopologyHandler struct {
	topologyRepo *repository.TopologyRepository
	topoService  *services.TopologyService
	icmpService  *services.IcmpPingService
}

func NewTopologyHandler(topologyRepo *repository.TopologyRepository, topoService *services.TopologyService, icmpService *services.IcmpPingService) *TopologyHandler {
	return &TopologyHandler{
		topologyRepo: topologyRepo,
		topoService:  topoService,
		icmpService:  icmpService,
	}
}

func (h *TopologyHandler) PingDevice(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query param 'ip' is required"})
		return
	}
	output := h.icmpService.PingHostOutput(c.Request.Context(), ip, 4)
	c.JSON(http.StatusOK, gin.H{"success": true, "output": output})
}

func (h *TopologyHandler) GetGraph(c *gin.Context) {
	var sheetID *int
	if s := c.Query("sheetId"); s != "" {
		if id, err := strconv.Atoi(s); err == nil {
			sheetID = &id
		}
	}

	graph, err := h.topoService.GetGraph(c.Request.Context(), sheetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": graph})
}

// Sheets
func (h *TopologyHandler) ListSheets(c *gin.Context) {
	sheets, err := h.topologyRepo.ListSheets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sheets})
}

func (h *TopologyHandler) CreateSheet(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	sheet, err := h.topologyRepo.CreateSheet(c.Request.Context(), req.Name, req.SortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sheet})
}

func (h *TopologyHandler) UpdateSheet(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if err := h.topologyRepo.UpdateSheet(c.Request.Context(), id, req.Name, req.SortOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Sheet updated."})
}

func (h *TopologyHandler) DeleteSheet(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.topologyRepo.DeleteSheet(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Sheet deleted."})
}

// Devices
func (h *TopologyHandler) SaveDevice(c *gin.Context) {
	var dev domain.TopologyDevice
	if err := c.ShouldBindJSON(&dev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if dev.ID == "" {
		dev.ID = fmt.Sprintf("dev-%s", uuid.New().String()[:8])
	}

	if err := h.topologyRepo.SaveDevice(c.Request.Context(), dev); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": dev})
}

func (h *TopologyHandler) UpdatePosition(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if err := h.topologyRepo.UpdatePosition(c.Request.Context(), id, req.X, req.Y); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Position updated."})
}

func (h *TopologyHandler) DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.topologyRepo.DeleteDevice(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Device deleted."})
}

// Edges
func (h *TopologyHandler) SaveEdge(c *gin.Context) {
	var edge domain.TopologyEdge
	if err := c.ShouldBindJSON(&edge); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if err := h.topologyRepo.SaveEdge(c.Request.Context(), edge); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": edge})
}

func (h *TopologyHandler) DeleteEdge(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.topologyRepo.DeleteEdge(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Edge deleted."})
}

// Discovery
func (h *TopologyHandler) DiscoverPrometheus(c *gin.Context) {
	devices, err := h.topoService.DiscoverFromPrometheus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": devices})
}

func (h *TopologyHandler) ScanSubnet(c *gin.Context) {
	cidr := c.Query("cidr")
	if cidr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query param 'cidr' is required (e.g. 192.168.1.0/24)"})
		return
	}

	devices, err := h.topoService.ScanSubnet(c.Request.Context(), cidr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": devices})
}
