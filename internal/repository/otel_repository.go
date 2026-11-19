package repository

import (
	"context"
	"errors"
	"strings"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OTelRepository struct{}

func NewOTelRepository() *OTelRepository {
	return &OTelRepository{}
}

// List returns all OpenTelemetry configurations, with passwords/keys masked for security
func (r *OTelRepository) List(ctx context.Context, userOpt ...interface{}) ([]domain.OpenTelemetryConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var userID int
	var userRole string
	if len(userOpt) >= 2 {
		if id, ok := userOpt[0].(int); ok {
			userID = id
		}
		if role, ok := userOpt[1].(string); ok {
			userRole = role
		}
	}

	isAdmin := domain.IsAdminRole(userRole) || (userID == 0 && userRole == "ADMIN") || len(userOpt) == 0

	var rows pgx.Rows
	if isAdmin {
		query := `
			SELECT 
				c.id, c.name, c.tags, c.ssh_host, c.ssh_port, c.ssh_user, c.ssh_auth,
				c.config_path, c.service_name, c.reload_mode, c.last_status, c.is_active,
				c.created_at, c.updated_at,
				c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
				(SELECT COUNT(*) FROM opentelemetry_shares WHERE config_id = c.id) AS shares_count
			FROM opentelemetry_configs c
			LEFT JOIN users u ON c.user_id = u.id
			ORDER BY c.name ASC
		`
		rows, err = pool.Query(ctx, query)
	} else {
		query := `
			SELECT 
				c.id, c.name, c.tags, c.ssh_host, c.ssh_port, c.ssh_user, c.ssh_auth,
				c.config_path, c.service_name, c.reload_mode, c.last_status, c.is_active,
				c.created_at, c.updated_at,
				c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
				(SELECT COUNT(*) FROM opentelemetry_shares WHERE config_id = c.id) AS shares_count,
				COALESCE(ots.permission, '') AS share_perm
			FROM opentelemetry_configs c
			LEFT JOIN users u ON c.user_id = u.id
			LEFT JOIN opentelemetry_shares ots ON c.id = ots.config_id AND ots.user_id = $1
			WHERE c.visibility = 'public'
			   OR c.user_id = $1
			   OR ots.user_id = $1
			ORDER BY c.name ASC
		`
		rows, err = pool.Query(ctx, query, userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.OpenTelemetryConfig
	for rows.Next() {
		var c domain.OpenTelemetryConfig
		var sharePerm string
		if isAdmin {
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Tags, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth,
				&c.ConfigPath, &c.ServiceName, &c.ReloadMode, &c.LastStatus, &c.IsActive,
				&c.CreatedAt, &c.UpdatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount,
			); err != nil {
				return nil, err
			}
			c.IsOwner = true
			c.UserPermission = "manage"
		} else {
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Tags, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth,
				&c.ConfigPath, &c.ServiceName, &c.ReloadMode, &c.LastStatus, &c.IsActive,
				&c.CreatedAt, &c.UpdatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount, &sharePerm,
			); err != nil {
				return nil, err
			}
			c.IsOwner = (c.UserID != nil && *c.UserID == userID)
			if c.IsOwner {
				c.UserPermission = "manage"
			} else if sharePerm != "" {
				c.UserPermission = sharePerm
			} else {
				c.UserPermission = "read"
			}
		}

		if c.Tags == nil {
			c.Tags = []string{}
		}
		list = append(list, c)
	}

	if list == nil {
		list = []domain.OpenTelemetryConfig{}
	}
	return list, nil
}

// GetByID returns a single OpenTelemetry configuration with decrypted credentials
func (r *OTelRepository) GetByID(ctx context.Context, id string) (*domain.OpenTelemetryConfig, error) {
	if err := RequireConnectionAccess(ctx, "opentelemetry_configs", "opentelemetry_shares", "config_id", id, "read"); err != nil {
		return nil, err
	}
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			id, name, tags, ssh_host, ssh_port, ssh_user, ssh_auth,
			ssh_password, ssh_key, config_path, service_name, reload_mode,
			last_status, is_active, created_at, updated_at
		FROM opentelemetry_configs
		WHERE id = $1
	`
	var c domain.OpenTelemetryConfig
	err = pool.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Name, &c.Tags, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth,
		&c.SSHPassword, &c.SSHKey, &c.ConfigPath, &c.ServiceName, &c.ReloadMode,
		&c.LastStatus, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}

	if c.SSHPassword != nil && *c.SSHPassword != "" {
		if decrypted, err := config.DecryptText(*c.SSHPassword); err == nil {
			c.SSHPassword = &decrypted
		}
	}
	if c.SSHKey != nil && *c.SSHKey != "" {
		if decrypted, err := config.DecryptText(*c.SSHKey); err == nil {
			c.SSHKey = &decrypted
		}
	}

	return &c, nil
}

// Save creates or updates an OpenTelemetry configuration
func (r *OTelRepository) Save(ctx context.Context, cfg domain.OpenTelemetryConfig, userOpt ...interface{}) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	var userID int
	var userRole string
	if len(userOpt) >= 2 {
		if id, ok := userOpt[0].(int); ok {
			userID = id
		}
		if role, ok := userOpt[1].(string); ok {
			userRole = role
		}
	}

	isAdmin := domain.IsAdminRole(userRole) || (userID == 0 && userRole == "ADMIN") || len(userOpt) == 0

	if cfg.ID != "" {
		var existingOwner *int
		errExist := pool.QueryRow(ctx, "SELECT user_id FROM opentelemetry_configs WHERE id = $1", cfg.ID).Scan(&existingOwner)
		if errExist == nil && !isAdmin && userID > 0 {
			if existingOwner == nil || *existingOwner != userID {
				var perm string
				errShare := pool.QueryRow(ctx, "SELECT permission FROM opentelemetry_shares WHERE config_id = $1 AND user_id = $2", cfg.ID, userID).Scan(&perm)
				if errShare != nil || perm != "manage" {
					return errors.New("you do not have permission to edit this OpenTelemetry configuration")
				}
			}
		}
	}

	if strings.TrimSpace(cfg.ID) == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.SSHPort <= 0 {
		cfg.SSHPort = 22
	}
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		cfg.ConfigPath = "/etc/otelcol-contrib/config.yaml"
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = "otelcol-contrib"
	}
	if strings.TrimSpace(cfg.ReloadMode) == "" {
		cfg.ReloadMode = "restart"
	}
	if strings.TrimSpace(cfg.LastStatus) == "" {
		cfg.LastStatus = "unknown"
	}
	if cfg.Tags == nil {
		cfg.Tags = []string{}
	}

	var encPassword, encKey *string
	if cfg.SSHPassword != nil && *cfg.SSHPassword != "" && *cfg.SSHPassword != "********" {
		if enc, err := config.EncryptText(*cfg.SSHPassword); err == nil {
			encPassword = &enc
		}
	}
	if cfg.SSHKey != nil && *cfg.SSHKey != "" && *cfg.SSHKey != "********" {
		if enc, err := config.EncryptText(*cfg.SSHKey); err == nil {
			encKey = &enc
		}
	}

	var assignedUserID *int
	if userID > 0 {
		assignedUserID = &userID
	} else if cfg.UserID != nil {
		assignedUserID = cfg.UserID
	}
	if cfg.Visibility == "" {
		cfg.Visibility = "private"
	}

	query := `
		INSERT INTO opentelemetry_configs (
			id, name, tags, ssh_host, ssh_port, ssh_user, ssh_auth,
			ssh_password, ssh_key, config_path, service_name, reload_mode,
			last_status, is_active, user_id, visibility, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			tags = EXCLUDED.tags,
			ssh_host = EXCLUDED.ssh_host,
			ssh_port = EXCLUDED.ssh_port,
			ssh_user = EXCLUDED.ssh_user,
			ssh_auth = EXCLUDED.ssh_auth,
			ssh_password = COALESCE($8, opentelemetry_configs.ssh_password),
			ssh_key = COALESCE($9, opentelemetry_configs.ssh_key),
			config_path = EXCLUDED.config_path,
			service_name = EXCLUDED.service_name,
			reload_mode = EXCLUDED.reload_mode,
			is_active = EXCLUDED.is_active,
			visibility = COALESCE(NULLIF(EXCLUDED.visibility, ''), opentelemetry_configs.visibility),
			updated_at = NOW()
	`

	_, err = pool.Exec(ctx, query,
		cfg.ID, cfg.Name, cfg.Tags, cfg.SSHHost, cfg.SSHPort, cfg.SSHUser, cfg.SSHAuth,
		encPassword, encKey, cfg.ConfigPath, cfg.ServiceName, cfg.ReloadMode,
		cfg.LastStatus, cfg.IsActive, assignedUserID, cfg.Visibility,
	)
	return err
}

// UpdateStatus updates the connection/service status of an OpenTelemetry host
func (r *OTelRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE opentelemetry_configs SET last_status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

// Delete removes an OpenTelemetry configuration
func (r *OTelRepository) Delete(ctx context.Context, id string, userOpt ...interface{}) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	var userID int
	var userRole string
	if len(userOpt) >= 2 {
		if uid, ok := userOpt[0].(int); ok {
			userID = uid
		}
		if role, ok := userOpt[1].(string); ok {
			userRole = role
		}
	}

	if !domain.IsAdminRole(userRole) && userID > 0 {
		var ownerID *int
		errCheck := pool.QueryRow(ctx, "SELECT user_id FROM opentelemetry_configs WHERE id = $1", id).Scan(&ownerID)
		if errCheck != nil {
			return errors.New("host configuration not found")
		}
		if ownerID == nil || *ownerID != userID {
			var perm string
			errShare := pool.QueryRow(ctx, "SELECT permission FROM opentelemetry_shares WHERE config_id = $1 AND user_id = $2", id, userID).Scan(&perm)
			if errShare != nil || perm != "manage" {
				return errors.New("you do not have permission to delete this OpenTelemetry configuration")
			}
		}
	}

	res, err := pool.Exec(ctx, `DELETE FROM opentelemetry_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("host configuration not found")
	}
	return nil
}

// SaveHistory records a backup of configuration content
func (r *OTelRepository) SaveHistory(ctx context.Context, h domain.OpenTelemetryConfigHistory) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	if strings.TrimSpace(h.ID) == "" {
		h.ID = uuid.New().String()
	}
	query := `
		INSERT INTO opentelemetry_config_history (id, otel_config_id, content, created_by, change_summary, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err = pool.Exec(ctx, query, h.ID, h.OTelConfigID, h.Content, h.CreatedBy, h.ChangeSummary)
	return err
}

// ListHistory returns version history for a given OpenTelemetry config
func (r *OTelRepository) ListHistory(ctx context.Context, otelConfigID string, limit int) ([]domain.OpenTelemetryConfigHistory, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT id, otel_config_id, content, created_by, change_summary, created_at
		FROM opentelemetry_config_history
		WHERE otel_config_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := pool.Query(ctx, query, otelConfigID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.OpenTelemetryConfigHistory
	for rows.Next() {
		var h domain.OpenTelemetryConfigHistory
		if err := rows.Scan(&h.ID, &h.OTelConfigID, &h.Content, &h.CreatedBy, &h.ChangeSummary, &h.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, nil
}
