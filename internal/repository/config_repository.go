package repository

import (
	"context"
	"encoding/json"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type ConfigRepository struct{}

func NewConfigRepository() *ConfigRepository {
	return &ConfigRepository{}
}

// AppConfig (Key-Value)
func (r *ConfigRepository) GetAppConfig(ctx context.Context, key string) (string, error) {
	pool, err := database.GetPool()
	if err != nil {
		return "", err
	}
	var val string
	err = pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&val)
	return val, err
}

func (r *ConfigRepository) SetAppConfig(ctx context.Context, key, value string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO app_config (key, value, updated_at) VALUES ($1, $2, NOW())
                             ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, value)
	return err
}

// Grafana Configs
func (r *ConfigRepository) ListGrafana(ctx context.Context) ([]domain.GrafanaConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, host, token, datasource_uid, is_active, created_at FROM grafana_configs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.GrafanaConfig
	for rows.Next() {
		var c domain.GrafanaConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *ConfigRepository) GetActiveGrafana(ctx context.Context) (*domain.GrafanaConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.GrafanaConfig
	err = pool.QueryRow(ctx, `SELECT id, name, host, token, datasource_uid, is_active, created_at FROM grafana_configs WHERE is_active = true LIMIT 1`).
		Scan(&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConfigRepository) SaveGrafana(ctx context.Context, c domain.GrafanaConfig) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	query := `INSERT INTO grafana_configs (id, name, host, token, datasource_uid, is_active)
              VALUES ($1, $2, $3, $4, $5, $6)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, host = EXCLUDED.host, token = EXCLUDED.token,
                datasource_uid = EXCLUDED.datasource_uid, is_active = EXCLUDED.is_active`
	_, err = pool.Exec(ctx, query, c.ID, c.Name, c.Host, c.Token, c.DatasourceUID, c.IsActive)
	return err
}

func (r *ConfigRepository) SetActiveGrafana(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `UPDATE grafana_configs SET is_active = false`)
	_, err = pool.Exec(ctx, `UPDATE grafana_configs SET is_active = true WHERE id = $1`, id)
	return err
}

func (r *ConfigRepository) DeleteGrafana(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM grafana_configs WHERE id = $1`, id)
	return err
}

// Prometheus Configs
func (r *ConfigRepository) ListPrometheus(ctx context.Context) ([]domain.PrometheusConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, is_active, created_at FROM prometheus_configs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PrometheusConfig
	for rows.Next() {
		var c domain.PrometheusConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *ConfigRepository) GetPrometheusByID(ctx context.Context, id string) (*domain.PrometheusConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.PrometheusConfig
	err = pool.QueryRow(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at FROM prometheus_configs WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if c.SSHPassword != nil && *c.SSHPassword != "" {
		if dec, err := config.DecryptText(*c.SSHPassword); err == nil {
			c.SSHPassword = &dec
		}
	}
	if c.SSHKey != nil && *c.SSHKey != "" {
		if dec, err := config.DecryptText(*c.SSHKey); err == nil {
			c.SSHKey = &dec
		}
	}
	return &c, nil
}

func (r *ConfigRepository) GetActivePrometheus(ctx context.Context) (*domain.PrometheusConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.PrometheusConfig
	err = pool.QueryRow(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at FROM prometheus_configs WHERE is_active = true LIMIT 1`).
		Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConfigRepository) SavePrometheus(ctx context.Context, c domain.PrometheusConfig) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	var encPwd, encKey *string
	if c.SSHPassword != nil && *c.SSHPassword != "" && *c.SSHPassword != "********" {
		if enc, err := config.EncryptText(*c.SSHPassword); err == nil {
			encPwd = &enc
		}
	}
	if c.SSHKey != nil && *c.SSHKey != "" && *c.SSHKey != "********" {
		if enc, err := config.EncryptText(*c.SSHKey); err == nil {
			encKey = &enc
		}
	}

	query := `INSERT INTO prometheus_configs (id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, mode = EXCLUDED.mode, path = EXCLUDED.path, reload_url = EXCLUDED.reload_url,
                ssh_host = EXCLUDED.ssh_host, ssh_port = EXCLUDED.ssh_port, ssh_user = EXCLUDED.ssh_user,
                ssh_auth = EXCLUDED.ssh_auth, is_active = EXCLUDED.is_active`
	_, err = pool.Exec(ctx, query, c.ID, c.Name, c.Mode, c.Path, c.ReloadURL, c.SSHHost, c.SSHPort, c.SSHUser, c.SSHAuth, encPwd, encKey, c.IsActive)
	return err
}

func (r *ConfigRepository) SetActivePrometheus(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `UPDATE prometheus_configs SET is_active = false`)
	_, err = pool.Exec(ctx, `UPDATE prometheus_configs SET is_active = true WHERE id = $1`, id)
	return err
}

func (r *ConfigRepository) DeletePrometheus(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM prometheus_configs WHERE id = $1`, id)
	return err
}

// Monitoring Views
func (r *ConfigRepository) ListMonitoringViews(ctx context.Context) ([]domain.MonitoringView, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, description, interval, mode, panels, created_at FROM monitoring_views ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.MonitoringView
	for rows.Next() {
		var v domain.MonitoringView
		var panelsRaw []byte
		if err := rows.Scan(&v.ID, &v.Name, &v.Description, &v.Interval, &v.Mode, &panelsRaw, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(panelsRaw, &v.Panels)
		list = append(list, v)
	}
	return list, nil
}

func (r *ConfigRepository) SaveMonitoringView(ctx context.Context, v domain.MonitoringView) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	panelsJSON, err := json.Marshal(v.Panels)
	if err != nil {
		panelsJSON = []byte("[]")
	}

	query := `INSERT INTO monitoring_views (id, name, description, interval, mode, panels)
              VALUES ($1, $2, $3, $4, $5, $6)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, description = EXCLUDED.description, interval = EXCLUDED.interval,
                mode = EXCLUDED.mode, panels = EXCLUDED.panels`
	_, err = pool.Exec(ctx, query, v.ID, v.Name, v.Description, v.Interval, v.Mode, panelsJSON)
	return err
}

func (r *ConfigRepository) DeleteMonitoringView(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM monitoring_views WHERE id = $1`, id)
	return err
}
