package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsHandler struct {
	configRepo    *repository.ConfigRepository
	userRepo      *repository.UserRepository
	systemService *services.SystemService
}

func NewSettingsHandler(
	configRepo *repository.ConfigRepository,
	userRepo *repository.UserRepository,
	systemService *services.SystemService,
) *SettingsHandler {
	return &SettingsHandler{
		configRepo:    configRepo,
		userRepo:      userRepo,
		systemService: systemService,
	}
}

// System Stats
func (h *SettingsHandler) GetSystemStats(c *gin.Context) {
	stats := h.systemService.GetStats(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// Activity Logs
func (h *SettingsHandler) ListActivityLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, count, err := h.userRepo.ListActivityLogs(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"logs": logs, "total": count}})
}

// Users CRUD
func (h *SettingsHandler) ListUsers(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
}

func (h *SettingsHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}
	if req.Role == "" {
		req.Role = "operator"
	}

	hash, err := config.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	user, err := h.userRepo.Create(c.Request.Context(), req.Username, hash, req.Role, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User created.", "data": user})
}

func (h *SettingsHandler) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	currentUserID := c.GetInt("userId")
	if id == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Cannot delete your own account"})
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User deleted."})
}

// Grafana Configs
func (h *SettingsHandler) ListGrafana(c *gin.Context) {
	list, err := h.configRepo.ListGrafana(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SettingsHandler) SaveGrafana(c *gin.Context) {
	var cfg domain.GrafanaConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("graf-%s", uuid.New().String()[:8])
	}
	if err := h.configRepo.SaveGrafana(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Grafana configuration saved.", "data": cfg})
}

func (h *SettingsHandler) SetActiveGrafana(c *gin.Context) {
	id := c.Param("id")
	if err := h.configRepo.SetActiveGrafana(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Active Grafana server set."})
}

func (h *SettingsHandler) DeleteGrafana(c *gin.Context) {
	id := c.Param("id")
	if err := h.configRepo.DeleteGrafana(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Grafana config deleted."})
}

// Prometheus Configs
func (h *SettingsHandler) ListPrometheus(c *gin.Context) {
	list, err := h.configRepo.ListPrometheus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SettingsHandler) SavePrometheus(c *gin.Context) {
	var cfg domain.PrometheusConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("prom-%s", uuid.New().String()[:8])
	}
	if err := h.configRepo.SavePrometheus(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Prometheus configuration saved.", "data": cfg})
}

func (h *SettingsHandler) SetActivePrometheus(c *gin.Context) {
	id := c.Param("id")
	if err := h.configRepo.SetActivePrometheus(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Active Prometheus server set."})
}

func (h *SettingsHandler) DeletePrometheus(c *gin.Context) {
	id := c.Param("id")
	if err := h.configRepo.DeletePrometheus(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Prometheus config deleted."})
}

// Database Connection Reconfiguration
func (h *SettingsHandler) GetDatabaseConfig(c *gin.Context) {
	cfg := config.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"host":     cfg.DB.Host,
			"port":     cfg.DB.Port,
			"user":     cfg.DB.User,
			"database": cfg.DB.Database,
			"ssl":      cfg.DB.SSL,
		},
	})
}

func (h *SettingsHandler) UpdateDatabaseConfig(c *gin.Context) {
	var req config.DBConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	appCfg := config.GetConfig()
	if err := appCfg.UpdateDBConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Reconnect database pool
	reconnCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.InitDatabase(reconnCtx, appCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Saved config, but database connection failed: %v", err),
		})
		return
	}

	logger.Info("Settings", fmt.Sprintf("PostgreSQL database switched to %s:%d/%s", req.Host, req.Port, req.Database))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Database connected and synchronized successfully."})
}

func (h *SettingsHandler) TestDatabaseConnection(c *gin.Context) {
	var req config.DBConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	connCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(req.ConnString())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": fmt.Sprintf("Invalid connection string parameters: %v", err)})
		return
	}

	pool, err := pgxpool.NewWithConfig(connCtx, poolConfig)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": fmt.Sprintf("Failed to initialize connection pool: %v", err)})
		return
	}
	defer pool.Close()

	if err := pool.Ping(connCtx); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": fmt.Sprintf("Database ping failed: %v", err)})
		return
	}

	var version string
	_ = pool.QueryRow(connCtx, "SELECT version()").Scan(&version)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Connection successful! Connected to PostgreSQL at %s:%d/%s", req.Host, req.Port, req.Database),
		"version": version,
	})
}

// Monitoring Views (Slide Shows / Kiosk)
func (h *SettingsHandler) ListMonitoringViews(c *gin.Context) {
	list, err := h.configRepo.ListMonitoringViews(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SettingsHandler) SaveMonitoringView(c *gin.Context) {
	var v domain.MonitoringView
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}
	if v.ID == "" {
		v.ID = fmt.Sprintf("view-%s", uuid.New().String()[:8])
	}
	if v.Interval <= 0 {
		v.Interval = 15
	}
	if v.Mode == "" {
		v.Mode = "slideshow"
	}
	if err := h.configRepo.SaveMonitoringView(c.Request.Context(), v); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Slide Show saved.", "data": v})
}

func (h *SettingsHandler) DeleteMonitoringView(c *gin.Context) {
	id := c.Param("id")
	if err := h.configRepo.DeleteMonitoringView(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Slide Show deleted."})
}

