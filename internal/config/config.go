package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Port           int      `json:"port"`
	Env            string   `json:"env"`
	DataDir        string   `json:"dataDir"`
	LogsDir        string   `json:"logsDir"`
	AllowedOrigins []string `json:"allowedOrigins"`
	DB             DBConfig `json:"db"`
	mu             sync.RWMutex
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSL      bool   `json:"ssl"`
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

		dbPort, _ := strconv.Atoi(getEnv("PGPORT", "5432"))
		if dbPort <= 0 {
			dbPort = 5432
		}

		dbSSL := getEnv("PGSSL", "false") == "true"

		dbCfg := DBConfig{
			Host:     getEnv("PGHOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("PGUSER", "postgres"),
			Password: getEnv("PGPASSWORD", "postgres"),
			Database: getEnv("PGDATABASE", "hephaestus"),
			SSL:      dbSSL,
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

		originsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:5000,http://localhost:5173,http://localhost:3000")
		origins := strings.Split(originsStr, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}

		encKey := getEnv("ENCRYPTION_KEY", "hephaestus-super-secure-key-32b!")
		SetSecretKey(encKey)

		globalConfig = &Config{
			Port:           port,
			Env:            getEnv("NODE_ENV", "production"),
			DataDir:        dataDir,
			LogsDir:        logsDir,
			AllowedOrigins: origins,
			DB:             dbCfg,
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

// UpdateDBConfig updates the database config in-memory and writes it encrypted to data/db_config.json
func (c *Config) UpdateDBConfig(newDB DBConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.DB = newDB

	encryptedPassword, err := EncryptText(newDB.Password)
	if err != nil {
		encryptedPassword = newDB.Password
	}

	payload := map[string]interface{}{
		"host":      newDB.Host,
		"port":      newDB.Port,
		"user":      newDB.User,
		"password":  encryptedPassword,
		"database":  newDB.Database,
		"ssl":       newDB.SSL,
		"encrypted": true,
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
