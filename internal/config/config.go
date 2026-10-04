package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Port           int      `json:"port"`
	Env            string               `json:"env"`
	DataDir        string               `json:"dataDir"`
	LogsDir        string               `json:"logsDir"`
	AllowedOrigins []string             `json:"allowedOrigins"`
	DB             DBConfig             `json:"db"`
	Threads        ServiceThreadsConfig `json:"threads"`
	mu             sync.RWMutex
}

type ServiceThreadsConfig struct {
	ICMPPing     int `json:"icmpPing"`     // Default: 20
	OpenSearch   int `json:"openSearch"`   // Default: 5
	Backup       int `json:"backup"`       // Default: 2
	SNMP         int `json:"snmp"`         // Default: 4
	Discovery    int `json:"discovery"`    // Default: 5
	Cron         int `json:"cron"`         // Default: 4
	Alert        int `json:"alert"`        // Default: 4
	Prometheus   int `json:"prometheus"`   // Default: 8
	SSHTelemetry int `json:"sshTelemetry"` // Default: 10
	WorkerPool   int `json:"workerPool"`   // Default: 10
	Grok         int `json:"grok"`         // Default: 4
	DataPrepper  int `json:"dataPrepper"`  // Default: 2
	IPAM         int `json:"ipam"`         // Default: 30
}

func DefaultServiceThreads() ServiceThreadsConfig {
	return ServiceThreadsConfig{
		ICMPPing:     20,
		OpenSearch:   5,
		Backup:       2,
		SNMP:         4,
		Discovery:    5,
		Cron:         4,
		Alert:        4,
		Prometheus:   8,
		SSHTelemetry: 10,
		WorkerPool:   10,
		Grok:         4,
		DataPrepper:  2,
		IPAM:         30,
	}
}

type DBConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password"`
	Database        string `json:"database"`
	SSL             bool   `json:"ssl"`
	MaxConns        int    `json:"maxConns"`        // Maximum active connections in pool (default: 25)
	MinConns        int    `json:"minConns"`        // Minimum idle connections in pool (default: 5)
	MaxConnIdleTime int    `json:"maxConnIdleTime"` // Max idle duration before closing in seconds (default: 600)
	MaxConnLifetime int    `json:"maxConnLifetime"` // Max connection lifetime in seconds (default: 3600)
}

func (c *DBConfig) ConnString() string {
	sslMode := "disable"
	if c.SSL {
		sslMode = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, sslMode)
}

