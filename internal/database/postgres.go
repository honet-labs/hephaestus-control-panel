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
	if schemaSQL == "" {
		return fmt.Errorf("schema migration SQL is empty")
	}
	_, err := pool.Exec(ctx, schemaSQL)
	return err
}
