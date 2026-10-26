package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// Automatically reconcile any stuck 'running' tasks against actual disk files or timeouts
	h.backupService.ReconcileHistory(c.Request.Context())

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

	entry, _ := h.backupRepo.GetHistoryEntry(c.Request.Context(), id)
	if entry != nil && entry.Filename != "" {
		// Clean up physical file on disk if it exists
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
		possiblePaths = append(possiblePaths,
			filepath.Join("/app/backups", entry.Filename),
			filepath.Join("/opt/backups", entry.Filename),
			filepath.Join("/app/backups/database", entry.Filename),
			filepath.Join("/opt/backups/database", entry.Filename),
			filepath.Join("backups", entry.Filename),
		)
		for _, p := range possiblePaths {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				_ = os.Remove(p)
				break
			}
		}
	}

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

// -------------------------------------------------------------
// Docker Container Backup & Restore Handlers
// -------------------------------------------------------------

func (h *BackupHandler) ListContainerBackups(c *gin.Context) {
	list, err := h.backupService.ListContainerBackups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *BackupHandler) RunContainerBackup(c *gin.Context) {
	var req domain.BackupContainerRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: connectionId and containerId are required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	meta, err := h.backupService.BackupContainer(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Container '%s' backup created successfully in backup/container-docker.", meta.ContainerName),
		"data":    meta,
	})
}

func (h *BackupHandler) RunContainerBackupStream(c *gin.Context) {
	var req domain.BackupContainerRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: connectionId and containerId are required"})
		return
	}

	taskID, err := h.backupService.StartContainerBackupAsync(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.streamTaskLogs(c, taskID)
}

func (h *BackupHandler) InspectContainerBackup(c *gin.Context) {
	filename := c.Param("filename")
	meta, err := h.backupService.InspectContainerBackup(c.Request.Context(), filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": meta})
}

func (h *BackupHandler) RestoreContainer(c *gin.Context) {
	var req domain.RestoreContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: filename and targetConnectionId are required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	details, err := h.backupService.RestoreContainer(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Container '%s' successfully restored to target Docker host!", details.Name),
		"data":    details,
	})
}

func (h *BackupHandler) RestoreContainerStream(c *gin.Context) {
	var req domain.RestoreContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: filename and targetConnectionId are required"})
		return
	}

	taskID, err := h.backupService.StartContainerRestoreAsync(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.streamTaskLogs(c, taskID)
}

func (h *BackupHandler) streamTaskLogs(c *gin.Context, taskID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	sendMsg := func(step string, status string, message string, timestamp string) bool {
		payload := map[string]interface{}{
			"taskId":    taskID,
			"step":      step,
			"status":    status, // "info", "progress", "success", "error"
			"message":   message,
			"timestamp": timestamp,
		}
		data, _ := json.Marshal(payload)
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	logChan, unsubscribe, snapshot, ok := h.backupService.SubscribeTask(taskID)
	if !ok {
		sendMsg("error", "error", "Task not found or already completed", time.Now().Format("15:04:05"))
		return
	}
	defer unsubscribe()

	for _, l := range snapshot {
		if !sendMsg(l.Step, l.Status, l.Message, l.Timestamp) {
			return
		}
	}

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return // Client disconnected or minimized. Task safely keeps running on the server.
		case entry, open := <-logChan:
			if !open {
				task := h.backupService.GetActiveTask(taskID)
				if task != nil && task.Status == "success" {
					sendMsg("complete", "success", "Process execution completed successfully!", time.Now().Format("15:04:05"))
				} else if task != nil && task.Status == "failed" {
					sendMsg("error", "error", task.Error, time.Now().Format("15:04:05"))
				}
				return
			}
			if !sendMsg(entry.Step, entry.Status, entry.Message, entry.Timestamp) {
				return
			}
		}
	}
}

func (h *BackupHandler) GetHistoryLogs(c *gin.Context) {
	id := c.Param("id")

	// 1. Check if active in memory
	task := h.backupService.GetActiveTask(id)
	if task != nil {
		snapshot := task.GetLogsSnapshot()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"id":      id,
				"running": task.Status == "running",
				"status":  task.Status,
				"logs":    snapshot,
			},
		})
		return
	}

	// 2. Not in active memory, retrieve from database
	entry, err := h.backupRepo.GetHistoryEntry(c.Request.Context(), id)
	if err != nil || entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "History entry not found"})
		return
	}

	var parsedLogs []services.ActiveBackupTaskLog
	rawLog := ""
	if entry.ErrorMessage != nil {
		rawLog = *entry.ErrorMessage
	}

	lines := strings.Split(rawLog, "\n")
	re := regexp.MustCompile(`^\[(.*?)\]\s+\[(.*?)\]\s+(.*)$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if len(m) == 4 {
			step := strings.ToLower(m[2])
			status := "info"
			if step == "error" || strings.Contains(strings.ToLower(m[3]), "fail") {
				status = "error"
			} else if step == "complete" || step == "success" {
				status = "success"
			} else if step == "commit" || step == "stream" || step == "extract" || step == "load" || step == "deploy" || step == "quiesce" || step == "package" {
				status = "progress"
			}
			parsedLogs = append(parsedLogs, services.ActiveBackupTaskLog{
				Timestamp: m[1],
				Step:      step,
				Status:    status,
				Message:   m[3],
			})
		} else {
			parsedLogs = append(parsedLogs, services.ActiveBackupTaskLog{
				Timestamp: entry.StartedAt.Format("15:04:05"),
				Step:      "log",
				Status:    entry.Status,
				Message:   line,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":      id,
			"running": entry.Status == "running",
			"status":  entry.Status,
			"rawLog":  rawLog,
			"logs":    parsedLogs,
		},
	})
}

func (h *BackupHandler) DownloadContainerBackup(c *gin.Context) {
	filename := c.Param("filename")
	filePath, err := h.backupService.FindContainerBackupFile(filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	c.Header("Content-Type", "application/gzip")
	c.File(filePath)
}

func (h *BackupHandler) DeleteContainerBackup(c *gin.Context) {
	filename := c.Param("filename")
	if err := h.backupService.DeleteContainerBackup(c.Request.Context(), filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container backup deleted successfully."})
}
