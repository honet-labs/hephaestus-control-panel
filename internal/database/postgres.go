package database

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/000001_init_schema.sql
var schemaSQL string

var (
	activePool   *pgxpool.Pool
	poolLock     sync.RWMutex
	isConnected  bool
	connErr      error
	isConfigured bool
)

// InitDatabase initializes the PostgreSQL connection pool and runs database schema migrations
func InitDatabase(ctx context.Context, cfg *config.Config) error {
	poolLock.Lock()
	defer poolLock.Unlock()

	if activePool != nil {
		activePool.Close()
		activePool = nil
	}

	isConnected = false
	connErr = nil

	// Step 1: Ensure target database exists
	if err := ensureDatabaseExists(ctx, cfg.DB); err != nil {
		logger.Warn("Database", fmt.Sprintf("Failed to auto-verify database existence: %v", err))
	}

	// Step 2: Connect to the target database
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.ConnString())
	if err != nil {
		connErr = err
		logger.Error("Database", "Failed to parse database connection string", err)
		return err
	}

	maxConns := cfg.DB.MaxConns
	if maxConns <= 0 {
		maxConns = 10
	}
	minConns := cfg.DB.MinConns
	if minConns < 0 {
		minConns = 2
	}
	if minConns > maxConns {
		minConns = maxConns
	}

	idleTime := time.Duration(cfg.DB.MaxConnIdleTime) * time.Second
	if idleTime <= 0 {
		idleTime = 5 * time.Minute
	}

	lifetime := time.Duration(cfg.DB.MaxConnLifetime) * time.Second
	if lifetime <= 0 {
		lifetime = 1 * time.Hour
	}

	poolConfig.MaxConns = int32(maxConns)
	poolConfig.MinConns = int32(minConns)
	poolConfig.MaxConnIdleTime = idleTime
	poolConfig.MaxConnLifetime = lifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		connErr = err
		logger.Error("Database", "Failed to initialize pgxpool", err)
		return err
	}

	// Step 3: Test connection with ping
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		connErr = err
		pool.Close()
		logger.Warn("Database", fmt.Sprintf("PostgreSQL connection failed: %v. Running in Setup Mode.", err))
		return err
	}

	activePool = pool
	isConnected = true
	isConfigured = true

	logger.Info("Database", fmt.Sprintf("Connected to PostgreSQL at %s:%d/%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Database))

	// Step 4: Execute database schema migrations
	if err := runMigrations(ctx, pool); err != nil {
		logger.Error("Database", "Schema migration failed", err)
		return err
	}

	logger.Info("Database", "Database schema and tables synchronized successfully.")
	return nil
}

// GetPool returns the active pgxpool instance
func GetPool() (*pgxpool.Pool, error) {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if !isConnected || activePool == nil {
		return nil, fmt.Errorf("database is not connected")
	}
	return activePool, nil
}

// GetDB returns the active pgxpool instance (or nil if not connected)
func GetDB() *pgxpool.Pool {
	poolLock.RLock()
	defer poolLock.RUnlock()
	return activePool
}

// IsConnected returns whether the database connection is active
func IsConnected() bool {
	poolLock.RLock()
	defer poolLock.RUnlock()
	return isConnected
}

// ensureDatabaseExists tries connecting to the default 'postgres' db and creates the target DB if missing
func ensureDatabaseExists(ctx context.Context, dbCfg config.DBConfig) error {
	adminConn, err := pgx.Connect(ctx, dbCfg.AdminConnString())
	if err != nil {
		return err
	}
	defer adminConn.Close(ctx)

	var exists bool
	err = adminConn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbCfg.Database).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		logger.Info("Database", fmt.Sprintf("Database '%s' does not exist. Creating automatically...", dbCfg.Database))
		// Note: CREATE DATABASE cannot run inside a transaction or prepared statement with parameters
		_, err = adminConn.Exec(ctx, fmt.Sprintf("CREATE DATABASE \"%s\"", dbCfg.Database))
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		logger.Info("Database", fmt.Sprintf("Database '%s' created successfully!", dbCfg.Database))
	}

	return nil
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Pre-migration upgrades: ensure existing tables receive new columns before schemaSQL seeds them
	preUpgradeSQL := `
		DO $$ 
		BEGIN 
			IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'system_roles') THEN 
				ALTER TABLE system_roles ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '{}'::jsonb;
			END IF; 
			IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users') THEN 
				ALTER TABLE users ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN DEFAULT false;
			END IF; 
			IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'remote_host_configs') THEN 
				ALTER TABLE remote_host_configs ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
				ALTER TABLE remote_host_configs ADD COLUMN IF NOT EXISTS group_name VARCHAR(255) DEFAULT 'Default';
				ALTER TABLE remote_host_configs ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';
			END IF; 
			IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'activity_logs') THEN 
				ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
			END IF; 
			IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'topology_pending') THEN 
				ALTER TABLE topology_pending ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
			END IF; 
		END $$;
	`
	if _, err := pool.Exec(ctx, preUpgradeSQL); err != nil {
		logger.Warn("Database", fmt.Sprintf("Pre-migration upgrade warning: %v", err))
	}

	if schemaSQL == "" {
		return fmt.Errorf("schema migration SQL is empty")
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		logger.Warn("Database", fmt.Sprintf("Schema initial migration notice (continuing with incremental upgrades): %v", err))
	}

	// Dynamic incremental updates for existing databases
	upgradeSQL := `
		ALTER TABLE system_roles ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '{}'::jsonb;
		INSERT INTO system_roles (name, description, is_default, permissions) VALUES 
			('ADMIN', 'Full system administrator with unrestricted access', true, '{"*": "manage"}'::jsonb),
			('OPERATOR', 'Operational user with read and manage access to monitoring, servers, and network', true, '{"dashboard": "manage", "remote_servers": "manage", "network_topology": "manage", "backup": "manage", "connections": "manage", "snmp": "manage", "opensearch": "manage", "grok_debugger": "manage", "dataprepper_config": "manage", "prometheus_config": "manage", "opentelemetry_config": "manage", "slideshow": "manage", "settings": "manage"}'::jsonb),
			('VIEWER', 'Read-only observer access across all monitoring and telemetry views', true, '{"dashboard": "read", "remote_servers": "read", "network_topology": "read", "backup": "read", "connections": "read", "snmp": "read", "opensearch": "read", "grok_debugger": "read", "dataprepper_config": "read", "prometheus_config": "read", "opentelemetry_config": "read", "slideshow": "read", "settings": "none"}'::jsonb)
		ON CONFLICT (name) DO UPDATE SET 
			permissions = EXCLUDED.permissions,
			description = EXCLUDED.description;

		-- Ensure master admin accounts have full ADMIN role
		UPDATE users SET role = 'ADMIN' WHERE LOWER(username) IN ('admin', 'administrator', 'root') AND role != 'ADMIN';

		CREATE TABLE IF NOT EXISTS remote_host_firewall_rules (
			id VARCHAR(50) PRIMARY KEY,
			host_id VARCHAR(50) NOT NULL REFERENCES remote_host_configs(id) ON DELETE CASCADE,
			protocol VARCHAR(20) NOT NULL DEFAULT 'ALL',
			port_range VARCHAR(50) NOT NULL DEFAULT 'ALL',
			source_ip VARCHAR(50) NOT NULL DEFAULT '0.0.0.0/0',
			action VARCHAR(20) NOT NULL DEFAULT 'ALLOW',
			description TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_remote_host_firewall_rules_host_id ON remote_host_firewall_rules(host_id);

		ALTER TABLE remote_host_configs ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
		CREATE INDEX IF NOT EXISTS idx_remote_host_configs_user_id ON remote_host_configs(user_id);

		UPDATE remote_host_configs 
		SET user_id = (SELECT id FROM users WHERE role = 'ADMIN' ORDER BY id ASC LIMIT 1)
		WHERE user_id IS NULL AND EXISTS (SELECT 1 FROM users WHERE role = 'ADMIN');

		CREATE TABLE IF NOT EXISTS remote_host_shares (
			id VARCHAR(50) PRIMARY KEY,
			host_id VARCHAR(50) NOT NULL REFERENCES remote_host_configs(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			permission VARCHAR(20) NOT NULL DEFAULT 'read',
			shared_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(host_id, user_id)
		);
		CREATE INDEX IF NOT EXISTS idx_remote_host_shares_host_id ON remote_host_shares(host_id);
		CREATE INDEX IF NOT EXISTS idx_remote_host_shares_user_id ON remote_host_shares(user_id);

		INSERT INTO backup_destinations (id, name, dest_type, config, is_active)
		SELECT 'dest-local-default', 'Local Storage (Default)', 'local', '{"path": "/app/backups"}'::jsonb, true
		WHERE NOT EXISTS (SELECT 1 FROM backup_destinations WHERE id = 'dest-local-default' OR (dest_type = 'local' AND (config->>'path' = '/app/backups' OR config->>'path' = '/opt/backups')));

		ALTER TABLE backup_schedules ADD COLUMN IF NOT EXISTS db_config_ids TEXT[] DEFAULT '{}';
		ALTER TABLE backup_schedules ALTER COLUMN db_config_id DROP NOT NULL;

		CREATE TABLE IF NOT EXISTS opentelemetry_configs (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			tags TEXT[] DEFAULT '{}',
			ssh_host VARCHAR(255) NOT NULL,
			ssh_port INTEGER DEFAULT 22,
			ssh_user VARCHAR(255) NOT NULL DEFAULT 'root',
			ssh_auth VARCHAR(50) NOT NULL DEFAULT 'password',
			ssh_password TEXT,
			ssh_key TEXT,
			config_path VARCHAR(255) NOT NULL DEFAULT '/etc/otelcol-contrib/config.yaml',
			service_name VARCHAR(100) NOT NULL DEFAULT 'otelcol-contrib',
			reload_mode VARCHAR(50) NOT NULL DEFAULT 'restart',
			last_status VARCHAR(50) DEFAULT 'unknown',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS opentelemetry_config_history (
			id VARCHAR(50) PRIMARY KEY,
			otel_config_id VARCHAR(50) REFERENCES opentelemetry_configs(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			created_by VARCHAR(100),
			change_summary VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_otel_configs_ssh_host ON opentelemetry_configs(ssh_host);
		CREATE INDEX IF NOT EXISTS idx_otel_config_history_config_id ON opentelemetry_config_history(otel_config_id);

		CREATE TABLE IF NOT EXISTS vaultwarden_configs (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(255) NOT NULL DEFAULT 'Vaultwarden',
			server_url TEXT NOT NULL,
			email VARCHAR(255) NOT NULL,
			master_password_encrypted TEXT NOT NULL,
			is_active BOOLEAN DEFAULT true,
			last_synced_at TIMESTAMP WITH TIME ZONE,
			cached_ciphers JSONB DEFAULT '[]'::jsonb,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		UPDATE system_roles 
		SET permissions = permissions || '{"opentelemetry_config": "manage"}'::jsonb 
		WHERE name IN ('ADMIN', 'OPERATOR') AND NOT (permissions ? 'opentelemetry_config');

		UPDATE system_roles 
		SET permissions = permissions || '{"opentelemetry_config": "read"}'::jsonb 
		WHERE name = 'VIEWER' AND NOT (permissions ? 'opentelemetry_config');

		UPDATE system_roles 
		SET permissions = permissions || '{"security": "manage"}'::jsonb 
		WHERE name IN ('ADMIN', 'OPERATOR') AND NOT (permissions ? 'security');

		UPDATE system_roles 
		SET permissions = permissions || '{"security": "read"}'::jsonb 
		WHERE name = 'VIEWER' AND NOT (permissions ? 'security');

		CREATE TABLE IF NOT EXISTS docker_connections (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			host_type VARCHAR(50) NOT NULL DEFAULT 'local',
			socket_path VARCHAR(255) DEFAULT '/var/run/docker.sock',
			tcp_url VARCHAR(255),
			remote_host_id VARCHAR(50) REFERENCES remote_host_configs(id) ON DELETE SET NULL,
			ssh_host VARCHAR(255),
			ssh_port INTEGER DEFAULT 22,
			ssh_user VARCHAR(255),
			ssh_auth VARCHAR(50) DEFAULT 'password',
			ssh_password_encrypted TEXT,
			ssh_key_encrypted TEXT,
			is_active BOOLEAN DEFAULT true,
			is_default BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		INSERT INTO docker_connections (id, name, host_type, socket_path, tcp_url, is_active, is_default)
		SELECT 'docker-local-default', 'Local Docker Host', 'local', '/var/run/docker.sock', '', true, true
		WHERE NOT EXISTS (SELECT 1 FROM docker_connections WHERE id = 'docker-local-default' OR is_default = true);

		UPDATE system_roles 
		SET permissions = permissions || '{"infrastructure": "manage", "connections": "manage", "reports": "manage"}'::jsonb 
		WHERE name IN ('ADMIN', 'OPERATOR');

		CREATE TABLE IF NOT EXISTS docker_container_metadata (
			connection_id VARCHAR(50) NOT NULL,
			container_id VARCHAR(100) NOT NULL,
			container_name VARCHAR(255) NOT NULL,
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			username VARCHAR(100),
			visibility VARCHAR(20) NOT NULL DEFAULT 'public',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(connection_id, container_id)
		);
		CREATE INDEX IF NOT EXISTS idx_docker_container_meta_user ON docker_container_metadata(user_id);

		CREATE TABLE IF NOT EXISTS visual_reports (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT DEFAULT '',
			mode VARCHAR(50) NOT NULL DEFAULT 'document',
			page_orientation VARCHAR(20) NOT NULL DEFAULT 'portrait',
			header_config JSONB NOT NULL DEFAULT '{"title": "System Utilization Report", "subtitle": "", "showDate": true, "logoText": "HEPHAESTUS"}'::jsonb,
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS visual_report_widgets (
			id VARCHAR(50) PRIMARY KEY,
			report_id VARCHAR(50) NOT NULL REFERENCES visual_reports(id) ON DELETE CASCADE,
			page_number INTEGER NOT NULL DEFAULT 1,
			title VARCHAR(255) NOT NULL,
			chart_type VARCHAR(50) NOT NULL DEFAULT 'line',
			source_type VARCHAR(50) NOT NULL DEFAULT 'opensearch',
			source_config JSONB NOT NULL DEFAULT '{}'::jsonb,
			time_range VARCHAR(50) NOT NULL DEFAULT '24h',
			theme VARCHAR(50) DEFAULT 'default',
			width_percent INTEGER NOT NULL DEFAULT 50,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_visual_reports_user_id ON visual_reports(user_id);
		CREATE INDEX IF NOT EXISTS idx_visual_report_widgets_report_id ON visual_report_widgets(report_id);
		CREATE INDEX IF NOT EXISTS idx_visual_report_widgets_page ON visual_report_widgets(report_id, page_number);

		-- Status Pages (Halaman Status) upgrade
		CREATE TABLE IF NOT EXISTS status_pages (
			id VARCHAR(50) PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			slug VARCHAR(100) UNIQUE NOT NULL,
			description TEXT DEFAULT '',
			footer_text TEXT DEFAULT '',
			theme VARCHAR(20) DEFAULT 'auto',
			refresh_interval INTEGER DEFAULT 60,
			is_public BOOLEAN DEFAULT true,
			is_published BOOLEAN DEFAULT true,
			show_tags BOOLEAN DEFAULT true,
			custom_css TEXT DEFAULT '',
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS status_page_groups (
			id VARCHAR(50) PRIMARY KEY,
			page_id VARCHAR(50) NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			sort_order INTEGER DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS status_page_items (
			id VARCHAR(50) PRIMARY KEY,
			page_id VARCHAR(50) NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
			group_id VARCHAR(50) REFERENCES status_page_groups(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			source_type VARCHAR(50) NOT NULL,
			source_id VARCHAR(100),
			source_config JSONB NOT NULL DEFAULT '{}'::jsonb,
			description VARCHAR(255) DEFAULT '',
			sort_order INTEGER DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS status_page_incidents (
			id VARCHAR(50) PRIMARY KEY,
			page_id VARCHAR(50) NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'investigating',
			severity VARCHAR(50) NOT NULL DEFAULT 'minor',
			message TEXT NOT NULL,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_status_pages_slug ON status_pages(slug);
		CREATE INDEX IF NOT EXISTS idx_status_pages_is_public ON status_pages(is_public);
		CREATE INDEX IF NOT EXISTS idx_status_page_groups_page_id ON status_page_groups(page_id);
		CREATE INDEX IF NOT EXISTS idx_status_page_items_page_id ON status_page_items(page_id);
		CREATE INDEX IF NOT EXISTS idx_status_page_items_group_id ON status_page_items(group_id);
		CREATE INDEX IF NOT EXISTS idx_status_page_incidents_page_id ON status_page_incidents(page_id);

		UPDATE system_roles 
		SET permissions = permissions || '{"status_pages": "manage"}'::jsonb 
		WHERE name IN ('ADMIN', 'OPERATOR');

		UPDATE system_roles 
		SET permissions = permissions || '{"status_pages": "read"}'::jsonb 
		WHERE name = 'VIEWER';
	`
	if _, err := pool.Exec(ctx, upgradeSQL); err != nil {
		logger.Warn("Database", fmt.Sprintf("Incremental upgrades execution notice: %v", err))
	}

	return nil
}
