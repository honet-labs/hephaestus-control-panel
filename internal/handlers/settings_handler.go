package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	shareRepo     *repository.ConnectionShareRepository
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
		shareRepo:     repository.NewConnectionShareRepository(),
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
		Username            string `json:"username" binding:"required"`
		Password            string `json:"password" binding:"required"`
		Role                string `json:"role"`
		ForcePasswordChange bool   `json:"forcePasswordChange"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: username and password are required"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Username cannot be empty"})
		return
	}
	if len(req.Password) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Password must be at least 10 characters"})
		return
	}
	if req.Role == "" {
		req.Role = "OPERATOR"
	}

	hash, err := config.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	user, err := h.userRepo.Create(c.Request.Context(), req.Username, hash, req.Role, req.ForcePasswordChange)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Username already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	currentUserID := c.GetInt("userId")
	adminUsername := c.GetString("username")
	details := fmt.Sprintf("Admin '%s' created user '%s' (role: %s)", adminUsername, user.Username, user.Role)
	_ = h.userRepo.LogActivity(c.Request.Context(), "Users", "Create", details, "success", &currentUserID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User created successfully.", "data": user})
}

func (h *SettingsHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	var req struct {
		Username            string `json:"username"`
		Password            string `json:"password"`
		Role                string `json:"role"`
		ForcePasswordChange *bool  `json:"forcePasswordChange"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Role = strings.TrimSpace(req.Role)

	existing, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found"})
		return
	}

	currentUserID := c.GetInt("userId")
	// If updating own account and is ADMIN, do not permit removing ADMIN role
	if id == currentUserID && (existing.IsAdmin() || domain.IsAdminRole(existing.Role)) {
		if req.Role != "" && !domain.IsAdminRole(req.Role) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "You cannot remove ADMIN privileges from your own account"})
			return
		}
	}

	var passwordHash string
	if req.Password != "" {
		if len(req.Password) < 10 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Password must be at least 10 characters"})
			return
		}
		hash, err := config.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to hash password"})
			return
		}
		passwordHash = hash
	}

	if err := h.userRepo.UpdateUser(c.Request.Context(), id, req.Username, passwordHash, req.Role, req.ForcePasswordChange); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Username is already taken by another account"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Invalidate active sessions if password was updated (except current user if editing self)
	if passwordHash != "" && id != currentUserID {
		_ = h.userRepo.DeleteUserSessions(c.Request.Context(), id)
	}

	adminUsername := c.GetString("username")
	details := fmt.Sprintf("Admin '%s' updated user ID %d ('%s')", adminUsername, id, existing.Username)
	_ = h.userRepo.LogActivity(c.Request.Context(), "Users", "Update", details, "success", &currentUserID)

	updatedUser, _ := h.userRepo.GetByID(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully.",
		"data":    updatedUser,
	})
}

func (h *SettingsHandler) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	currentUserID := c.GetInt("userId")
	if id == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Cannot delete your own account"})
		return
	}

	target, _ := h.userRepo.GetByID(c.Request.Context(), id)
	targetName := fmt.Sprintf("ID #%d", id)
	if target != nil {
		targetName = target.Username
	}

	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	adminUsername := c.GetString("username")
	details := fmt.Sprintf("Admin '%s' deleted user '%s'", adminUsername, targetName)
	_ = h.userRepo.LogActivity(c.Request.Context(), "Users", "Delete", details, "success", &currentUserID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User deleted."})
}

func (h *SettingsHandler) UpdateUserRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid role"})
		return
	}

	if err := h.userRepo.UpdateUserRole(c.Request.Context(), id, req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User role updated."})
}

// System Roles & Permissions Handlers
func (h *SettingsHandler) ListRoles(c *gin.Context) {
	roles, err := h.userRepo.ListRoles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": roles})
}

func (h *SettingsHandler) SaveRole(c *gin.Context) {
	var role domain.SystemRole
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid role payload"})
		return
	}

	if strings.TrimSpace(role.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Role name is required"})
		return
	}

	if err := h.userRepo.SaveRole(c.Request.Context(), &role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Role saved.", "data": role})
}

