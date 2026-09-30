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

	if sheetID != nil {
		userID, userRole := getUserContext(c)
		hasAccess, _, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), *sheetID, userID, userRole)
		if err != nil || !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you do not have permission to view this topology sheet"})
			return
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
	userID, userRole := getUserContext(c)
	sheets, err := h.topologyRepo.ListSheets(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sheets})
}

func (h *TopologyHandler) CreateSheet(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		SortOrder  int    `json:"sortOrder"`
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.Visibility != "public" {
		req.Visibility = "private"
	}

	userID, _ := getUserContext(c)
	var uid *int
	if userID > 0 {
		uid = &userID
	}

	sheet, err := h.topologyRepo.CreateSheet(c.Request.Context(), req.Name, req.SortOrder, uid, req.Visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sheet})
}

func (h *TopologyHandler) UpdateSheet(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, perm, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole) && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you do not have permission to edit this sheet"})
		return
	}

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

func (h *TopologyHandler) UpdateSheetVisibility(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only sheet owner or administrator can modify visibility"})
		return
	}

	var req struct {
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.Visibility != "public" {
		req.Visibility = "private"
	}

	if err := h.topologyRepo.UpdateSheetVisibility(c.Request.Context(), id, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Sheet visibility updated successfully."})
}

func (h *TopologyHandler) DeleteSheet(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only sheet owner or administrator can delete this sheet"})
		return
	}

	if err := h.topologyRepo.DeleteSheet(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Sheet deleted."})
}

// ==================== SHEET SHARING HANDLERS ====================

func (h *TopologyHandler) ListSheetShares(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only sheet owner or administrator can view shares"})
		return
	}

	shares, err := h.topologyRepo.ListSheetShares(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *TopologyHandler) AddSheetShare(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only sheet owner or administrator can share access"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "You cannot share a sheet with yourself"})
		return
	}

	if err := h.topologyRepo.AddSheetShare(c.Request.Context(), id, req.UserID, req.Permission, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access granted successfully."})
}

func (h *TopologyHandler) DeleteSheetShare(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	targetUserID, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), id, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only sheet owner or administrator can revoke access"})
		return
	}

	if err := h.topologyRepo.DeleteSheetShare(c.Request.Context(), id, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access revoked successfully."})
}

func (h *TopologyHandler) ListAvailableUsers(c *gin.Context) {
	users, err := h.topologyRepo.ListAvailableUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
}

// Devices
func (h *TopologyHandler) SaveDevice(c *gin.Context) {
	var dev domain.TopologyDevice
	if err := c.ShouldBindJSON(&dev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if dev.SheetID != nil {
		userID, userRole := getUserContext(c)
		hasAccess, isOwner, perm, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), *dev.SheetID, userID, userRole)
		if err == nil && (!hasAccess || (!isOwner && !domain.IsAdminRole(userRole) && perm != "manage")) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: read-only access for this sheet"})
			return
		}
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
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		SheetID *int    `json:"sheetId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if err := h.topologyRepo.UpdatePosition(c.Request.Context(), id, req.X, req.Y, req.SheetID); err != nil {
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

func (h *TopologyHandler) RemoveDeviceFromCanvas(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		SheetID *int `json:"sheetId"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.SheetID != nil {
		userID, userRole := getUserContext(c)
		hasAccess, isOwner, perm, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), *req.SheetID, userID, userRole)
		if err == nil && (!hasAccess || (!isOwner && !domain.IsAdminRole(userRole) && perm != "manage")) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: read-only access for this sheet"})
			return
		}
	}

	if err := h.topologyRepo.RemoveDeviceFromCanvas(c.Request.Context(), id, req.SheetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Device removed from canvas."})
}

// Edges
func (h *TopologyHandler) SaveEdge(c *gin.Context) {
	var edge domain.TopologyEdge
	if err := c.ShouldBindJSON(&edge); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if edge.SheetID != nil {
		userID, userRole := getUserContext(c)
		hasAccess, isOwner, perm, err := h.topologyRepo.CheckSheetAccess(c.Request.Context(), *edge.SheetID, userID, userRole)
		if err == nil && (!hasAccess || (!isOwner && !domain.IsAdminRole(userRole) && perm != "manage")) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: read-only access for this sheet"})
			return
		}
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

// SyncRemoteServers synchronizes registered Remote Hosts into Topology devices
func (h *TopologyHandler) SyncRemoteServers(c *gin.Context) {
	var sheetID *int
	if sheetParam := c.Query("sheetId"); sheetParam != "" {
		if id, err := strconv.Atoi(sheetParam); err == nil {
			sheetID = &id
		}
	}

	devices, err := h.topoService.SyncFromRemoteServers(c.Request.Context(), sheetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Successfully synced %d remote server(s) into topology.", len(devices)),
		"data":    devices,
	})
}

