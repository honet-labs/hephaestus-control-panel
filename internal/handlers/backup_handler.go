package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

func (h *BackupHandler) TestDBConfig(c *gin.Context) {
	var cfg domain.BackupDbConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: " + err.Error()})
		return
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

	// If testing an existing configuration without re-entering passwords
	if cfg.ID != "" && (cfg.Password == "" || cfg.Password == "********") {
		existing, err := h.backupRepo.GetRawDBConfig(c.Request.Context(), cfg.ID)
		if err == nil && existing != nil {
			cfg.Password = existing.Password
			if cfg.SSHPassword == nil || *cfg.SSHPassword == "" || *cfg.SSHPassword == "********" {
				cfg.SSHPassword = existing.SSHPassword
			}
			if cfg.SSHKey == nil || *cfg.SSHKey == "" || *cfg.SSHKey == "********" {
				cfg.SSHKey = existing.SSHKey
			}
		}
	}

	msg, err := h.backupService.TestDBConfig(c.Request.Context(), &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Connection test failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": msg})
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

func (h *BackupHandler) TestDestination(c *gin.Context) {
	var dest domain.BackupDestination
	if err := c.ShouldBindJSON(&dest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: " + err.Error()})
		return
	}

	if dest.Name == "" {
		dest.Name = "Test Destination"
	}

	if dest.ID != "" && dest.Config != nil {
		existing, err := h.backupRepo.GetRawDestination(c.Request.Context(), dest.ID)
		if err == nil && existing != nil && existing.Config != nil {
			if sec, ok := dest.Config["secretAccessKey"].(string); ok && (sec == "" || sec == "********") {
				dest.Config["secretAccessKey"] = existing.Config["secretAccessKey"]
			}
			if pwd, ok := dest.Config["password"].(string); ok && (pwd == "" || pwd == "********") {
				dest.Config["password"] = existing.Config["password"]
			}
			if key, ok := dest.Config["sshKey"].(string); ok && (key == "" || key == "********") {
				dest.Config["sshKey"] = existing.Config["sshKey"]
			}
		}
	}

	if err := h.backupService.TestDestination(c.Request.Context(), &dest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Connection test failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Storage connection successful! Test file successfully uploaded."})
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

func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	id := c.Param("id")
	entry, err := h.backupRepo.GetHistoryEntry(c.Request.Context(), id)
	if err != nil || entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Backup record not found"})
		return
	}

	if entry.Status != "success" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Backup job did not complete successfully (status: %s)", entry.Status)})
		return
	}

	// Try to locate file on disk
	var possiblePaths []string

	if entry.DestinationID != nil && *entry.DestinationID != "" {
		dest, err := h.backupRepo.GetRawDestination(c.Request.Context(), *entry.DestinationID)
		if err == nil && dest != nil && dest.Config != nil {
			if cfgPath, ok := dest.Config["path"].(string); ok && cfgPath != "" {
				resolved := services.ResolveLocalBackupPath(cfgPath)
				possiblePaths = append(possiblePaths,
					filepath.Join(resolved, entry.Filename),
					filepath.Join(cfgPath, entry.Filename),
				)
			}
		}
	}

	// Fallback standard locations
	possiblePaths = append(possiblePaths,
		filepath.Join("/app/backups", entry.Filename),
		filepath.Join("/opt/backups", entry.Filename),
		filepath.Join("/app/backups/database", entry.Filename),
		filepath.Join("/opt/backups/database", entry.Filename),
		filepath.Join("backups", entry.Filename),
	)

	var foundPath string
	for _, p := range possiblePaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			foundPath = p
			break
		}
	}

	// If still not found, search recursively inside /app/backups or /opt/backups
	if foundPath == "" {
		for _, rootDir := range []string{"/app/backups", "/opt/backups"} {
			if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() {
				_ = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
					if err == nil && !d.IsDir() && d.Name() == entry.Filename {
						foundPath = path
						return filepath.SkipAll
					}
					return nil
				})
			}
			if foundPath != "" {
				break
			}
		}
	}

	if foundPath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Backup file '%s' was not found on local storage. It may have been saved to a remote cloud destination or lost due to container recreation without a mounted volume.", entry.Filename),
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", entry.Filename))
	c.Header("Content-Type", "application/gzip")
	c.File(foundPath)
}
