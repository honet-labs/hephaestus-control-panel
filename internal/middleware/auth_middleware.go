package middleware

import (
	"net/http"
	"strings"

	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates session token from Authorization header or Cookie
func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""

		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if cookie, err := c.Cookie("hephaestus_session"); err == nil {
			token = cookie
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Unauthorized",
				"message": "Authentication required. Please login.",
			})
			return
		}

		user, err := authService.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Unauthorized",
				"message": "Invalid or expired session. Please login again.",
			})
			return
		}

		// Store user in context
		c.Set("user", user)
		c.Set("userId", user.ID)
		c.Set("userRole", user.Role)
		c.Next()
	}
}

// RequireRole checks whether the authenticated user has one of the allowed roles
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Forbidden",
				"message": "Access denied.",
			})
			return
		}

		roleStr := userRole.(string)
		for _, r := range roles {
			if strings.EqualFold(r, roleStr) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Forbidden",
			"message": "You do not have permission to access this resource.",
		})
	}
}