func (h *SettingsHandler) DeleteRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.userRepo.DeleteRole(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Role deleted."})
}

// Grafana Configs
func (h *SettingsHandler) ListGrafana(c *gin.Context) {
	userID, userRole := getUserContext(c)
	list, err := h.configRepo.ListGrafana(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SettingsHandler) SaveGrafana(c *gin.Context) {
	userID, userRole := getUserContext(c)
	var cfg domain.GrafanaConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Host = strings.TrimRight(strings.TrimSpace(cfg.Host), "/")

	if cfg.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Connection Name is required"})
		return
	}
	if cfg.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "API Endpoint URL is required"})
		return
	}
	if !strings.HasPrefix(cfg.Host, "http://") && !strings.HasPrefix(cfg.Host, "https://") {
		cfg.Host = "http://" + cfg.Host
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("graf-%s", uuid.New().String()[:8])
	}
	if err := h.configRepo.SaveGrafana(c.Request.Context(), cfg, userID, userRole); err != nil {
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
	userID, userRole := getUserContext(c)
	id := c.Param("id")
	if err := h.configRepo.DeleteGrafana(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Grafana config deleted."})
}

func (h *SettingsHandler) ListGrafanaShares(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}
	shares, err := h.shareRepo.ListShares(c.Request.Context(), "grafana_shares", "config_id", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *SettingsHandler) AddGrafanaShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body: userId is required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "grafana_configs", "grafana_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to share this configuration"})
		return
	}

	if err := h.shareRepo.AddShare(c.Request.Context(), "grafana_shares", "config_id", configID, req.UserID, req.Permission, currentUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access granted successfully"})
}

func (h *SettingsHandler) DeleteGrafanaShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil || configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Valid configuration ID and user ID are required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "grafana_configs", "grafana_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to modify shares for this configuration"})
		return
	}

	if err := h.shareRepo.DeleteShare(c.Request.Context(), "grafana_shares", "config_id", configID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access revoked successfully"})
}

func (h *SettingsHandler) UpdateGrafanaVisibility(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	var req struct {
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Visibility (private/public) is required"})
		return
	}

	hasAccess, isOwner, _, err := h.shareRepo.CheckAccess(c.Request.Context(), "grafana_configs", "grafana_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only the connection owner can change its visibility"})
		return
	}

	if err := h.shareRepo.UpdateVisibility(c.Request.Context(), "grafana_configs", configID, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Visibility updated to %s", req.Visibility)})
}

// TestGrafana verifies network connectivity, bearer token, and datasource UID against Grafana server
func (h *SettingsHandler) TestGrafana(c *gin.Context) {
	var req domain.GrafanaConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request payload"})
		return
	}

	host := strings.TrimRight(strings.TrimSpace(req.Host), "/")
	token := strings.TrimSpace(req.Token)

	if req.ID != "" && (host == "" || token == "" || token == "••••••••" || token == "********") {
		dbCfg, err := h.configRepo.GetGrafanaByID(c.Request.Context(), req.ID)
		if err == nil && dbCfg != nil {
			if host == "" {
				host = strings.TrimRight(strings.TrimSpace(dbCfg.Host), "/")
			}
			if token == "" || token == "••••••••" || token == "********" {
				token = dbCfg.Token
			}
			if req.DatasourceUID == "" {
				req.DatasourceUID = dbCfg.DatasourceUID
			}
		}
	}

	if host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "API Endpoint URL is required"})
		return
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 1. Test basic reachability via /api/health
	healthURL := host + "/api/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to reach Grafana server at %s: %v", host, err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Grafana returned error status %d from %s", resp.StatusCode, healthURL),
		})
		return
	}

	// 2. If token is provided, verify authentication via /api/org
	if token != "" {
		orgReq, err := http.NewRequestWithContext(c.Request.Context(), "GET", host+"/api/org", nil)
		if err == nil {
			orgReq.Header.Set("Authorization", "Bearer "+token)
			orgResp, err := client.Do(orgReq)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   fmt.Sprintf("Authentication query failed: %v", err),
				})
				return
			}
			defer orgResp.Body.Close()

			if orgResp.StatusCode == http.StatusUnauthorized || orgResp.StatusCode == http.StatusForbidden {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "Invalid Bearer Token: Unauthorized (401). Please verify your Service Account Token.",
				})
				return
			}
		}
	}

	// 3. If Datasource UID is provided, verify existence
	if req.DatasourceUID != "" && token != "" {
		dsReq, err := http.NewRequestWithContext(c.Request.Context(), "GET", host+"/api/datasources/uid/"+req.DatasourceUID, nil)
		if err == nil {
			dsReq.Header.Set("Authorization", "Bearer "+token)
			dsResp, err := client.Do(dsReq)
			if err == nil {
				defer dsResp.Body.Close()
				if dsResp.StatusCode == http.StatusNotFound {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"error":   fmt.Sprintf("Datasource UID '%s' not found in Grafana (404)", req.DatasourceUID),
					})
					return
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Grafana connection verified successfully!",
	})
}

