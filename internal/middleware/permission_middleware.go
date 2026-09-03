package middleware

import (
	"net/http"
	"strings"

	"go-hephaestus/internal/core/domain"

	"github.com/gin-gonic/gin"
)

// RequirePermission checks if the authenticated user has access to the specified feature with action ("read" or "manage").
// Action hierarchy:
// - "manage" grants both read and write/edit capabilities.
// - "read" only grants read/viewing capabilities.
// - "ADMIN" role or wildcard {"*": "manage"} automatically grants all capabilities.
func RequirePermission(feature string, requiredAction string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userObj, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Unauthorized",
				"message": "Authentication required.",
			})
			return
		}

		user, ok := userObj.(*domain.User)
		if !ok || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Unauthorized",
				"message": "User session invalid.",
			})
			return
		}

		// 1. Superadmin role check
		if strings.EqualFold(user.Role, "ADMIN") {
			c.Next()
			return
		}

		// 2. Check wildcard "*" permission
		if user.Permissions != nil {
			if wild, ok := user.Permissions["*"]; ok && strings.EqualFold(wild, "manage") {
				c.Next()
				return
			}
		}

		// 3. Check specific feature permission
		featurePerm := "none"
		if user.Permissions != nil {
			if p, ok := user.Permissions[feature]; ok {
				featurePerm = strings.ToLower(p)
			}
		}

		allowed := false
		switch strings.ToLower(requiredAction) {
		case "read":
			// Either "read" or "manage" permits read actions
			if featurePerm == "read" || featurePerm == "manage" {
				allowed = true
			}
		case "manage", "write", "edit":
			// Only "manage" permits mutating actions
			if featurePerm == "manage" {
				allowed = true
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":  false,
				"error":    "Forbidden",
				"message":  "Access denied: insufficient permissions for feature '" + feature + "'.",
				"feature":  feature,
				"required": requiredAction,
				"current":  featurePerm,
			})
			return
		}

		c.Next()
	}
}
