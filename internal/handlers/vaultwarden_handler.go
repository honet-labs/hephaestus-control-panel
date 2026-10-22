package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type VaultwardenHandler struct {
	vwService *services.VaultwardenService
	vwRepo    *repository.VaultwardenRepository
}

func NewVaultwardenHandler(vwService *services.VaultwardenService, vwRepo *repository.VaultwardenRepository) *VaultwardenHandler {
	return &VaultwardenHandler{
		vwService: vwService,
		vwRepo:    vwRepo,
	}
}

// GetConfig returns the active Vaultwarden integration settings (with MasterPassword masked)
func (h *VaultwardenHandler) GetConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")

	cfg, err := h.vwRepo.GetConfigPublic(c.Request.Context(), userID, userRole, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to load Vaultwarden configuration",
			"details": err.Error(),
		})
		return
	}

	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"configured": false,
			"data":       nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"configured": cfg.ServerURL != "" && cfg.Email != "",
		"data":       cfg,
	})
}

// ListConfigs returns all Vaultwarden instances accessible to the user
func (h *VaultwardenHandler) ListConfigs(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configs, err := h.vwRepo.ListConfigs(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list Vaultwarden configurations",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configs,
	})
}

// SaveConfig saves or updates a user-scoped Vaultwarden connection
func (h *VaultwardenHandler) SaveConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)

	var input struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		ServerURL      string `json:"serverUrl" binding:"required"`
		Email          string `json:"email" binding:"required"`
		MasterPassword string `json:"masterPassword"`
		Visibility     string `json:"visibility"`
		IsActive       *bool  `json:"isActive"`
		AutoSync       bool   `json:"autoSync"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Validation failed: serverUrl and email are required",
		})
		return
	}

	serverURL := strings.TrimRight(strings.TrimSpace(input.ServerURL), "/")
	email := strings.ToLower(strings.TrimSpace(input.Email))

	cfg := domain.VaultwardenConfig{
		ID:             input.ID,
		Name:           input.Name,
		ServerURL:      serverURL,
		Email:          email,
		MasterPassword: input.MasterPassword,
		Visibility:     input.Visibility,
		IsActive:       true,
	}
	if input.IsActive != nil {
		cfg.IsActive = *input.IsActive
	}

	saved, err := h.vwRepo.SaveConfig(c.Request.Context(), cfg, userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Invalidate any existing cached session for this instance to ensure fresh authentication
	h.vwService.InvalidateSession(serverURL, email)

	// Trigger initial background sync if autoSync requested
	if input.AutoSync && input.MasterPassword != "" {
		go func() {
			_, _ = h.vwService.SyncVault(context.Background(), userID, userRole, saved.ID)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Vaultwarden configuration saved successfully",
		"data":    saved,
	})
}

// TestConnection tests connectivity and master password against Vaultwarden
func (h *VaultwardenHandler) TestConnection(c *gin.Context) {
	userID, userRole := getUserContext(c)

	var input struct {
		ConfigID       string `json:"configId"`
		ServerURL      string `json:"serverUrl" binding:"required"`
		Email          string `json:"email" binding:"required"`
		MasterPassword string `json:"masterPassword"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Server URL and email are required",
		})
		return
	}

	pwd := input.MasterPassword
	if pwd == "" && input.ConfigID != "" {
		existing, err := h.vwRepo.GetConfig(c.Request.Context(), userID, userRole, input.ConfigID)
		if err == nil && existing != nil {
			pwd = existing.MasterPassword
		}
	}

	if pwd == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Master password is required for connection testing",
		})
		return
	}

	ok, msg, count := h.vwService.TestConnection(c.Request.Context(), input.ServerURL, input.Email, pwd)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   msg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    msg,
		"totalItems": count,
	})
}

// SyncVault triggers an immediate synchronization of credentials
func (h *VaultwardenHandler) SyncVault(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")

	res, err := h.vwService.SyncVault(c.Request.Context(), userID, userRole, configID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
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

// GetCiphers returns decrypted credentials with optional keyword and folder query filters
func (h *VaultwardenHandler) GetCiphers(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")
	keyword := c.Query("keyword")
	folder := c.Query("folder")

	items, err := h.vwService.GetCiphers(c.Request.Context(), userID, userRole, keyword, folder, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to load vault credentials",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"totalItems": len(items),
		"data":       items,
	})
}

// DeleteConfig disconnects and deletes the Vaultwarden integration
func (h *VaultwardenHandler) DeleteConfig(c *gin.Context) {
	userID, userRole := getUserContext(c)
	id := c.Query("id")
	if id == "" || id == "active" {
		cfg, err := h.vwRepo.GetConfig(c.Request.Context(), userID, userRole)
		if err == nil && cfg != nil {
			id = cfg.ID
		}
	}

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No active Vaultwarden configuration to delete",
		})
		return
	}

	if err := h.vwRepo.DeleteConfig(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Vaultwarden configuration deleted successfully",
	})
}

// CreateCipher handles adding a new credential directly into Vaultwarden
func (h *VaultwardenHandler) CreateCipher(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")

	var input domain.CreateVaultCipherRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Validation failed: Item name is required",
			"details": err.Error(),
		})
		return
	}

	item, err := h.vwService.CreateCipher(c.Request.Context(), userID, userRole, input, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create credential in Vaultwarden",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Credential created and synchronized successfully",
		"data":    item,
	})
}

