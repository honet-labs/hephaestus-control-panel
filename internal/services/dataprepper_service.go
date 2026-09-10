package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"gopkg.in/yaml.v3"
)

type DataPrepperService struct {
	sshService *SSHService
	httpClient *http.Client
}

func NewDataPrepperService(sshService *SSHService) *DataPrepperService {
	return &DataPrepperService{
		sshService: sshService,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *DataPrepperService) GetActiveConfig(ctx context.Context, instanceID ...string) (*domain.DataPrepperConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	targetID := ""
	if len(instanceID) > 0 && instanceID[0] != "" {
		targetID = instanceID[0]
	}

	// 1. If specific ID requested, check dataprepper_configs first
	if targetID != "" {
		query := `SELECT id, name, mode, pipelines_dir, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
	              FROM dataprepper_configs WHERE id = $1 LIMIT 1`
		var c domain.DataPrepperConfig
		err = pool.QueryRow(ctx, query, targetID).Scan(&c.ID, &c.Name, &c.Mode, &c.PipelinesDir, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
		if err == nil {
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

		// Also check prometheus_configs by ID
		var p domain.PrometheusConfig
		pQuery := `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
		           FROM prometheus_configs WHERE id = $1 LIMIT 1`
		err = pool.QueryRow(ctx, pQuery, targetID).Scan(&p.ID, &p.Name, &p.Mode, &p.Path, &p.ReloadURL, &p.SSHHost, &p.SSHPort, &p.SSHUser, &p.SSHAuth, &p.SSHPassword, &p.SSHKey, &p.IsActive, &p.CreatedAt)
		if err == nil {
			return s.convertPrometheusToDP(&p), nil
		}
	}

	// 2. Query active from dataprepper_configs
	query := `SELECT id, name, mode, pipelines_dir, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
              FROM dataprepper_configs WHERE is_active = true LIMIT 1`
	var c domain.DataPrepperConfig
	err = pool.QueryRow(ctx, query).Scan(&c.ID, &c.Name, &c.Mode, &c.PipelinesDir, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.SSHPassword, &c.SSHKey, &c.IsActive, &c.CreatedAt)
	if err == nil {
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

	// 3. Fallback to prometheus_configs where name indicates Data Prepper
	var p domain.PrometheusConfig
	pQuery := `SELECT id, name, mode, path, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password, ssh_key, is_active, created_at 
	           FROM prometheus_configs 
	           WHERE LOWER(name) LIKE '%data prepper%' OR LOWER(name) LIKE '%dataprepper%' OR LOWER(path) LIKE '%pipeline%'
	           ORDER BY is_active DESC, created_at DESC LIMIT 1`
	err = pool.QueryRow(ctx, pQuery).Scan(&p.ID, &p.Name, &p.Mode, &p.Path, &p.ReloadURL, &p.SSHHost, &p.SSHPort, &p.SSHUser, &p.SSHAuth, &p.SSHPassword, &p.SSHKey, &p.IsActive, &p.CreatedAt)
	if err == nil {
		return s.convertPrometheusToDP(&p), nil
	}

	return nil, fmt.Errorf("no active Data Prepper configuration found in registry")
}

func (s *DataPrepperService) convertPrometheusToDP(p *domain.PrometheusConfig) *domain.DataPrepperConfig {
	pwd := p.SSHPassword
	if pwd != nil && *pwd != "" {
		if dec, err := config.DecryptText(*pwd); err == nil {
			pwd = &dec
		}
	}
	key := p.SSHKey
	if key != nil && *key != "" {
		if dec, err := config.DecryptText(*key); err == nil {
			key = &dec
		}
	}

	pipelinesDir := p.Path
	if pipelinesDir == "" || pipelinesDir == "/etc/prometheus/prometheus.yml" {
		pipelinesDir = "/opt/data-prepper/pipelines"
	}

	return &domain.DataPrepperConfig{
		ID:           p.ID,
		Name:         p.Name,
		Mode:         p.Mode,
		PipelinesDir: pipelinesDir,
		ReloadURL:    &p.ReloadURL,
		SSHHost:      p.SSHHost,
		SSHPort:      p.SSHPort,
		SSHUser:      p.SSHUser,
		SSHAuth:      p.SSHAuth,
		SSHPassword:  pwd,
		SSHKey:       key,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
	}
}

func (s *DataPrepperService) makeRemoteHostConfig(cfg *domain.DataPrepperConfig) *domain.RemoteHostConfig {
	port := 22
	if cfg.SSHPort != nil && *cfg.SSHPort > 0 {
		port = *cfg.SSHPort
	}
	user := "root"
	if cfg.SSHUser != nil && *cfg.SSHUser != "" {
		user = *cfg.SSHUser
	}
	auth := "password"
	if cfg.SSHAuth != nil && *cfg.SSHAuth != "" {
		auth = *cfg.SSHAuth
	}

	return &domain.RemoteHostConfig{
		Host:     *cfg.SSHHost,
		Port:     port,
		Username: user,
		AuthType: auth,
		Password: cfg.SSHPassword,
		SSHKey:   cfg.SSHKey,
	}
}

func (s *DataPrepperService) ListPipelines(ctx context.Context, instanceID ...string) ([]string, error) {
	cfg, err := s.GetActiveConfig(ctx, instanceID...)
	if err != nil {
		return nil, err
	}

	mode := strings.ToLower(cfg.Mode)
	if mode == "local" || cfg.SSHHost == nil || *cfg.SSHHost == "" {
		files, err := os.ReadDir(cfg.PipelinesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read local pipelines directory '%s': %w", cfg.PipelinesDir, err)
		}
		var list []string
		for _, f := range files {
			if !f.IsDir() && (filepath.Ext(f.Name()) == ".yml" || filepath.Ext(f.Name()) == ".yaml") {
				list = append(list, f.Name())
			}
		}
		return list, nil
	}

	// Remote via SSH / SFTP
	remoteCfg := s.makeRemoteHostConfig(cfg)
	entries, err := s.sshService.SftpListDirWithConfig(remoteCfg, cfg.PipelinesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines from remote '%s': %w", cfg.PipelinesDir, err)
	}

	var list []string
	for _, e := range entries {
		if !e.IsDir && (filepath.Ext(e.Name) == ".yml" || filepath.Ext(e.Name) == ".yaml") {
			list = append(list, e.Name)
		}
	}
	return list, nil
}

func (s *DataPrepperService) GetPipelineFile(ctx context.Context, instanceID, fileName string) (string, error) {
	cfg, err := s.GetActiveConfig(ctx, instanceID)
	if err != nil {
		return "", err
	}

	cleanFileName := filepath.Base(fileName)
	targetPath := filepath.ToSlash(filepath.Join(cfg.PipelinesDir, cleanFileName))

	mode := strings.ToLower(cfg.Mode)
	if mode == "local" || cfg.SSHHost == nil || *cfg.SSHHost == "" {
		data, err := os.ReadFile(targetPath)
		if err != nil {
			return "", fmt.Errorf("failed to read local pipeline file '%s': %w", targetPath, err)
		}
		return string(data), nil
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)
	return s.sshService.ReadFile(remoteCfg, targetPath)
}

func (s *DataPrepperService) SavePipelineFile(ctx context.Context, instanceID, fileName, content string) error {
	cfg, err := s.GetActiveConfig(ctx, instanceID)
	if err != nil {
		return err
	}

	cleanFileName := filepath.Base(fileName)
	targetPath := filepath.ToSlash(filepath.Join(cfg.PipelinesDir, cleanFileName))

	mode := strings.ToLower(cfg.Mode)
	if mode == "local" || cfg.SSHHost == nil || *cfg.SSHHost == "" {
		return os.WriteFile(targetPath, []byte(content), 0644)
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)
	return s.sshService.WriteFile(remoteCfg, targetPath, content)
}

func (s *DataPrepperService) ValidateYAML(content string) (bool, string) {
	var body interface{}
	err := yaml.Unmarshal([]byte(content), &body)
	if err != nil {
		return false, fmt.Sprintf("YAML syntax error: %v", err)
	}
	return true, "Valid YAML syntax"
}
