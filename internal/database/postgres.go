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

	poolConfig.MaxConns = 15
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 1 * time.Hour

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
		return err
	}

	// Dynamic incremental updates for existing databases
	upgradeSQL := `
		ALTER TABLE system_roles ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '{}'::jsonb;
		INSERT INTO system_roles (name, description, is_default, permissions) VALUES 
			('ADMIN', 'Full system administrator with unrestricted access', true, '{"*": "manage"}'::jsonb),
			('OPERATOR', 'Operational user with read and manage access to monitoring, servers, and network', true, '{"dashboard": "manage", "remote_servers": "manage", "network_topology": "manage", "backup": "read", "connections": "read", "snmp": "manage", "opensearch": "read", "grok_debugger": "read", "dataprepper_config": "read", "prometheus_config": "read", "slideshow": "read", "settings": "read"}'::jsonb),
			('VIEWER', 'Read-only observer access across all monitoring and telemetry views', true, '{"dashboard": "read", "remote_servers": "read", "network_topology": "read", "backup": "read", "connections": "read", "snmp": "read", "opensearch": "read", "grok_debugger": "read", "dataprepper_config": "read", "prometheus_config": "read", "slideshow": "read", "settings": "none"}'::jsonb)
		ON CONFLICT (name) DO UPDATE SET 
			permissions = EXCLUDED.permissions,
			description = EXCLUDED.description;

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
	`
	_, _ = pool.Exec(ctx, upgradeSQL)

	return nil
}
