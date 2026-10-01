package handlers

import (
	"net/http"
	"strconv"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type IpamHandler struct {
	service *services.IpamService
}

func NewIpamHandler(service *services.IpamService) *IpamHandler {
	return &IpamHandler{service: service}
}

// ListSubnets handles GET /api/v1/ipam/subnets
func (h *IpamHandler) ListSubnets(c *gin.Context) {
	subnets, err := h.service.ListSubnets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list subnets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    subnets,
	})
}

// GetSubnet handles GET /api/v1/ipam/subnets/:id
func (h *IpamHandler) GetSubnet(c *gin.Context) {
	id := c.Param("id")
	subnet, addresses, err := h.service.GetSubnet(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Subnet not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      subnet,
		"addresses": addresses,
	})
}

// CreateSubnet handles POST /api/v1/ipam/subnets
func (h *IpamHandler) CreateSubnet(c *gin.Context) {
	uid, _ := getUserContext(c)
	var uID *int
	if uid > 0 {
		uID = &uid
	}
	var req domain.CreateIpamSubnetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	subnet, err := h.service.CreateSubnet(c.Request.Context(), &req, uID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Subnet created successfully",
		"data":    subnet,
	})
}

// UpdateSubnet handles PUT /api/v1/ipam/subnets/:id
func (h *IpamHandler) UpdateSubnet(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateIpamSubnetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	subnet, err := h.service.UpdateSubnet(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subnet updated successfully",
		"data":    subnet,
	})
}

// DeleteSubnet handles DELETE /api/v1/ipam/subnets/:id
func (h *IpamHandler) DeleteSubnet(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteSubnet(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete subnet",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subnet and associated addresses deleted successfully",
	})
}

// TriggerScan handles POST /api/v1/ipam/subnets/:id/scan
func (h *IpamHandler) TriggerScan(c *gin.Context) {
	id := c.Param("id")
	log, err := h.service.ScanSubnet(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Scan failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subnet scan completed successfully",
		"data":    log,
	})
}

// ScanAllSubnets handles POST /api/v1/ipam/subnets/scan-all
func (h *IpamHandler) ScanAllSubnets(c *gin.Context) {
	res, err := h.service.ScanAllSubnets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Scan all subnets failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All subnets scanned successfully",
		"data":    res,
	})
}

// GetNextAvailableIP handles GET /api/v1/ipam/subnets/:id/next-available
func (h *IpamHandler) GetNextAvailableIP(c *gin.Context) {
	id := c.Param("id")
	ip, err := h.service.GetNextAvailableIP(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"available": ip,
	})
}

// ListScanLogs handles GET /api/v1/ipam/subnets/:id/logs
func (h *IpamHandler) ListScanLogs(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	logs, err := h.service.ListScanLogs(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve scan logs",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

// SaveAddress handles POST /api/v1/ipam/addresses
func (h *IpamHandler) SaveAddress(c *gin.Context) {
	var req domain.SaveIpamAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	addr, err := h.service.SaveAddress(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "IP address record saved",
		"data":    addr,
	})
}

// UpdateAddress handles PUT /api/v1/ipam/addresses/:id
func (h *IpamHandler) UpdateAddress(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateIpamAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	addr, err := h.service.UpdateAddress(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "IP address updated",
		"data":    addr,
	})
}

// DeleteAddress handles DELETE /api/v1/ipam/addresses/:id
func (h *IpamHandler) DeleteAddress(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteAddress(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to release IP address",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "IP address released successfully",
	})
}

// PingIP handles POST /api/v1/ipam/addresses/ping
func (h *IpamHandler) PingIP(c *gin.Context) {
	var body struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "IP is required"})
		return
	}

	reachable, latency, err := h.service.PingSingleIP(c.Request.Context(), body.IP)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"reachable": reachable,
		"latencyMs": latency,
	})
}

// GetSummaryStats handles GET /api/v1/ipam/stats
func (h *IpamHandler) GetSummaryStats(c *gin.Context) {
	stats, err := h.service.GetSummaryStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve summary stats",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