// GetGrafanaDatasources fetches all datasources from Grafana API
func (h *SettingsHandler) GetGrafanaDatasources(c *gin.Context) {
	grafanaID := c.Query("id")
	var cfg *domain.GrafanaConfig
	var err error

	if grafanaID != "" {
		cfg, err = h.configRepo.GetGrafanaByID(c.Request.Context(), grafanaID)
	} else {
		cfg, err = h.configRepo.GetActiveGrafana(c.Request.Context())
	}

	if err != nil || cfg == nil || cfg.Host == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "No active Grafana configuration found. Please add a Grafana connection in Add Connections.",
			"data":    []interface{}{},
		})
		return
	}

	endpoint := strings.TrimRight(cfg.Host, "/") + "/api/datasources"
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "GET", endpoint, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "data": []interface{}{}})
		return
	}

	if cfg.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to query Grafana at %s: %v", cfg.Host, err),
			"data":    []interface{}{},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Grafana returned HTTP %d", resp.StatusCode),
			"data":    []interface{}{},
		})
		return
	}

	var rawList []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawList); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "Failed to parse Grafana datasources", "data": []interface{}{}})
		return
	}

	var result []gin.H
	for _, item := range rawList {
		uid, _ := item["uid"].(string)
		name, _ := item["name"].(string)
		dsType, _ := item["type"].(string)
		isDef, _ := item["isDefault"].(bool)
		result = append(result, gin.H{
			"uid":       uid,
			"name":      name,
			"type":      dsType,
			"isDefault": isDef,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"activeConfig": gin.H{
			"id":            cfg.ID,
			"name":          cfg.Name,
			"host":          cfg.Host,
			"datasourceUid": cfg.DatasourceUID,
		},
	})
}

