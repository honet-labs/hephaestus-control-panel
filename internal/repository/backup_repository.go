package repository

import (
	"context"
	"encoding/json"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type BackupRepository struct{}

func NewBackupRepository() *BackupRepository {
	return &BackupRepository{}
}

// Database Configs
func (r *BackupRepository) ListDBConfigs(ctx context.Context) ([]domain.BackupDbConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, db_type, host, port, username, database_name, ssh_host, ssh_port, ssh_user, ssh_auth, created_at FROM backup_database_configs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.BackupDbConfig
	for rows.Next() {
		var c domain.BackupDbConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.DBType, &c.Host, &c.Port, &c.Username, &c.DatabaseName, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Password = "********"
		list = append(list, c)
	}
	return list, nil
}

func (r *BackupRepository) GetRawDBConfig(ctx context.Context, id string) (*domain.BackupDbConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var c domain.BackupDbConfig
	err = pool.QueryRow(ctx, `SELECT id, name, db_type, host, port, username, password, database_name, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, created_at FROM backup_database_configs WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.DBType, &c.Host, &c.Port, &c.Username, &c.Password, &c.DatabaseName, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.CreatedAt)
	if err != nil {
		return nil, err
	}

	if dec, err := config.DecryptText(c.Password); err == nil {
		c.Password = dec
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

func (r *BackupRepository) SaveDBConfig(ctx context.Context, c domain.BackupDbConfig) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	encPassword := c.Password
	if c.Password != "" && c.Password != "********" {
		if enc, err := config.EncryptText(c.Password); err == nil {
			encPassword = enc
		}
	}

	var encSSHPassword, encSSHKey *string
	if c.SSHPassword != nil && *c.SSHPassword != "" && *c.SSHPassword != "********" {
		if enc, err := config.EncryptText(*c.SSHPassword); err == nil {
			encSSHPassword = &enc
		}
	}
	if c.SSHKey != nil && *c.SSHKey != "" && *c.SSHKey != "********" {
		if enc, err := config.EncryptText(*c.SSHKey); err == nil {
			encSSHKey = &enc
		}
	}

	query := `INSERT INTO backup_database_configs (id, name, db_type, host, port, username, password, database_name, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, db_type = EXCLUDED.db_type, host = EXCLUDED.host, port = EXCLUDED.port,
                username = EXCLUDED.username,
                password = CASE WHEN $7 = '********' THEN backup_database_configs.password ELSE $7 END,
                database_name = EXCLUDED.database_name, ssh_host = EXCLUDED.ssh_host, ssh_port = EXCLUDED.ssh_port,
                ssh_user = EXCLUDED.ssh_user, ssh_auth = EXCLUDED.ssh_auth,
                ssh_password = CASE WHEN $13 IS NULL OR $13 = '********' THEN backup_database_configs.ssh_password ELSE $13 END,
                ssh_key = CASE WHEN $14 IS NULL OR $14 = '********' THEN backup_database_configs.ssh_key ELSE $14 END`

	_, err = pool.Exec(ctx, query, c.ID, c.Name, c.DBType, c.Host, c.Port, c.Username, encPassword, c.DatabaseName, c.SSHHost, c.SSHPort, c.SSHUser, c.SSHAuth, encSSHPassword, encSSHKey)
	return err
}

func (r *BackupRepository) DeleteDBConfig(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM backup_database_configs WHERE id = $1`, id)
	return err
}

// Destinations
func (r *BackupRepository) ListDestinations(ctx context.Context) ([]domain.BackupDestination, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, dest_type, config, is_active, created_at FROM backup_destinations ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.BackupDestination
	for rows.Next() {
		var d domain.BackupDestination
		var cfgRaw []byte
		if err := rows.Scan(&d.ID, &d.Name, &d.DestType, &cfgRaw, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfgRaw, &d.Config)
		// Mask secrets in response
		if _, ok := d.Config["secretAccessKey"]; ok {
			d.Config["secretAccessKey"] = "********"
		}
		if _, ok := d.Config["password"]; ok {
			d.Config["password"] = "********"
		}
		list = append(list, d)
	}
	return list, nil
}

func (r *BackupRepository) GetRawDestination(ctx context.Context, id string) (*domain.BackupDestination, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var d domain.BackupDestination
	var cfgRaw []byte
	err = pool.QueryRow(ctx, `SELECT id, name, dest_type, config, is_active, created_at FROM backup_destinations WHERE id = $1`, id).
		Scan(&d.ID, &d.Name, &d.DestType, &cfgRaw, &d.IsActive, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(cfgRaw, &d.Config)

	// Decrypt sensitive fields
	if sec, ok := d.Config["secretAccessKey"].(string); ok && sec != "" {
		if dec, err := config.DecryptText(sec); err == nil {
			d.Config["secretAccessKey"] = dec
		}
	}
	if pwd, ok := d.Config["password"].(string); ok && pwd != "" {
		if dec, err := config.DecryptText(pwd); err == nil {
			d.Config["password"] = dec
		}
	}

	return &d, nil
}

func (r *BackupRepository) SaveDestination(ctx context.Context, d domain.BackupDestination) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	safeConfig := make(map[string]interface{})
	for k, v := range d.Config {
		safeConfig[k] = v
	}

	if sec, ok := safeConfig["secretAccessKey"].(string); ok && sec != "" && sec != "********" {
		if enc, err := config.EncryptText(sec); err == nil {
			safeConfig["secretAccessKey"] = enc
		}
	}
	if pwd, ok := safeConfig["password"].(string); ok && pwd != "" && pwd != "********" {
		if enc, err := config.EncryptText(pwd); err == nil {
			safeConfig["password"] = enc
		}
	}

	cfgJSON, _ := json.Marshal(safeConfig)
	query := `INSERT INTO backup_destinations (id, name, dest_type, config, is_active)
              VALUES ($1, $2, $3, $4, $5)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, dest_type = EXCLUDED.dest_type, config = EXCLUDED.config, is_active = EXCLUDED.is_active`
	_, err = pool.Exec(ctx, query, d.ID, d.Name, d.DestType, cfgJSON, d.IsActive)
	return err
}

func (r *BackupRepository) DeleteDestination(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM backup_destinations WHERE id = $1`, id)
	return err
}

// Schedules
func (r *BackupRepository) ListSchedules(ctx context.Context) ([]domain.BackupSchedule, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, db_config_id, destination_id, cron_expression, is_active, last_run, next_run, created_at FROM backup_schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.BackupSchedule
	for rows.Next() {
		var s domain.BackupSchedule
		if err := rows.Scan(&s.ID, &s.Name, &s.DBConfigID, &s.DestinationID, &s.CronExpression, &s.IsActive, &s.LastRun, &s.NextRun, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func (r *BackupRepository) SaveSchedule(ctx context.Context, s domain.BackupSchedule) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	query := `INSERT INTO backup_schedules (id, name, db_config_id, destination_id, cron_expression, is_active)
              VALUES ($1, $2, $3, $4, $5, $6)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, db_config_id = EXCLUDED.db_config_id, destination_id = EXCLUDED.destination_id,
                cron_expression = EXCLUDED.cron_expression, is_active = EXCLUDED.is_active`
	_, err = pool.Exec(ctx, query, s.ID, s.Name, s.DBConfigID, s.DestinationID, s.CronExpression, s.IsActive)
	return err
}

func (r *BackupRepository) UpdateScheduleRuns(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE backup_schedules SET last_run = NOW() WHERE id = $1`, id)
	return err
}

func (r *BackupRepository) DeleteSchedule(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM backup_schedules WHERE id = $1`, id)
	return err
}

// History
func (r *BackupRepository) ListHistory(ctx context.Context, limit, offset int) ([]domain.BackupHistoryEntry, int, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, 0, err
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM backup_history`).Scan(&count)

	rows, err := pool.Query(ctx, `SELECT id, db_config_id, destination_id, db_name, db_type, dest_type, filename, file_size, status, error_message, started_at, completed_at FROM backup_history ORDER BY started_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.BackupHistoryEntry
	for rows.Next() {
		var h domain.BackupHistoryEntry
		if err := rows.Scan(&h.ID, &h.DBConfigID, &h.DestinationID, &h.DBName, &h.DBType, &h.DestType, &h.Filename, &h.FileSize, &h.Status, &h.ErrorMessage, &h.StartedAt, &h.CompletedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, h)
	}
	return list, count, nil
}

func (r *BackupRepository) CreateHistory(ctx context.Context, h domain.BackupHistoryEntry) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	query := `INSERT INTO backup_history (id, db_config_id, destination_id, db_name, db_type, dest_type, filename, file_size, status, started_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'running', NOW())`
	_, err = pool.Exec(ctx, query, h.ID, h.DBConfigID, h.DestinationID, h.DBName, h.DBType, h.DestType, h.Filename, h.FileSize)
	return err
}

func (r *BackupRepository) UpdateHistoryStatus(ctx context.Context, id, status string, fileSize int64, errMsg *string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	query := `UPDATE backup_history SET status = $1, file_size = $2, error_message = $3, completed_at = NOW() WHERE id = $4`
	_, err = pool.Exec(ctx, query, status, fileSize, errMsg, id)
	return err
}

func (r *BackupRepository) DeleteHistory(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM backup_history WHERE id = $1`, id)
	return err
}
