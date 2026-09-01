package handlers

import (
	"net/http"
	"strconv"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/scheduler"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BackupHandler struct {
	backupRepo    *repository.BackupRepository
	backupService *services.BackupService
	scheduler     *scheduler.CronScheduler
}

func NewBackupHandler(
	backupRepo *repository.BackupRepository,
	backupService *services.BackupService,
	scheduler *scheduler.CronScheduler,
) *BackupHandler {
	return &BackupHandler{
		backupRepo:    backupRepo,
		backupService: backupService,
		scheduler:     scheduler,
	}
}

// Database Configs
func (h *BackupHandler) ListDBConfigs(c *gin.Context) {
	list, err := h.backupRepo.ListDBConfigs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *BackupHandler) SaveDBConfig(c *gin.Context) {
	var cfg domain.BackupDbConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("bdc-%s", uuid.New().String()[:8])
	}
	if cfg.Port <= 0 {
		switch cfg.DBType {
		case "postgresql":
			cfg.Port = 5432
		case "mysql", "mariadb":
			cfg.Port = 3306
		case "sqlserver":
			cfg.Port = 1433
		default:
			cfg.Port = 5432
		}
	}

	if err := h.backupRepo.SaveDBConfig(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Database config saved.", "data": cfg})
}

func (h *BackupHandler) DeleteDBConfig(c *gin.Context) {
	id := c.Param("id")
	if err := h.backupRepo.DeleteDBConfig(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Database config deleted."})
}

// Destinations
func (h *BackupHandler) ListDestinations(c *gin.Context) {
	list, err := h.backupRepo.ListDestinations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *BackupHandler) SaveDestination(c *gin.Context) {
	var dest domain.BackupDestination
	if err := c.ShouldBindJSON(&dest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if dest.ID == "" {
		dest.ID = fmt.Sprintf("bdest-%s", uuid.New().String()[:8])
	}

	if err := h.backupRepo.SaveDestination(c.Request.Context(), dest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Destination saved.", "data": dest})
}

func (h *BackupHandler) DeleteDestination(c *gin.Context) {
	id := c.Param("id")
	if err := h.backupRepo.DeleteDestination(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Destination deleted."})
}

// Schedules
func (h *BackupHandler) ListSchedules(c *gin.Context) {
	list, err := h.backupRepo.ListSchedules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *BackupHandler) SaveSchedule(c *gin.Context) {
	var sched domain.BackupSchedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if sched.ID == "" {
		sched.ID = fmt.Sprintf("bsch-%s", uuid.New().String()[:8])
	}

	if err := h.backupRepo.SaveSchedule(c.Request.Context(), sched); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.scheduler.ReloadBackupSchedules(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Schedule saved.", "data": sched})
}

func (h *BackupHandler) DeleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.backupRepo.DeleteSchedule(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	h.scheduler.ReloadBackupSchedules(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Schedule deleted."})
}

// Trigger Manual Backup
func (h *BackupHandler) RunBackup(c *gin.Context) {
	var req struct {
		DBConfigID    string `json:"dbConfigId" binding:"required"`
		DestinationID string `json:"destinationId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "dbConfigId and destinationId are required"})
		return
	}

	jobID, err := h.backupService.TriggerBackup(c.Request.Context(), req.DBConfigID, req.DestinationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Backup job enqueued successfully.",
		"data": gin.H{
			"jobId": jobID,
		},
	})
}

// History
func (h *BackupHandler) ListHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	history, count, err := h.backupRepo.ListHistory(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"history": history,
			"total":   count,
		},
	})
}

func (h *BackupHandler) DeleteHistory(c *gin.Context) {
	id := c.Param("id")
	if err := h.backupRepo.DeleteHistory(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "History entry deleted."})
}
