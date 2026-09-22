package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type StatusPageHandler struct {
	statusPageService *services.StatusPageService
	authService       *services.AuthService
}

func NewStatusPageHandler(statusPageService *services.StatusPageService, authService *services.AuthService) *StatusPageHandler {
	return &StatusPageHandler{
		statusPageService: statusPageService,
		authService:       authService,
	}
}

// -----------------------------------------------------------------------------
// Public / Standalone Endpoints
// -----------------------------------------------------------------------------

// GetPublicReport retrieves a published status page report by slug.
// If the page is marked as private, valid authentication is verified.
func (h *StatusPageHandler) GetPublicReport(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Slug parameter is required",
		})
		return
	}

	report, err := h.statusPageService.GetLiveReport(c.Request.Context(), slug, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Status page not found",
			"message": err.Error(),
		})
		return
	}

	// If page is private, check authentication
	if !report.IsPublic {
		token := ""
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if qToken := c.Query("token"); qToken != "" {
			token = qToken
		} else if cToken, err := c.Cookie("hephaestus_session"); err == nil {
			token = cToken
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success":      false,
				"requiresAuth": true,
				"title":        report.Title,
				"slug":         report.Slug,
				"error":        "Authentication required",
				"message":      "This status page is private. Please sign in to view status.",
			})
			return
		}

		_, err := h.authService.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success":      false,
				"requiresAuth": true,
				"title":        report.Title,
				"slug":         report.Slug,
				"error":        "Invalid or expired session",
				"message":      "Please log in with a valid account to access this status page.",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// -----------------------------------------------------------------------------
// Protected Admin Endpoints
// -----------------------------------------------------------------------------

// ListPages returns all configured status pages
func (h *StatusPageHandler) ListPages(c *gin.Context) {
	pages, err := h.statusPageService.ListPages(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pages,
	})
}

// GetPage returns a single status page with its groups, items, and incidents
func (h *StatusPageHandler) GetPage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID is required"})
		return
	}

	page, err := h.statusPageService.GetPageByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Status page not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    page,
	})
}

// CreatePage creates a new status page
func (h *StatusPageHandler) CreatePage(c *gin.Context) {
	var req domain.StatusPage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("sp-%d", time.Now().UnixNano())
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Title is required"})
		return
	}
	if req.Slug == "" {
		req.Slug = strings.ToLower(strings.ReplaceAll(req.Title, " ", "-"))
	}

	// Capture user id if available
	if userObj, exists := c.Get("user"); exists {
		if u, ok := userObj.(*domain.User); ok && u != nil {
			req.UserID = &u.ID
		}
	}

	if err := h.statusPageService.CreatePage(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
		"message": "Status page created successfully",
	})
}

// UpdatePage updates status page general properties
func (h *StatusPageHandler) UpdatePage(c *gin.Context) {
	id := c.Param("id")
	var req domain.StatusPage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	req.ID = id

	if err := h.statusPageService.UpdatePage(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
		"message": "Status page updated successfully",
	})
}

// DeletePage removes a status page
func (h *StatusPageHandler) DeletePage(c *gin.Context) {
	id := c.Param("id")
	if err := h.statusPageService.DeletePage(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Status page deleted successfully",
	})
}

// SaveGroup adds or updates a group in a status page
func (h *StatusPageHandler) SaveGroup(c *gin.Context) {
	pageID := c.Param("id")
	var req domain.StatusPageGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	req.PageID = pageID
	if req.ID == "" {
		req.ID = fmt.Sprintf("grp-%d", time.Now().UnixNano())
	}

	if err := h.statusPageService.SaveGroup(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
		"message": "Group saved successfully",
	})
}

// DeleteGroup removes a group
func (h *StatusPageHandler) DeleteGroup(c *gin.Context) {
	groupID := c.Param("groupId")
	if err := h.statusPageService.DeleteGroup(c.Request.Context(), groupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group deleted successfully",
	})
}

// SaveItem adds or updates a monitored item
func (h *StatusPageHandler) SaveItem(c *gin.Context) {
	pageID := c.Param("id")
	var req domain.StatusPageItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	req.PageID = pageID
	if req.ID == "" {
		req.ID = fmt.Sprintf("item-%d", time.Now().UnixNano())
	}

	if err := h.statusPageService.SaveItem(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
		"message": "Monitored item saved successfully",
	})
}

// DeleteItem removes a monitored item
func (h *StatusPageHandler) DeleteItem(c *gin.Context) {
	itemID := c.Param("itemId")
	if err := h.statusPageService.DeleteItem(c.Request.Context(), itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Monitored item deleted successfully",
	})
}

// SaveIncident creates or updates an incident / maintenance announcement
func (h *StatusPageHandler) SaveIncident(c *gin.Context) {
	pageID := c.Param("id")
	var req domain.StatusPageIncident
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	req.PageID = pageID
	if req.ID == "" {
		req.ID = fmt.Sprintf("inc-%d", time.Now().UnixNano())
	}

	if err := h.statusPageService.SaveIncident(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
		"message": "Incident saved successfully",
	})
}

// DeleteIncident removes an incident
func (h *StatusPageHandler) DeleteIncident(c *gin.Context) {
	incidentID := c.Param("incidentId")
	if err := h.statusPageService.DeleteIncident(c.Request.Context(), incidentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Incident deleted successfully",
	})
}

// GetLiveReportAdmin evaluates live status for admin preview
func (h *StatusPageHandler) GetLiveReportAdmin(c *gin.Context) {
	id := c.Param("id")
	report, err := h.statusPageService.GetLiveReport(c.Request.Context(), id, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// GetSourceOptions returns candidate data sources from Topology, OpenSearch, Prometheus, Grafana, and Remote Servers
func (h *StatusPageHandler) GetSourceOptions(c *gin.Context) {
	options, err := h.statusPageService.GetSourceOptions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    options,
	})
}
