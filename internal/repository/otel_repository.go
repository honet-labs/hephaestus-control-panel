package repository

import (
	"context"
	"errors"
	"strings"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
)

type OTelRepository struct{}

func NewOTelRepository() *OTelRepository {
	return &OTelRepository{}
}

// List returns all OpenTelemetry configurations, with passwords/keys masked for security
func (r *OTelRepository) List(ctx context.Context) ([]domain.OpenTelemetryConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			id, name, tags, ssh_host, ssh_port, ssh_user, ssh_auth,
			config_path, service_name, reload_mode, last_status, is_active,
			created_at, updated_at
		FROM opentelemetry_configs
		ORDER BY name ASC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.OpenTelemetryConfig
	for rows.Next() {
		var c domain.OpenTelemetryConfig
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Tags, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth,
			&c.ConfigPath, &c.ServiceName, &c.ReloadMode, &c.LastStatus, &c.IsActive,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if c.Tags == nil {
			c.Tags = []string{}
		}
		list = append(list, c)
	}
	return list, nil
}

// GetByID returns a single OpenTelemetry configuration with decrypted credentials
func (r *OTelRepository) GetByID(ctx context.Context, id string) (*domain.OpenTelemetryConfig, error) {
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
func (r *OTelRepository) Save(ctx context.Context, cfg domain.OpenTelemetryConfig) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
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

	query := `
		INSERT INTO opentelemetry_configs (
			id, name, tags, ssh_host, ssh_port, ssh_user, ssh_auth,
			ssh_password, ssh_key, config_path, service_name, reload_mode,
			last_status, is_active, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
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
			updated_at = NOW()
	`

	_, err = pool.Exec(ctx, query,
		cfg.ID, cfg.Name, cfg.Tags, cfg.SSHHost, cfg.SSHPort, cfg.SSHUser, cfg.SSHAuth,
		encPassword, encKey, cfg.ConfigPath, cfg.ServiceName, cfg.ReloadMode,
		cfg.LastStatus, cfg.IsActive,
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
func (r *OTelRepository) Delete(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
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
