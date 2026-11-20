package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
func (r *ConfigRepository) ListGrafana(ctx context.Context, userID int, userRole string) ([]domain.GrafanaConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)
	var list []domain.GrafanaConfig

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		query := `
			SELECT c.id, c.name, c.host, c.token, c.datasource_uid, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM grafana_shares WHERE config_id = c.id) AS shares_count
			FROM grafana_configs c
			LEFT JOIN users u ON c.user_id = u.id
			ORDER BY c.name ASC
		`
		rows, err := pool.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.GrafanaConfig
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount,
			); err != nil {
				return nil, err
			}
			c.IsOwner = true
			c.UserPermission = "manage"
			list = append(list, c)
		}
	} else {
		query := `
			SELECT c.id, c.name, c.host, c.token, c.datasource_uid, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM grafana_shares WHERE config_id = c.id) AS shares_count,
			       COALESCE(gs.permission, '') AS share_perm
			FROM grafana_configs c
			LEFT JOIN users u ON c.user_id = u.id
			LEFT JOIN grafana_shares gs ON c.id = gs.config_id AND gs.user_id = $1
			WHERE c.visibility = 'public'
			   OR c.user_id = $1
			   OR gs.user_id = $1
			ORDER BY c.name ASC
		`
		rows, err := pool.Query(ctx, query, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.GrafanaConfig
			var sharePerm string
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt,
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
			list = append(list, c)
		}
	}

	if list == nil {
		list = []domain.GrafanaConfig{}
	}
	return list, nil
}

func (r *ConfigRepository) GetActiveGrafana(ctx context.Context) (*domain.GrafanaConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.GrafanaConfig
	err = pool.QueryRow(ctx, `SELECT id, name, host, token, datasource_uid, is_active, created_at, user_id, COALESCE(visibility, 'private') FROM grafana_configs WHERE is_active = true LIMIT 1`).
		Scan(&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt, &c.UserID, &c.Visibility)
	if err != nil {
		return nil, err
	}
	if err := RequireConnectionAccess(ctx, "grafana_configs", "grafana_shares", "config_id", c.ID, "read"); err != nil {
		return nil, err
	}
	c.Token, err = config.DecryptText(c.Token)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt Grafana token: %w", err)
	}
	return &c, nil
}

