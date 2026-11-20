package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

// Feature-level RBAC is not sufficient: also authorize the selected profile.
func IntegrationAccessGuard(prom *services.PrometheusService, dp *services.DataPrepperService) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, _ := c.Get("user")
		user, _ := value.(*domain.User)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
			return
		}
		if user.IsAdmin() {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		table := ""
		id := c.Param("id")
		action := "manage"
		if c.Request.Method == http.MethodGet {
			action = "read"
		}
		switch {
		case strings.HasPrefix(path, "/api/v1/otel/hosts"):
			table = "opentelemetry"
		case strings.HasPrefix(path, "/api/v1/settings/grafana/"):
			table = "grafana"
		case strings.HasPrefix(path, "/api/v1/settings/prometheus/"):
			table = "prometheus"
		case strings.HasPrefix(path, "/api/v1/opensearch/configs/"):
			table = "opensearch"
		}
		if strings.HasPrefix(path, "/api/v1/otel/hosts") && id == "" && c.Request.Method != http.MethodGet {
			var selection struct {
				ID string `json:"id"`
			}
			if !readSelection(c, &selection) {
				return
			}
			id = selection.ID
		}
		if table != "" && id != "" && !requireIntegration(c, table, id, action) {
			return
		}
		if c.Request.Method != http.MethodGet && (path == "/api/v1/prometheus/config" || path == "/api/v1/prometheus/reload") {
			var selection struct {
				InstanceID string `json:"instanceId"`
			}
			if path == "/api/v1/prometheus/config" && !readSelection(c, &selection) {
				return
			}
			var cfg *domain.PrometheusConfig
			var err error
			if selection.InstanceID != "" {
				cfg, err = repository.NewConfigRepository().GetPrometheusByID(c.Request.Context(), selection.InstanceID)
			} else {
				cfg, err = prom.GetActiveConfig(c.Request.Context())
			}
			if err != nil || cfg == nil {
				denyIntegration(c)
				return
			}
			if !requireIntegration(c, "prometheus", cfg.ID, "manage") {
				return
			}
		}
		if c.Request.Method != http.MethodGet && strings.HasPrefix(path, "/api/v1/dataprepper/pipeline") {
			var selection struct {
				InstanceID string `json:"instanceId"`
			}
			if c.Request.Method != http.MethodDelete && !readSelection(c, &selection) {
				return
			}
			if query := c.Query("instanceId"); query != "" {
				selection.InstanceID = query
			}
			cfg, err := dp.GetActiveConfig(c.Request.Context(), selection.InstanceID)
			if err != nil || cfg == nil {
				denyIntegration(c)
				return
			}
			pool, err := database.GetPool()
			if err != nil {
				denyIntegration(c)
				return
			}
			var isDP bool
			if err := pool.QueryRow(c.Request.Context(), "SELECT EXISTS(SELECT 1 FROM dataprepper_configs WHERE id=$1)", cfg.ID).Scan(&isDP); err != nil {
				denyIntegration(c)
				return
			}
			profile := "prometheus"
			if isDP {
				profile = "dataprepper"
			}
			if !requireIntegration(c, profile, cfg.ID, "manage") {
				return
			}
		}
		c.Next()
	}
}

func readSelection(c *gin.Context, value interface{}) bool {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "error": "Configuration request too large"})
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	if err := json.Unmarshal(body, value); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON configuration"})
		return false
	}
	return true
}

func requireIntegration(c *gin.Context, profile, id, action string) bool {
	if err := repository.RequireConnectionAccess(c.Request.Context(), profile+"_configs", profile+"_shares", "config_id", id, action); err != nil {
		denyIntegration(c)
		return false
	}
	return true
}

func denyIntegration(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied to the selected integration profile"})
}
