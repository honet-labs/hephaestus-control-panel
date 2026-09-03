package handlers

import (
	"net/http"
	"strings"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Bad Request",
			"message": "Username and password are required.",
		})
		return
	}

	user, token, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Unauthorized",
			"message": err.Error(),
		})
		return
	}

	c.SetCookie("hephaestus_session", token, 86400*7, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":                  user.ID,
				"username":            user.Username,
				"role":                user.Role,
				"permissions":         user.Permissions,
				"forcePasswordChange": user.ForcePasswordChange,
			},
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token := ""
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else if cookie, err := c.Cookie("hephaestus_session"); err == nil {
		token = cookie
	}

	if token != "" {
		_ = h.authService.Logout(c.Request.Context(), token)
	}

	c.SetCookie("hephaestus_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out successfully."})
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": user})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	userID := c.GetInt("userId")
	if err := h.authService.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password changed successfully."})
}

// SetupHandler handles initial installation wizard
type SetupHandler struct {
	authService *services.AuthService
}

func NewSetupHandler(authService *services.AuthService) *SetupHandler {
	return &SetupHandler{authService: authService}
}

func (h *SetupHandler) GetSetupStatus(c *gin.Context) {
	completed := h.authService.IsSetupCompleted(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"setupCompleted": completed,
		},
	})
}

func (h *SetupHandler) CompleteSetup(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Username and password required"})
		return
	}

	user, token, err := h.authService.CompleteSetup(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.SetCookie("hephaestus_session", token, 86400*7, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Initial setup completed successfully.",
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}
