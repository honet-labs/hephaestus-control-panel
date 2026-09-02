package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

func (s *DataPrepperService) GetActiveConfig(ctx context.Context) (*domain.DataPrepperConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, mode, pipelines_dir, reload_url, ssh_host, ssh_port, ssh_user, ssh_auth, is_active, created_at 
              FROM dataprepper_configs WHERE is_active = true LIMIT 1`
	var c domain.DataPrepperConfig
	err = pool.QueryRow(ctx, query).Scan(&c.ID, &c.Name, &c.Mode, &c.PipelinesDir, &c.ReloadURL, &c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *DataPrepperService) ListPipelines(ctx context.Context) ([]string, error) {
	cfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	if cfg.Mode == "local" {
		files, err := os.ReadDir(cfg.PipelinesDir)
		if err != nil {
			return nil, err
		}
		var list []string
		for _, f := range files {
			if !f.IsDir() && (filepath.Ext(f.Name()) == ".yml" || filepath.Ext(f.Name()) == ".yaml") {
				list = append(list, f.Name())
			}
		}
		return list, nil
	}

	// Remote via SFTP
	entries, err := s.sshService.SftpListDir(ctx, cfg.ID, cfg.PipelinesDir)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, e := range entries {
		if !e.IsDir && (filepath.Ext(e.Name) == ".yml" || filepath.Ext(e.Name) == ".yaml") {
			list = append(list, e.Name)
		}
	}
	return list, nil
}

func (s *DataPrepperService) ValidateYAML(content string) (bool, string) {
	var body interface{}
	err := yaml.Unmarshal([]byte(content), &body)
	if err != nil {
		return false, fmt.Sprintf("YAML syntax error: %v", err)
	}
	return true, "Valid YAML syntax"
}
