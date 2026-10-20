package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type DataPrepperHandler struct {
	dpService *services.DataPrepperService
}

func NewDataPrepperHandler(dpService *services.DataPrepperService) *DataPrepperHandler {
	return &DataPrepperHandler{dpService: dpService}
}

func (h *DataPrepperHandler) ListPipelines(c *gin.Context) {
	instanceID := c.Query("instanceId")
	list, err := h.dpService.ListPipelines(c.Request.Context(), instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *DataPrepperHandler) GetPipelineFile(c *gin.Context) {
	instanceID := c.Query("instanceId")
	file := c.Query("file")
	if file == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file parameter is required"})
		return
	}

	content, err := h.dpService.GetPipelineFile(c.Request.Context(), instanceID, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"file": file, "content": content}})
}

func (h *DataPrepperHandler) SavePipelineFile(c *gin.Context) {
	var req struct {
		InstanceID    string `json:"instanceId"`
		File          string `json:"file" binding:"required"`
		Content       string `json:"content"`
		ChangeSummary string `json:"changeSummary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	saveRes, err := h.dpService.SavePipelineFile(c.Request.Context(), req.InstanceID, req.File, req.Content, usernameStr, req.ChangeSummary)
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

func (h *DataPrepperHandler) ListHistory(c *gin.Context) {
	instanceID := c.Query("instanceId")
	file := c.Query("file")
	if file == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file parameter is required"})
		return
	}

	history, err := h.dpService.ListHistory(c.Request.Context(), instanceID, file, 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": history})
}

func (h *DataPrepperHandler) DeletePipelineFile(c *gin.Context) {
	instanceID := c.Query("instanceId")
	file := c.Query("file")
	if file == "" {
		var req struct {
			InstanceID string `json:"instanceId"`
			File       string `json:"file"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			if instanceID == "" {
				instanceID = req.InstanceID
			}
			file = req.File
		}
	}

	if file == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file parameter is required"})
		return
	}

	if err := h.dpService.DeletePipelineFile(c.Request.Context(), instanceID, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Pipeline '%s' deleted and Data Prepper service restarted.", file)})
}

func (h *DataPrepperHandler) RenamePipelineFile(c *gin.Context) {
	var req struct {
		InstanceID string `json:"instanceId"`
		OldFile    string `json:"oldFile" binding:"required"`
		NewFile    string `json:"newFile" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	cleanNew := filepath.Base(strings.TrimSpace(req.NewFile))
	if !strings.HasSuffix(cleanNew, ".yaml") && !strings.HasSuffix(cleanNew, ".yml") {
		cleanNew += ".yaml"
	}

	if err := h.dpService.RenamePipelineFile(c.Request.Context(), req.InstanceID, req.OldFile, cleanNew); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"newFile": cleanNew,
		"message": fmt.Sprintf("Pipeline '%s' renamed to '%s' successfully.", req.OldFile, cleanNew),
	})
}

func (h *DataPrepperHandler) ValidateYAML(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
		Yaml    string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "content is required"})
		return
	}

	content := req.Content
	if content == "" && req.Yaml != "" {
		content = req.Yaml
	}

	valid, msg, line := h.dpService.ValidateYAML(content)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   valid,
		"message": msg,
		"error":   msg,
		"line":    line,
	})
}
