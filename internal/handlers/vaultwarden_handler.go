package handlers

import (
	"net/http"
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
	cfg, err := h.vwRepo.GetConfigPublic(c.Request.Context())
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

// SaveConfig saves or updates the Vaultwarden connection parameters
func (h *VaultwardenHandler) SaveConfig(c *gin.Context) {
	var input struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		ServerURL      string `json:"serverUrl" binding:"required"`
		Email          string `json:"email" binding:"required"`
		MasterPassword string `json:"masterPassword"`
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
		IsActive:       true,
	}
	if input.IsActive != nil {
		cfg.IsActive = *input.IsActive
	}

	saved, err := h.vwRepo.SaveConfig(c.Request.Context(), cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save Vaultwarden configuration",
			"details": err.Error(),
		})
		return
	}

	// Trigger initial background sync if autoSync requested
	if input.AutoSync && input.MasterPassword != "" {
		go func() {
			_, _ = h.vwService.SyncVault(c.Request.Context())
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
	var input struct {
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

	// If master password is empty, check if one is already saved in DB
	pwd := input.MasterPassword
	if pwd == "" {
		existing, err := h.vwRepo.GetConfig(c.Request.Context())
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

// SyncVault triggers an immediate synchronization of all credentials
func (h *VaultwardenHandler) SyncVault(c *gin.Context) {
	res, err := h.vwService.SyncVault(c.Request.Context())
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
	keyword := c.Query("keyword")
	folder := c.Query("folder")

	items, err := h.vwService.GetCiphers(c.Request.Context(), keyword, folder)
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
	id := c.Query("id")
	if id == "" {
		id = "active"
	}

	if err := h.vwRepo.DeleteConfig(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete Vaultwarden configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Vaultwarden configuration deleted successfully",
	})
}
