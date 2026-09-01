package handlers

import (
	"net/http"

	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type SnmpHandler struct {
	snmpRepo    *repository.SnmpRepository
	snmpService *services.SnmpService
}

func NewSnmpHandler(snmpRepo *repository.SnmpRepository, snmpService *services.SnmpService) *SnmpHandler {
	return &SnmpHandler{
		snmpRepo:    snmpRepo,
		snmpService: snmpService,
	}
}

func (h *SnmpHandler) Query(c *gin.Context) {
	var req struct {
		Host      string `json:"host" binding:"required"`
		Port      uint16 `json:"port"`
		Version   string `json:"version"`
		Community string `json:"community"`
		OID       string `json:"oid" binding:"required"`
		Operation string `json:"operation"` // "get" or "walk"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid query parameters"})
		return
	}

	if req.Port == 0 {
		req.Port = 161
	}
	if req.Community == "" {
		req.Community = "public"
	}
	if req.Operation == "" {
		req.Operation = "get"
	}

	results, err := h.snmpService.Query(req.Host, req.Port, req.Version, req.Community, req.OID, req.Operation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func (h *SnmpHandler) ListMibs(c *gin.Context) {
	list, err := h.snmpRepo.ListImportedMibs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SnmpHandler) ImportMib(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	var content string
	var mibName string

	if err == nil {
		defer file.Close()
		data, _ := io.ReadAll(file)
		content = string(data)
		mibName = header.Filename
	} else {
		var req struct {
			Name    string `json:"name" binding:"required"`
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "MIB file or name/content is required"})
			return
		}
		content = req.Content
		mibName = req.Name
	}

	mib, err := h.snmpService.ImportMibText(c.Request.Context(), mibName, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "MIB imported successfully.", "data": mib})
}

func (h *SnmpHandler) DeleteMib(c *gin.Context) {
	name := c.Param("name")
	if err := h.snmpRepo.DeleteImportedMib(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "MIB deleted successfully."})
}

func (h *SnmpHandler) TranslateOID(c *gin.Context) {
	oid := c.Query("oid")
	name, info := h.snmpRepo.TranslateOid(c.Request.Context(), oid)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"oid":  oid,
			"name": name,
			"info": info,
		},
	})
}
