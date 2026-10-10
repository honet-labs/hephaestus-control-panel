package handlers

import (
	"fmt"
	"net/http"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type ServerInventoryHandler struct {
	service *services.ServerInventoryService
}

func NewServerInventoryHandler(service *services.ServerInventoryService) *ServerInventoryHandler {
	return &ServerInventoryHandler{service: service}
}

func (h *ServerInventoryHandler) List(c *gin.Context) {
	userID, userRole := getUserContext(c)
	search := c.Query("search")
	status := c.Query("status")
	osType := c.Query("osType")

	items, err := h.service.List(c.Request.Context(), userID, userRole, search, status, osType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list server inventory: " + err.Error()})
		return
	}

	if items == nil {
		items = []domain.ServerInventoryItem{}
	}

	c.JSON(http.StatusOK, items)
}

func (h *ServerInventoryHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server inventory stats: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *ServerInventoryHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	item, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve server inventory: " + err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server inventory item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *ServerInventoryHandler) Create(c *gin.Context) {
	userID, _ := getUserContext(c)

	var req domain.CreateServerInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	item, err := h.service.Create(c.Request.Context(), req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create server inventory item: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *ServerInventoryHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateServerInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	item, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server inventory item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *ServerInventoryHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete server inventory item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Server inventory item deleted successfully", "id": id})
}

func (h *ServerInventoryHandler) SyncFromRemoteHost(c *gin.Context) {
	userID, _ := getUserContext(c)
	remoteHostID := c.Param("remoteHostId")
	if remoteHostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Remote host ID is required"})
		return
	}

	item, err := h.service.SyncFromRemoteHost(c.Request.Context(), remoteHostID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to probe remote server specifications: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Server specifications synchronized successfully from remote host",
		"item":    item,
	})
}

func (h *ServerInventoryHandler) SyncAllRemoteHosts(c *gin.Context) {
	userID, userRole := getUserContext(c)

	result, err := h.service.SyncAllRemoteHosts(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync all remote servers: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *ServerInventoryHandler) DownloadTemplate(c *gin.Context) {
	templateBytes := h.service.GenerateCSVTemplate()

	c.Header("Content-Disposition", "attachment; filename=server_inventory_template.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", templateBytes)
}

func (h *ServerInventoryHandler) ExportCSV(c *gin.Context) {
	userID, userRole := getUserContext(c)

	exportBytes, err := h.service.ExportCSV(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed exporting server inventory CSV: " + err.Error()})
		return
	}

	filename := fmt.Sprintf("server_inventory_export_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", exportBytes)
}

func (h *ServerInventoryHandler) ImportCSV(c *gin.Context) {
	userID, _ := getUserContext(c)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file upload is required (form field: 'file')"})
		return
	}
	defer file.Close()

	if header.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file exceeds maximum allowed size of 10MB"})
		return
	}

	result, err := h.service.ImportCSV(c.Request.Context(), file, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to import CSV: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
