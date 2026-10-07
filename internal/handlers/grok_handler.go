package handlers

import (
	"net/http"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

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