func (c *DBConfig) AdminConnString() string {
	sslMode := "disable"
	if c.SSL {
		sslMode = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, sslMode)
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// LoadConfig reads configurations from environment variables and db_config.json
func LoadConfig() *Config {
	configOnce.Do(func() {
		dataDir := getEnv("DATA_DIR", "data")
		logsDir := getEnv("LOGS_DIR", "logs")

		_ = os.MkdirAll(dataDir, 0755)
		_ = os.MkdirAll(logsDir, 0755)

		port, _ := strconv.Atoi(getEnv("PORT", "5000"))
		if port <= 0 {
			port = 5000
		}

		dbHost := getEnv("DB_HOST", getEnv("PGHOST", "database"))
		dbPortStr := getEnv("DB_PORT", getEnv("PGPORT", "5432"))
		dbPort, _ := strconv.Atoi(dbPortStr)
		if dbPort <= 0 {
			dbPort = 5432
		}

		dbUser := getEnv("DB_USER", getEnv("PGUSER", "hephaestus"))
		dbPassword := getEnv("DB_PASSWORD", getEnv("PGPASSWORD", "hephaestus_secret"))
		dbName := getEnv("DB_NAME", getEnv("PGDATABASE", "hephaestus"))
		dbSSL := getEnv("DB_SSL", getEnv("PGSSL", "false")) == "true"

		maxConns, _ := strconv.Atoi(getEnv("DB_MAX_CONNS", "25"))
		if maxConns <= 0 {
			maxConns = 25
		}
		minConns, _ := strconv.Atoi(getEnv("DB_MIN_CONNS", "5"))
		if minConns < 0 {
			minConns = 5
		}
		maxConnIdleTime, _ := strconv.Atoi(getEnv("DB_MAX_CONN_IDLE_TIME", "600"))
		if maxConnIdleTime <= 0 {
			maxConnIdleTime = 600
		}
		maxConnLifetime, _ := strconv.Atoi(getEnv("DB_MAX_CONN_LIFETIME", "3600"))
		if maxConnLifetime <= 0 {
			maxConnLifetime = 3600
		}

		dbCfg := DBConfig{
			Host:            dbHost,
			Port:            dbPort,
			User:            dbUser,
			Password:        dbPassword,
			Database:        dbName,
			SSL:             dbSSL,
			MaxConns:        maxConns,
			MinConns:        minConns,
			MaxConnIdleTime: maxConnIdleTime,
			MaxConnLifetime: maxConnLifetime,
		}

		// Try loading saved db_config.json if available
		dbConfigFile := filepath.Join(dataDir, "db_config.json")
		if rawData, err := os.ReadFile(dbConfigFile); err == nil {
			var saved map[string]interface{}
			if err := json.Unmarshal(rawData, &saved); err == nil {
				if h, ok := saved["host"].(string); ok && h != "" {
					dbCfg.Host = h
				}
				if p, ok := saved["port"].(float64); ok && p > 0 {
					dbCfg.Port = int(p)
				}
				if u, ok := saved["user"].(string); ok && u != "" {
					dbCfg.User = u
				}
				if d, ok := saved["database"].(string); ok && d != "" {
					dbCfg.Database = d
				}
				if s, ok := saved["ssl"].(bool); ok {
					dbCfg.SSL = s
				}
				if mc, ok := saved["maxConns"].(float64); ok && mc > 0 {
					dbCfg.MaxConns = int(mc)
				}
				if mic, ok := saved["minConns"].(float64); ok && mic >= 0 {
					dbCfg.MinConns = int(mic)
				}
				if mcit, ok := saved["maxConnIdleTime"].(float64); ok && mcit > 0 {
					dbCfg.MaxConnIdleTime = int(mcit)
				}
				if mclt, ok := saved["maxConnLifetime"].(float64); ok && mclt > 0 {
					dbCfg.MaxConnLifetime = int(mclt)
				}
				if pwd, ok := saved["password"].(string); ok && pwd != "" {
					if isEncrypted, ok := saved["encrypted"].(bool); ok && isEncrypted {
						if decrypted, err := DecryptText(pwd); err == nil {
							dbCfg.Password = decrypted
						}
					} else {
						dbCfg.Password = pwd
					}
				}
			}
		}

		// Try loading saved service_threads.json if available
		threadsCfg := DefaultServiceThreads()
		threadsConfigFile := filepath.Join(dataDir, "service_threads.json")
		if rawThreads, err := os.ReadFile(threadsConfigFile); err == nil {
			_ = json.Unmarshal(rawThreads, &threadsCfg)
		}
		// Validate sane values
		defaults := DefaultServiceThreads()
		if threadsCfg.ICMPPing <= 0 { threadsCfg.ICMPPing = defaults.ICMPPing }
		if threadsCfg.OpenSearch <= 0 { threadsCfg.OpenSearch = defaults.OpenSearch }
		if threadsCfg.Backup <= 0 { threadsCfg.Backup = defaults.Backup }
		if threadsCfg.SNMP <= 0 { threadsCfg.SNMP = defaults.SNMP }
		if threadsCfg.Discovery <= 0 { threadsCfg.Discovery = defaults.Discovery }
		if threadsCfg.Cron <= 0 { threadsCfg.Cron = defaults.Cron }
		if threadsCfg.Alert <= 0 { threadsCfg.Alert = defaults.Alert }
		if threadsCfg.Prometheus <= 0 { threadsCfg.Prometheus = defaults.Prometheus }
		if threadsCfg.SSHTelemetry <= 0 { threadsCfg.SSHTelemetry = defaults.SSHTelemetry }
		if threadsCfg.WorkerPool <= 0 { threadsCfg.WorkerPool = defaults.WorkerPool }
		if threadsCfg.Grok <= 0 { threadsCfg.Grok = defaults.Grok }
		if threadsCfg.DataPrepper <= 0 { threadsCfg.DataPrepper = defaults.DataPrepper }
		if threadsCfg.IPAM <= 0 { threadsCfg.IPAM = defaults.IPAM }

		originsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:5000,http://localhost:5173,http://localhost:3000")
		origins := strings.Split(originsStr, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}

		encKey := getEnv("APP_ENCRYPTION_KEY", getEnv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
		SetSecretKey(encKey)

		globalConfig = &Config{
			Port:           port,
			Env:            getEnv("APP_ENV", getEnv("NODE_ENV", "production")),
			DataDir:        dataDir,
			LogsDir:        logsDir,
			AllowedOrigins: origins,
			DB:             dbCfg,
			Threads:        threadsCfg,
		}
	})

	return globalConfig
}

// GetConfig returns the singleton config instance
func GetConfig() *Config {
	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}

// GetServiceThreads returns active thread concurrency configuration
func (c *Config) GetServiceThreads() ServiceThreadsConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Threads
}

// UpdateServiceThreads updates the service threads config in-memory and saves to data/service_threads.json
func (c *Config) UpdateServiceThreads(newThreads ServiceThreadsConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	defaults := DefaultServiceThreads()
	if newThreads.ICMPPing <= 0 { newThreads.ICMPPing = defaults.ICMPPing }
	if newThreads.OpenSearch <= 0 { newThreads.OpenSearch = defaults.OpenSearch }
	if newThreads.Backup <= 0 { newThreads.Backup = defaults.Backup }
	if newThreads.SNMP <= 0 { newThreads.SNMP = defaults.SNMP }
	if newThreads.Discovery <= 0 { newThreads.Discovery = defaults.Discovery }
	if newThreads.Cron <= 0 { newThreads.Cron = defaults.Cron }
	if newThreads.Alert <= 0 { newThreads.Alert = defaults.Alert }
	if newThreads.Prometheus <= 0 { newThreads.Prometheus = defaults.Prometheus }
	if newThreads.SSHTelemetry <= 0 { newThreads.SSHTelemetry = defaults.SSHTelemetry }
	if newThreads.WorkerPool <= 0 { newThreads.WorkerPool = defaults.WorkerPool }
	if newThreads.Grok <= 0 { newThreads.Grok = defaults.Grok }
	if newThreads.DataPrepper <= 0 { newThreads.DataPrepper = defaults.DataPrepper }
	if newThreads.IPAM <= 0 { newThreads.IPAM = defaults.IPAM }

	c.Threads = newThreads

	raw, err := json.MarshalIndent(newThreads, "", "  ")
	if err != nil {
		return err
	}

	threadsConfigFile := filepath.Join(c.DataDir, "service_threads.json")
	return os.WriteFile(threadsConfigFile, raw, 0644)
}

// UpdateDBConfig updates the database config in-memory and writes it encrypted to data/db_config.json
func (c *Config) UpdateDBConfig(newDB DBConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.DB = newDB

	encryptedPassword, err := EncryptText(newDB.Password)
	if err != nil {
		encryptedPassword = newDB.Password
	}

	if newDB.MaxConns <= 0 {
		newDB.MaxConns = 25
	}
	if newDB.MinConns < 0 {
		newDB.MinConns = 5
	}
	if newDB.MaxConnIdleTime <= 0 {
		newDB.MaxConnIdleTime = 600
	}
	if newDB.MaxConnLifetime <= 0 {
		newDB.MaxConnLifetime = 3600
	}

	payload := map[string]interface{}{
		"host":            newDB.Host,
		"port":            newDB.Port,
		"user":            newDB.User,
		"password":        encryptedPassword,
		"database":        newDB.Database,
		"ssl":             newDB.SSL,
		"maxConns":        newDB.MaxConns,
		"minConns":        newDB.MinConns,
		"maxConnIdleTime": newDB.MaxConnIdleTime,
		"maxConnLifetime": newDB.MaxConnLifetime,
		"encrypted":       true,
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	dbConfigFile := filepath.Join(c.DataDir, "db_config.json")
	return os.WriteFile(dbConfigFile, raw, 0644)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}
