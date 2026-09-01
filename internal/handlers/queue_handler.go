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

func (h *QueueHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	wp := queue.GetWorkerPool()
	if err := wp.CancelJob(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Job cancellation requested."})
}
