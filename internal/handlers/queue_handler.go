package handlers

import (
	"net/http"

	"go-hephaestus/internal/queue"

	"github.com/gin-gonic/gin"
)

type QueueHandler struct{}

func NewQueueHandler() *QueueHandler {
	return &QueueHandler{}
}

func (h *QueueHandler) ListJobs(c *gin.Context) {
	wp := queue.GetWorkerPool()
	jobs := wp.ListJobs()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    jobs,
	})
}

func (h *QueueHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	wp := queue.GetWorkerPool()
	job, exists := wp.GetJob(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    job,
	})
}

func (h *QueueHandler) TriggerJob(c *gin.Context) {
	var req struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "type is required"})
		return
	}

	wp := queue.GetWorkerPool()
	job, err := wp.Enqueue(req.Type, map[string]interface{}{}, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Job enqueued successfully.",
		"data":    job,
	})
}

func (h *QueueHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	wp := queue.GetWorkerPool()
	if err := wp.CancelJob(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Job cancellation requested."})
}