// Prometheus Configs
func (h *SettingsHandler) ListPrometheus(c *gin.Context) {
	userID, userRole := getUserContext(c)
	list, err := h.configRepo.ListPrometheus(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	filterType := strings.ToLower(c.Query("type"))
	if filterType == "prometheus" {
		var filtered []domain.PrometheusConfig
		for _, item := range list {
			name := strings.ToLower(item.Name)
			path := strings.ToLower(item.Path)
			if !strings.Contains(name, "data prepper") && !strings.Contains(name, "dataprepper") && !strings.Contains(path, "pipeline") {
				filtered = append(filtered, item)
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": filtered})
		return
	} else if filterType == "dataprepper" || filterType == "data_prepper" {
		var filtered []domain.PrometheusConfig
		for _, item := range list {
			name := strings.ToLower(item.Name)
			path := strings.ToLower(item.Path)
			if strings.Contains(name, "data prepper") || strings.Contains(name, "dataprepper") || strings.Contains(path, "pipeline") {
				filtered = append(filtered, item)
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": filtered})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *SettingsHandler) SavePrometheus(c *gin.Context) {
	userID, userRole := getUserContext(c)
	var cfg domain.PrometheusConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Path = strings.TrimSpace(cfg.Path)

	if cfg.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Connection Name is required"})
		return
	}
	if cfg.Mode == "ssh" {
		if cfg.SSHHost == nil || strings.TrimSpace(*cfg.SSHHost) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH Host IP is required for SSH mode"})
			return
		}
	}
	if cfg.Path == "" {
		cfg.Path = "/etc/prometheus/prometheus.yml"
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("prom-%s", uuid.New().String()[:8])
	}
	if err := h.configRepo.SavePrometheus(c.Request.Context(), cfg, userID, userRole); err != nil {
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
	userID, userRole := getUserContext(c)
	id := c.Param("id")
	if err := h.configRepo.DeletePrometheus(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Prometheus config deleted."})
}

func (h *SettingsHandler) ListPrometheusShares(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}
	shares, err := h.shareRepo.ListShares(c.Request.Context(), "prometheus_shares", "config_id", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *SettingsHandler) AddPrometheusShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Configuration ID is required"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body: userId is required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "prometheus_configs", "prometheus_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to share this configuration"})
		return
	}

	if err := h.shareRepo.AddShare(c.Request.Context(), "prometheus_shares", "config_id", configID, req.UserID, req.Permission, currentUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access granted successfully"})
}

func (h *SettingsHandler) DeletePrometheusShare(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil || configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Valid configuration ID and user ID are required"})
		return
	}

	hasAccess, isOwner, perm, err := h.shareRepo.CheckAccess(c.Request.Context(), "prometheus_configs", "prometheus_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || (!isOwner && perm != "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "You do not have permission to modify shares for this configuration"})
		return
	}

	if err := h.shareRepo.DeleteShare(c.Request.Context(), "prometheus_shares", "config_id", configID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Share access revoked successfully"})
}

func (h *SettingsHandler) UpdatePrometheusVisibility(c *gin.Context) {
	currentUserID, currentUserRole := getUserContext(c)
	configID := c.Param("id")
	var req struct {
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Visibility (private/public) is required"})
		return
	}

	hasAccess, isOwner, _, err := h.shareRepo.CheckAccess(c.Request.Context(), "prometheus_configs", "prometheus_shares", "config_id", configID, currentUserID, currentUserRole)
	if err != nil || !hasAccess || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only the connection owner can change its visibility"})
		return
	}

	if err := h.shareRepo.UpdateVisibility(c.Request.Context(), "prometheus_configs", configID, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Visibility updated to %s", req.Visibility)})
}

// Database Connection Reconfiguration
func (h *SettingsHandler) GetDatabaseConfig(c *gin.Context) {
	cfg := config.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"host":            cfg.DB.Host,
			"port":            cfg.DB.Port,
			"user":            cfg.DB.User,
			"database":        cfg.DB.Database,
			"ssl":             cfg.DB.SSL,
			"maxConns":        cfg.DB.MaxConns,
			"minConns":        cfg.DB.MinConns,
			"maxConnIdleTime": cfg.DB.MaxConnIdleTime,
			"maxConnLifetime": cfg.DB.MaxConnLifetime,
		},
	})
}

func (h *SettingsHandler) UpdateDatabaseConfig(c *gin.Context) {
	var req config.DBConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.MaxConns <= 0 {
		req.MaxConns = 25
	}
	if req.MinConns < 0 {
		req.MinConns = 5
	}
	if req.MaxConnIdleTime <= 0 {
		req.MaxConnIdleTime = 600
	}
	if req.MaxConnLifetime <= 0 {
		req.MaxConnLifetime = 3600
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

// Service Threads Concurrency Management
func (h *SettingsHandler) GetServiceThreads(c *gin.Context) {
	appCfg := config.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    appCfg.GetServiceThreads(),
	})
}

func (h *SettingsHandler) UpdateServiceThreads(c *gin.Context) {
	var req config.ServiceThreadsConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	appCfg := config.GetConfig()
	if err := appCfg.UpdateServiceThreads(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service thread concurrency settings applied successfully.",
		"data":    appCfg.GetServiceThreads(),
	})
}

