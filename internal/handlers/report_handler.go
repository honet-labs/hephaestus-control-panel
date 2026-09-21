package handlers

import (
	"net/http"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type ReportHandler struct {
	reportService *services.ReportService
}

func NewReportHandler(reportService *services.ReportService) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
	}
}

// ListReports returns all saved visual reports
func (h *ReportHandler) ListReports(c *gin.Context) {
	reports, err := h.reportService.ListReports(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    reports,
	})
}

// GetReport returns single visual report with its widgets
func (h *ReportHandler) GetReport(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Report ID is required",
		})
		return
	}

	report, err := h.reportService.GetReportByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Report not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// CreateReport handles creation of a new report
func (h *ReportHandler) CreateReport(c *gin.Context) {
	var rep domain.VisualReport
	if err := c.ShouldBindJSON(&rep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if rep.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Report name is required",
		})
		return
	}

	if userIDVal, exists := c.Get("userId"); exists {
		if uid, ok := userIDVal.(int); ok {
			rep.UserID = &uid
		}
	}

	if err := h.reportService.CreateReport(c.Request.Context(), &rep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rep,
		"message": "Report created successfully",
	})
}

// UpdateReport updates report metadata and header configuration
func (h *ReportHandler) UpdateReport(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Report ID is required",
		})
		return
	}

	var rep domain.VisualReport
	if err := c.ShouldBindJSON(&rep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	rep.ID = id
	if err := h.reportService.UpdateReport(c.Request.Context(), &rep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rep,
		"message": "Report updated successfully",
	})
}

// DeleteReport removes a report and cascades its widgets
func (h *ReportHandler) DeleteReport(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Report ID is required",
		})
		return
	}

	if err := h.reportService.DeleteReport(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Report deleted successfully",
	})
}

// CreateWidget adds a new chart/table widget to a report
func (h *ReportHandler) CreateWidget(c *gin.Context) {
	reportID := c.Param("id")

	var w domain.VisualReportWidget
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if reportID == "" {
		reportID = w.ReportID
	}
	if reportID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Report ID is required",
		})
		return
	}

	w.ReportID = reportID
	if w.Title == "" {
		w.Title = "Metrics Trend Chart"
	}

	if err := h.reportService.CreateWidget(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    w,
		"message": "Widget added successfully",
	})
}

// UpdateWidget updates an existing widget
func (h *ReportHandler) UpdateWidget(c *gin.Context) {
	widgetID := c.Param("widgetId")
	if widgetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Widget ID is required",
		})
		return
	}

	var w domain.VisualReportWidget
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.ID = widgetID
	if err := h.reportService.UpdateWidget(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    w,
		"message": "Widget updated successfully",
	})
}

// DeleteWidget removes a widget
func (h *ReportHandler) DeleteWidget(c *gin.Context) {
	widgetID := c.Param("widgetId")
	if widgetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Widget ID is required",
		})
		return
	}

	if err := h.reportService.DeleteWidget(c.Request.Context(), widgetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Widget removed successfully",
	})
}

// QueryWidgetData executes live queries for Grafana or OpenSearch metrics
func (h *ReportHandler) QueryWidgetData(c *gin.Context) {
	var req domain.ReportQueryDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	res, err := h.reportService.QueryWidgetData(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    res,
	})
}