func (r *ConfigRepository) SaveGrafana(ctx context.Context, c domain.GrafanaConfig, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	isAdmin := domain.IsAdminRole(userRole)

	// Check existing
	if c.ID != "" {
		var existingOwner *int
		errExist := pool.QueryRow(ctx, "SELECT user_id FROM grafana_configs WHERE id = $1", c.ID).Scan(&existingOwner)
		if errExist == nil && !isAdmin && userID > 0 {
			if existingOwner == nil || *existingOwner != userID {
				var perm string
				errShare := pool.QueryRow(ctx, "SELECT permission FROM grafana_shares WHERE config_id = $1 AND user_id = $2", c.ID, userID).Scan(&perm)
				if errShare != nil || perm != "manage" {
					return fmt.Errorf("you do not have permission to edit this configuration")
				}
			}
		}
	}

	var assignedUserID *int
	if userID > 0 {
		assignedUserID = &userID
	}
	if c.Visibility == "" {
		c.Visibility = "private"
	}

	query := `INSERT INTO grafana_configs (id, name, host, token, datasource_uid, is_active, user_id, visibility)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, host = EXCLUDED.host,
                token = CASE WHEN EXCLUDED.token != '' THEN EXCLUDED.token ELSE grafana_configs.token END,
                datasource_uid = EXCLUDED.datasource_uid, is_active = EXCLUDED.is_active,
                visibility = COALESCE(NULLIF(EXCLUDED.visibility, ''), grafana_configs.visibility)`
	if c.Token == "********" || c.Token == "••••••••" {
		c.Token = ""
	}
	encryptedToken, err := config.EncryptText(c.Token)
	if err != nil {
		return fmt.Errorf("cannot encrypt Grafana token: %w", err)
	}
	_, err = pool.Exec(ctx, query, c.ID, c.Name, c.Host, encryptedToken, c.DatasourceUID, c.IsActive, assignedUserID, c.Visibility)
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

func (r *ConfigRepository) DeleteGrafana(ctx context.Context, id string, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if !domain.IsAdminRole(userRole) && userID > 0 {
		var ownerID *int
		errCheck := pool.QueryRow(ctx, "SELECT user_id FROM grafana_configs WHERE id = $1", id).Scan(&ownerID)
		if errCheck != nil {
			return fmt.Errorf("configuration not found")
		}
		if ownerID == nil || *ownerID != userID {
			var perm string
			errShare := pool.QueryRow(ctx, "SELECT permission FROM grafana_shares WHERE config_id = $1 AND user_id = $2", id, userID).Scan(&perm)
			if errShare != nil || perm != "manage" {
				return fmt.Errorf("you do not have permission to delete this configuration")
			}
		}
	}

	_, err = pool.Exec(ctx, `DELETE FROM grafana_configs WHERE id = $1`, id)
	return err
}

func (r *ConfigRepository) GetGrafanaByID(ctx context.Context, id string) (*domain.GrafanaConfig, error) {
	if err := RequireConnectionAccess(ctx, "grafana_configs", "grafana_shares", "config_id", id, "read"); err != nil {
		return nil, err
	}
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.GrafanaConfig
	err = pool.QueryRow(ctx, `SELECT id, name, host, token, datasource_uid, is_active, created_at, user_id, COALESCE(visibility, 'private') FROM grafana_configs WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Host, &c.Token, &c.DatasourceUID, &c.IsActive, &c.CreatedAt, &c.UserID, &c.Visibility)
	if err != nil {
		return nil, err
	}
	c.Token, err = config.DecryptText(c.Token)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt Grafana token: %w", err)
	}
	return &c, nil
}

// Prometheus Configs
func (r *ConfigRepository) ListPrometheus(ctx context.Context, userID int, userRole string) ([]domain.PrometheusConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)
	var list []domain.PrometheusConfig

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		query := `
			SELECT c.id, c.name, c.mode, c.path, c.reload_url, c.ssh_host, c.ssh_port, c.ssh_user, c.ssh_auth, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM prometheus_shares WHERE config_id = c.id) AS shares_count
			FROM prometheus_configs c
			LEFT JOIN users u ON c.user_id = u.id
			ORDER BY c.name ASC
		`
		rows, err := pool.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.PrometheusConfig
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.IsActive, &c.CreatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount,
			); err != nil {
				return nil, err
			}
			c.IsOwner = true
			c.UserPermission = "manage"
			list = append(list, c)
		}
	} else {
		query := `
			SELECT c.id, c.name, c.mode, c.path, c.reload_url, c.ssh_host, c.ssh_port, c.ssh_user, c.ssh_auth, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM prometheus_shares WHERE config_id = c.id) AS shares_count,
			       COALESCE(ps.permission, '') AS share_perm
			FROM prometheus_configs c
			LEFT JOIN users u ON c.user_id = u.id
			LEFT JOIN prometheus_shares ps ON c.id = ps.config_id AND ps.user_id = $1
			WHERE c.visibility = 'public'
			   OR c.user_id = $1
			   OR ps.user_id = $1
			ORDER BY c.name ASC
		`
		rows, err := pool.Query(ctx, query, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.PrometheusConfig
			var sharePerm string
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.IsActive, &c.CreatedAt,
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
			list = append(list, c)
		}
	}

	if list == nil {
		list = []domain.PrometheusConfig{}
	}
	return list, nil
}

func (r *ConfigRepository) GetPrometheusByID(ctx context.Context, id string) (*domain.PrometheusConfig, error) {
	if err := RequireConnectionAccess(ctx, "prometheus_configs", "prometheus_shares", "config_id", id, "read"); err != nil {
		return nil, err
	}
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.PrometheusConfig
	err = pool.QueryRow(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at, user_id, COALESCE(visibility, 'private') FROM prometheus_configs WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt, &c.UserID, &c.Visibility)
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
	err = pool.QueryRow(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
		FROM prometheus_configs 
		WHERE is_active = true 
		  AND LOWER(name) NOT LIKE '%data prepper%' 
		  AND LOWER(name) NOT LIKE '%dataprepper%' 
		  AND LOWER(path) NOT LIKE '%pipeline%'
		LIMIT 1`).
		Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
	if err != nil {
		// Fallback to any prometheus config that is not data prepper
		err = pool.QueryRow(ctx, `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
			FROM prometheus_configs 
			WHERE LOWER(name) NOT LIKE '%data prepper%' 
			  AND LOWER(name) NOT LIKE '%dataprepper%' 
			  AND LOWER(path) NOT LIKE '%pipeline%'
			ORDER BY is_active DESC, created_at DESC 
			LIMIT 1`).
			Scan(&c.ID, &c.Name, &c.Mode, &c.Path, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
	}
	if err := RequireConnectionAccess(ctx, "prometheus_configs", "prometheus_shares", "config_id", c.ID, "read"); err != nil {
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

func (r *ConfigRepository) SavePrometheus(ctx context.Context, c domain.PrometheusConfig, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	isAdmin := domain.IsAdminRole(userRole)
	// Local profiles grant access to the engine's filesystem, not the caller's
	// machine. Only administrators may introduce/retarget local file endpoints.
	isLocal := !strings.EqualFold(c.Mode, "ssh") && (c.SSHHost == nil || strings.TrimSpace(*c.SSHHost) == "")
	if !isAdmin && isLocal {
		existing, err := r.GetPrometheusByID(ctx, c.ID)
		if err != nil || existing == nil || existing.Path != c.Path || !strings.EqualFold(existing.Mode, c.Mode) || (existing.SSHHost != nil && *existing.SSHHost != "") {
			return fmt.Errorf("only administrators may create or change local integration file endpoints")
		}
	}

	// Check existing
	if c.ID != "" {
		var existingOwner *int
		errExist := pool.QueryRow(ctx, "SELECT user_id FROM prometheus_configs WHERE id = $1", c.ID).Scan(&existingOwner)
		if errExist == nil && !isAdmin && userID > 0 {
			if existingOwner == nil || *existingOwner != userID {
				var perm string
				errShare := pool.QueryRow(ctx, "SELECT permission FROM prometheus_shares WHERE config_id = $1 AND user_id = $2", c.ID, userID).Scan(&perm)
				if errShare != nil || perm != "manage" {
					return fmt.Errorf("you do not have permission to edit this configuration")
				}
			}
		}
	}

	var encPwd, encKey *string
	if c.SSHPassword != nil {
		cleanPwd := strings.TrimSpace(*c.SSHPassword)
		if cleanPwd != "" && cleanPwd != "********" && cleanPwd != "••••••" {
			if enc, err := config.EncryptText(cleanPwd); err == nil {
				encPwd = &enc
			}
		}
	}
	if c.SSHKey != nil {
		cleanKey := strings.TrimSpace(*c.SSHKey)
		if cleanKey != "" && cleanKey != "********" && cleanKey != "••••••" {
			if enc, err := config.EncryptText(cleanKey); err == nil {
				encKey = &enc
			}
		}
	}

	var assignedUserID *int
	if userID > 0 {
		assignedUserID = &userID
	}
	if c.Visibility == "" {
		c.Visibility = "private"
	}

	query := `INSERT INTO prometheus_configs (id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, user_id, visibility)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, mode = EXCLUDED.mode, path = EXCLUDED.path, reload_url = EXCLUDED.reload_url,
                ssh_host = EXCLUDED.ssh_host, ssh_port = EXCLUDED.ssh_port, ssh_user = EXCLUDED.ssh_user,
                ssh_auth = EXCLUDED.ssh_auth,
                ssh_password = CASE WHEN EXCLUDED.ssh_password IS NOT NULL THEN EXCLUDED.ssh_password ELSE prometheus_configs.ssh_password END,
                ssh_key = CASE WHEN EXCLUDED.ssh_key IS NOT NULL THEN EXCLUDED.ssh_key ELSE prometheus_configs.ssh_key END,
                is_active = EXCLUDED.is_active,
                visibility = COALESCE(NULLIF(EXCLUDED.visibility, ''), prometheus_configs.visibility)`
	_, err = pool.Exec(ctx, query, c.ID, c.Name, c.Mode, c.Path, c.ReloadURL, c.SSHHost, c.SSHPort, c.SSHUser, c.SSHAuth, encPwd, encKey, c.IsActive, assignedUserID, c.Visibility)
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

func (r *ConfigRepository) DeletePrometheus(ctx context.Context, id string, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if !domain.IsAdminRole(userRole) && userID > 0 {
		var ownerID *int
		errCheck := pool.QueryRow(ctx, "SELECT user_id FROM prometheus_configs WHERE id = $1", id).Scan(&ownerID)
		if errCheck != nil {
			return fmt.Errorf("configuration not found")
		}
		if ownerID == nil || *ownerID != userID {
			var perm string
			errShare := pool.QueryRow(ctx, "SELECT permission FROM prometheus_shares WHERE config_id = $1 AND user_id = $2", id, userID).Scan(&perm)
			if errShare != nil || perm != "manage" {
				return fmt.Errorf("you do not have permission to delete this configuration")
			}
		}
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
