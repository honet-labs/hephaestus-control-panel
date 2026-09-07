package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
)

type PrometheusService struct {
	configRepo *repository.ConfigRepository
	sshService *SSHService
	httpClient *http.Client
}

func NewPrometheusService(configRepo *repository.ConfigRepository, sshService *SSHService) *PrometheusService {
	return &PrometheusService{
		configRepo: configRepo,
		sshService: sshService,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *PrometheusService) Query(ctx context.Context, promQL string) (interface{}, error) {
	return s.QueryPromQL(ctx, promQL)
}

func (s *PrometheusService) QueryPromQL(ctx context.Context, promQL string) (interface{}, error) {
	promCfg, err := s.configRepo.GetActivePrometheus(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active Prometheus server found: %w", err)
	}

	baseURL := strings.TrimSuffix(promCfg.ReloadURL, "/-/reload")
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", baseURL, url.QueryEscape(promQL))

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PrometheusService) ReloadConfig(ctx context.Context) error {
	promCfg, err := s.configRepo.GetActivePrometheus(ctx)
	if err != nil {
		return fmt.Errorf("no active Prometheus server found: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", promCfg.ReloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("reload failed with status: %d", resp.StatusCode)
	}
	return nil
}

// TestConnection tests the reachability and credentials of a Prometheus configuration
func (s *PrometheusService) TestConnection(ctx context.Context, cfg domain.PrometheusConfig) (bool, string, error) {
	// If ID is provided and password is not supplied (or masked), retrieve saved decrypted credentials
	if cfg.ID != "" && (cfg.SSHPassword == nil || *cfg.SSHPassword == "" || *cfg.SSHPassword == "••••••" || *cfg.SSHPassword == "********") {
		dbCfg, err := s.configRepo.GetPrometheusByID(ctx, cfg.ID)
		if err == nil && dbCfg != nil {
			if cfg.Mode == "" {
				cfg.Mode = dbCfg.Mode
			}
			if cfg.Path == "" {
				cfg.Path = dbCfg.Path
			}
			if cfg.ReloadURL == "" {
				cfg.ReloadURL = dbCfg.ReloadURL
			}
			if cfg.SSHHost == nil || *cfg.SSHHost == "" {
				cfg.SSHHost = dbCfg.SSHHost
			}
			if cfg.SSHPort == nil || *cfg.SSHPort == 0 {
				cfg.SSHPort = dbCfg.SSHPort
			}
			if cfg.SSHUser == nil || *cfg.SSHUser == "" {
				cfg.SSHUser = dbCfg.SSHUser
			}
			if cfg.SSHAuth == nil || *cfg.SSHAuth == "" {
				cfg.SSHAuth = dbCfg.SSHAuth
			}
			if cfg.SSHPassword == nil || *cfg.SSHPassword == "" || *cfg.SSHPassword == "••••••" || *cfg.SSHPassword == "********" {
				cfg.SSHPassword = dbCfg.SSHPassword
			}
			if cfg.SSHKey == nil || *cfg.SSHKey == "" || *cfg.SSHKey == "********" {
				cfg.SSHKey = dbCfg.SSHKey
			}
		}
	}

	mode := strings.ToLower(cfg.Mode)
	if mode == "ssh" || (cfg.SSHHost != nil && *cfg.SSHHost != "") {
		if cfg.SSHHost == nil || *cfg.SSHHost == "" {
			return false, "SSH Host IP is required for SSH mode.", fmt.Errorf("SSH Host IP is required")
		}
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

		remoteCfg := &domain.RemoteHostConfig{
			Host:     *cfg.SSHHost,
			Port:     port,
			Username: user,
			AuthType: auth,
			Password: cfg.SSHPassword,
			SSHKey:   cfg.SSHKey,
		}

		// 1. Test SSH Dial / Authentication
		client, err := s.sshService.Dial(remoteCfg)
		if err != nil {
			return false, fmt.Sprintf("SSH connection/authentication failed (%s@%s:%d): %v", user, *cfg.SSHHost, port, err), err
		}
		defer client.Close()

		// 2. Test File Existence and Readability
		filePath := cfg.Path
		if filePath == "" {
			filePath = "/etc/prometheus/prometheus.yml"
		}
		_, err = s.sshService.ReadFile(remoteCfg, filePath)
		if err != nil {
			return false, fmt.Sprintf("SSH authenticated to %s@%s:%d, but failed to read prometheus file '%s': %v", user, *cfg.SSHHost, port, filePath, err), err
		}

		return true, fmt.Sprintf("Connection verified: SSH authenticated to %s@%s:%d and '%s' is accessible.", user, *cfg.SSHHost, port, filePath), nil
	}

	// Local file mode
	filePath := cfg.Path
	if filePath == "" {
		filePath = "/etc/prometheus/prometheus.yml"
	}
	if _, err := os.Stat(filePath); err != nil {
		return false, fmt.Sprintf("Local prometheus file not found at '%s': %v", filePath, err), err
	}

	return true, fmt.Sprintf("Local prometheus configuration file verified at '%s'", filePath), nil
}

// GetConfigFile reads the prometheus.yml file from the remote SSH server or local file system
func (s *PrometheusService) GetConfigFile(ctx context.Context, instanceID string) (string, *domain.PrometheusConfig, error) {
	var promCfg *domain.PrometheusConfig
	var err error
	if instanceID != "" {
		promCfg, err = s.configRepo.GetPrometheusByID(ctx, instanceID)
	} else {
		promCfg, err = s.configRepo.GetActivePrometheus(ctx)
	}
	if err != nil {
		return "", nil, fmt.Errorf("Prometheus configuration not found: %w", err)
	}

	filePath := promCfg.Path
	if filePath == "" {
		filePath = "/etc/prometheus/prometheus.yml"
	}

	mode := strings.ToLower(promCfg.Mode)
	if mode == "ssh" || (promCfg.SSHHost != nil && *promCfg.SSHHost != "") {
		port := 22
		if promCfg.SSHPort != nil && *promCfg.SSHPort > 0 {
			port = *promCfg.SSHPort
		}
		user := "root"
		if promCfg.SSHUser != nil && *promCfg.SSHUser != "" {
			user = *promCfg.SSHUser
		}
		auth := "password"
		if promCfg.SSHAuth != nil && *promCfg.SSHAuth != "" {
			auth = *promCfg.SSHAuth
		}

		remoteCfg := &domain.RemoteHostConfig{
			Host:     *promCfg.SSHHost,
			Port:     port,
			Username: user,
			AuthType: auth,
			Password: promCfg.SSHPassword,
			SSHKey:   promCfg.SSHKey,
		}

		content, err := s.sshService.ReadFile(remoteCfg, filePath)
		if err != nil {
			return "", promCfg, fmt.Errorf("failed to read remote file '%s' via SSH: %w", filePath, err)
		}
		return content, promCfg, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", promCfg, fmt.Errorf("failed to read local file '%s': %w", filePath, err)
	}
	return string(data), promCfg, nil
}

// SaveConfigFile writes updated YAML to the remote SSH server or local file system, and optionally triggers reload
func (s *PrometheusService) SaveConfigFile(ctx context.Context, instanceID string, content string, triggerReload bool) error {
	var promCfg *domain.PrometheusConfig
	var err error
	if instanceID != "" {
		promCfg, err = s.configRepo.GetPrometheusByID(ctx, instanceID)
	} else {
		promCfg, err = s.configRepo.GetActivePrometheus(ctx)
	}
	if err != nil {
		return fmt.Errorf("Prometheus configuration not found: %w", err)
	}

	filePath := promCfg.Path
	if filePath == "" {
		filePath = "/etc/prometheus/prometheus.yml"
	}

	mode := strings.ToLower(promCfg.Mode)
	if mode == "ssh" || (promCfg.SSHHost != nil && *promCfg.SSHHost != "") {
		port := 22
		if promCfg.SSHPort != nil && *promCfg.SSHPort > 0 {
			port = *promCfg.SSHPort
		}
		user := "root"
		if promCfg.SSHUser != nil && *promCfg.SSHUser != "" {
			user = *promCfg.SSHUser
		}
		auth := "password"
		if promCfg.SSHAuth != nil && *promCfg.SSHAuth != "" {
			auth = *promCfg.SSHAuth
		}

		remoteCfg := &domain.RemoteHostConfig{
			Host:     *promCfg.SSHHost,
			Port:     port,
			Username: user,
			AuthType: auth,
			Password: promCfg.SSHPassword,
			SSHKey:   promCfg.SSHKey,
		}

		if err := s.sshService.WriteFile(remoteCfg, filePath, content); err != nil {
			return fmt.Errorf("failed to write remote file '%s' via SSH: %w", filePath, err)
		}

		if triggerReload && promCfg.ReloadURL != "" {
			// Trigger reload on remote host via curl or http client
			_, _, _, _ = s.sshService.ExecuteCommand(remoteCfg, fmt.Sprintf("curl -s -X POST %s", promCfg.ReloadURL))
			_ = s.ReloadConfig(ctx)
		}
		return nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write local file '%s': %w", filePath, err)
	}

	if triggerReload {
		_ = s.ReloadConfig(ctx)
	}
	return nil
}