// DeleteCipher handles removing a credential directly from Vaultwarden
func (h *VaultwardenHandler) DeleteCipher(c *gin.Context) {
	userID, userRole := getUserContext(c)
	cipherID := c.Param("id")
	configID := c.Query("id")

	if cipherID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Cipher ID is required",
		})
		return
	}

	if err := h.vwService.DeleteCipher(c.Request.Context(), userID, userRole, cipherID, configID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete credential from Vaultwarden",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Credential deleted and vault synchronized successfully",
	})
}

// UpdateCipher handles modifying an existing credential in Vaultwarden
func (h *VaultwardenHandler) UpdateCipher(c *gin.Context) {
	userID, userRole := getUserContext(c)
	cipherID := c.Param("id")
	configID := c.Query("id")

	if cipherID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Cipher ID is required",
		})
		return
	}

	var input domain.CreateVaultCipherRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Validation failed: Item name is required",
			"details": err.Error(),
		})
		return
	}

	item, err := h.vwService.UpdateCipher(c.Request.Context(), userID, userRole, cipherID, input, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update credential in Vaultwarden",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Credential updated and synchronized successfully",
		"data":    item,
	})
}

// ==================== SHARING HANDLERS ====================

func (h *VaultwardenHandler) ListShares(c *gin.Context) {
	configID := c.Query("configId")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query param 'configId' is required"})
		return
	}

	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.vwRepo.SharesRepo().CheckAccess(c.Request.Context(), "vaultwarden_configs", "vaultwarden_shares", "config_id", configID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only connection owner or administrator can view shares"})
		return
	}

	shares, err := h.vwRepo.SharesRepo().ListShares(c.Request.Context(), "vaultwarden_shares", "config_id", configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *VaultwardenHandler) AddShare(c *gin.Context) {
	userID, userRole := getUserContext(c)

	var req struct {
		ConfigID   string `json:"configId" binding:"required"`
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	hasAccess, isOwner, _, err := h.vwRepo.SharesRepo().CheckAccess(c.Request.Context(), "vaultwarden_configs", "vaultwarden_shares", "config_id", req.ConfigID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only connection owner or administrator can share access"})
		return
	}

	if req.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "You cannot share a connection with yourself"})
		return
	}

	if err := h.vwRepo.SharesRepo().AddShare(c.Request.Context(), "vaultwarden_shares", "config_id", req.ConfigID, req.UserID, req.Permission, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access granted successfully."})
}

func (h *VaultwardenHandler) DeleteShare(c *gin.Context) {
	configID := c.Query("configId")
	targetUserID, err := strconv.Atoi(c.Param("userId"))
	if err != nil || configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid configId or userId"})
		return
	}

	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.vwRepo.SharesRepo().CheckAccess(c.Request.Context(), "vaultwarden_configs", "vaultwarden_shares", "config_id", configID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only connection owner or administrator can revoke access"})
		return
	}

	if err := h.vwRepo.SharesRepo().DeleteShare(c.Request.Context(), "vaultwarden_shares", "config_id", configID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access revoked successfully."})
}

func (h *VaultwardenHandler) UpdateVisibility(c *gin.Context) {
	userID, userRole := getUserContext(c)

	var req struct {
		ConfigID   string `json:"configId" binding:"required"`
		Visibility string `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	hasAccess, isOwner, _, err := h.vwRepo.SharesRepo().CheckAccess(c.Request.Context(), "vaultwarden_configs", "vaultwarden_shares", "config_id", req.ConfigID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !domain.IsAdminRole(userRole)) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only connection owner or administrator can modify visibility"})
		return
	}

	if err := h.vwRepo.SharesRepo().UpdateVisibility(c.Request.Context(), "vaultwarden_configs", req.ConfigID, req.Visibility); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Visibility updated successfully."})
}

// GetFolders returns all folders from cache or remote Vaultwarden
func (h *VaultwardenHandler) GetFolders(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")

	folders, err := h.vwService.GetFolders(c.Request.Context(), userID, userRole, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    folders,
	})
}

// CreateFolder creates a new folder in Vaultwarden
func (h *VaultwardenHandler) CreateFolder(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")

	var input domain.CreateVaultFolderRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Folder name is required",
		})
		return
	}

	folder, err := h.vwService.CreateFolder(c.Request.Context(), userID, userRole, input, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Folder created successfully",
		"data":    folder,
	})
}

// UpdateFolder updates an existing folder in Vaultwarden
func (h *VaultwardenHandler) UpdateFolder(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")
	folderID := c.Param("id")

	var input domain.CreateVaultFolderRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Folder name is required",
		})
		return
	}

	folder, err := h.vwService.UpdateFolder(c.Request.Context(), userID, userRole, folderID, input, configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Folder updated successfully",
		"data":    folder,
	})
}

// DeleteFolder deletes a folder from Vaultwarden
func (h *VaultwardenHandler) DeleteFolder(c *gin.Context) {
	userID, userRole := getUserContext(c)
	configID := c.Query("id")
	folderID := c.Param("id")

	if err := h.vwService.DeleteFolder(c.Request.Context(), userID, userRole, folderID, configID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Folder deleted successfully",
	})
}

