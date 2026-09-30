package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"

	"gopkg.in/yaml.v3"
)

type PrometheusService struct {
	configRepo      *repository.ConfigRepository
	sshService      *SSHService
	httpClient      *http.Client
	promMu          sync.RWMutex
	cachedBaseURL   string
	cachedActiveCfg *domain.PrometheusConfig
	cachedActiveAt  time.Time
}

func NewPrometheusService(configRepo *repository.ConfigRepository, sshService *SSHService) *PrometheusService {
	// Custom HTTP transport with persistent keep-alive connection pool
	// Prevents constant TCP connection churn during high concurrency metric polling
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &PrometheusService{
		configRepo: configRepo,
		sshService: sshService,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// InvalidateCache clears the active config and resolved base URL cache
func (s *PrometheusService) InvalidateCache() {
	s.promMu.Lock()
	s.cachedActiveCfg = nil
	s.cachedBaseURL = ""
	s.promMu.Unlock()
}

// GetActiveConfig returns the active Prometheus configuration with a 30-second in-memory TTL
// This eliminates dozens of redundant PostgreSQL roundtrips during concurrent metric queries
func (s *PrometheusService) GetActiveConfig(ctx context.Context) (*domain.PrometheusConfig, error) {
	s.promMu.RLock()
	if s.cachedActiveCfg != nil && time.Since(s.cachedActiveAt) < 30*time.Second {
		cfg := s.cachedActiveCfg
		s.promMu.RUnlock()
		return cfg, nil
	}
	s.promMu.RUnlock()

	cfg, err := s.configRepo.GetActivePrometheus(ctx)
	if err != nil {
		return nil, err
	}

	s.promMu.Lock()
	s.cachedActiveCfg = cfg
	s.cachedActiveAt = time.Now()
	s.promMu.Unlock()

	return cfg, nil
}

func (s *PrometheusService) resolveBaseURL(ctx context.Context, promCfg *domain.PrometheusConfig) string {
	s.promMu.RLock()
	cached := s.cachedBaseURL
	s.promMu.RUnlock()
	if cached != "" {
		return cached
	}

	rawBase := strings.TrimSuffix(promCfg.ReloadURL, "/-/reload")
	rawBase = strings.TrimRight(rawBase, "/")

	var candidates []string
	if rawBase != "" {
		candidates = append(candidates, rawBase)
	}

	if u, err := url.Parse(rawBase); err == nil {
		h := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "9090"
		}
		if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "" {
			candidates = append(candidates,
				fmt.Sprintf("http://host.docker.internal:%s", port),
				fmt.Sprintf("http://172.17.0.1:%s", port),
			)
			if promCfg.SSHHost != nil && *promCfg.SSHHost != "" && *promCfg.SSHHost != "localhost" && *promCfg.SSHHost != "127.0.0.1" {
				candidates = append(candidates, fmt.Sprintf("http://%s:%s", *promCfg.SSHHost, port))
			}
		}
	} else if promCfg.SSHHost != nil && *promCfg.SSHHost != "" {
		candidates = append(candidates, fmt.Sprintf("http://%s:9090", *promCfg.SSHHost))
	}

	// Always fallback to rawBase if probing doesn't pick anything
	workingURL := rawBase
	for _, cand := range candidates {
		probeCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		req, err := http.NewRequestWithContext(probeCtx, "GET", cand+"/-/healthy", nil)
		if err == nil {
			resp, rErr := s.httpClient.Do(req)
			if rErr == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					workingURL = cand
					cancel()
					break
				}
			}
		}
		cancel()
	}

	s.promMu.Lock()
	s.cachedBaseURL = workingURL
	s.promMu.Unlock()

	return workingURL
}

func (s *PrometheusService) Query(ctx context.Context, promQL string) (interface{}, error) {
	return s.QueryPromQL(ctx, promQL)
}

func (s *PrometheusService) QueryPromQL(ctx context.Context, promQL string) (interface{}, error) {
	data, err := s.QueryPromQLRaw(ctx, promQL)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PrometheusService) QueryPromQLRaw(ctx context.Context, promQL string) ([]byte, error) {
	promCfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active Prometheus server found: %w", err)
	}

	baseURL := s.resolveBaseURL(ctx, promCfg)
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (s *PrometheusService) QueryRangePromQL(ctx context.Context, promQL string, start, end time.Time, step string) ([]byte, error) {
	promCfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active Prometheus server found: %w", err)
	}

	baseURL := s.resolveBaseURL(ctx, promCfg)
	queryURL := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		baseURL, url.QueryEscape(promQL), start.Unix(), end.Unix(), step)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (s *PrometheusService) ReloadConfig(ctx context.Context) error {
	s.InvalidateCache()
	promCfg, err := s.GetActiveConfig(ctx)
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

		// 2. Test File or Directory Existence and Readability
		filePath := cfg.Path
		if filePath == "" {
			filePath = "/etc/prometheus/prometheus.yml"
		}

		isDataPrepper := strings.Contains(strings.ToLower(cfg.Name), "data prepper") ||
			strings.Contains(strings.ToLower(cfg.Name), "dataprepper") ||
			strings.Contains(strings.ToLower(filePath), "pipeline")

		exists, isDir, err := s.sshService.CheckPath(remoteCfg, filePath)
		if err != nil || !exists {
			itemType := "file"
			if isDataPrepper || strings.Contains(filePath, "pipeline") {
				itemType = "pipelines directory"
			}
			return false, fmt.Sprintf("SSH authenticated to %s@%s:%d, but failed to access remote %s '%s': %v", user, *cfg.SSHHost, port, itemType, err), err
		}

		if isDir {
			return true, fmt.Sprintf("Connection verified: SSH authenticated to %s@%s:%d and remote directory '%s' is accessible.", user, *cfg.SSHHost, port, filePath), nil
		}

		_, err = s.sshService.ReadFile(remoteCfg, filePath)
		if err != nil {
			return false, fmt.Sprintf("SSH authenticated to %s@%s:%d, but failed to read remote file '%s': %v", user, *cfg.SSHHost, port, filePath, err), err
		}

		return true, fmt.Sprintf("Connection verified: SSH authenticated to %s@%s:%d and '%s' is accessible.", user, *cfg.SSHHost, port, filePath), nil
	}

	// Local file / directory mode
	filePath := cfg.Path
	if filePath == "" {
		filePath = "/etc/prometheus/prometheus.yml"
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Sprintf("Local path not found at '%s': %v", filePath, err), err
	}
	if stat.IsDir() {
		return true, fmt.Sprintf("Local directory verified at '%s'", filePath), nil
	}

	return true, fmt.Sprintf("Local configuration file verified at '%s'", filePath), nil
}

// GetConfigFile reads the prometheus.yml file from the remote SSH server or local file system
func (s *PrometheusService) GetConfigFile(ctx context.Context, instanceID string) (string, *domain.PrometheusConfig, error) {
	var promCfg *domain.PrometheusConfig
	var err error
	if instanceID != "" {
		promCfg, err = s.configRepo.GetPrometheusByID(ctx, instanceID)
	} else {
		promCfg, err = s.GetActiveConfig(ctx)
	}
	if err != nil {
		return "", nil, fmt.Errorf("Prometheus configuration not found: %w", err)
	}

	nameLower := strings.ToLower(promCfg.Name)
	pathLower := strings.ToLower(promCfg.Path)
	if strings.Contains(nameLower, "data prepper") || strings.Contains(nameLower, "dataprepper") || strings.Contains(pathLower, "pipeline") {
		return "", promCfg, fmt.Errorf("instance '%s' is a Data Prepper pipeline directory, not a Prometheus config file. Please select a valid Prometheus instance", promCfg.Name)
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

type ServiceRestartResult struct {
	Attempted bool   `json:"attempted"`
	Success   bool   `json:"success"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

type PrometheusSaveResult struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Path    string               `json:"path"`
	Restart ServiceRestartResult `json:"restart"`
}

// RestartService restarts the Prometheus service via systemctl, docker, or reload endpoint
func (s *PrometheusService) RestartService(ctx context.Context, promCfg *domain.PrometheusConfig) ServiceRestartResult {
	result := ServiceRestartResult{Attempted: true}

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

		reloadURL := promCfg.ReloadURL
		if reloadURL == "" {
			reloadURL = "http://localhost:9090/-/reload"
		}

		restartScript := fmt.Sprintf(`
RELOAD_URL="%s"
if command -v systemctl >/dev/null 2>&1 && (systemctl list-unit-files 2>/dev/null | grep -qE '^prometheus(-server)?\.service' || systemctl is-active --quiet prometheus 2>/dev/null || systemctl is-active --quiet prometheus-server 2>/dev/null); then
    _run_sudo systemctl restart prometheus 2>/dev/null || _run_sudo systemctl restart prometheus-server 2>/dev/null
    sleep 1
    if systemctl is-active --quiet prometheus 2>/dev/null || systemctl is-active --quiet prometheus-server 2>/dev/null; then
        echo "Prometheus service restarted successfully via systemd and is active."
        exit 0
    else
        echo "Prometheus service failed to become active after restart."
        exit 1
    fi
elif command -v docker >/dev/null 2>&1 && (_run_sudo docker ps --format '{{.Names}}' 2>/dev/null | grep -qiE 'prometheus'); then
    prom_c=$(_run_sudo docker ps --format '{{.Names}}' 2>/dev/null | grep -iE 'prometheus' | head -n 1)
    _run_sudo docker restart "$prom_c"
    echo "Prometheus Docker container '$prom_c' restarted successfully."
    exit 0
else
    resp=$(curl -s -o /dev/null -w "%%{http_code}" -X POST "$RELOAD_URL" 2>/dev/null || echo "000")
    if [ "$resp" = "200" ]; then
        echo "Prometheus configuration reloaded via HTTP endpoint ($RELOAD_URL)."
        exit 0
    else
        echo "Failed to restart or reload Prometheus via $RELOAD_URL (HTTP $resp)."
        exit 1
    fi
fi
`, reloadURL)

		stdout, stderr, exitCode, err := s.sshService.ExecuteElevatedCommand(remoteCfg, restartScript)
		outStr := strings.TrimSpace(stdout)
		if outStr == "" {
			outStr = strings.TrimSpace(stderr)
		}
		if err != nil || exitCode != 0 {
			result.Success = false
			if err != nil {
				result.Error = fmt.Sprintf("Restart command failed (exit code %d): %v. %s", exitCode, err, outStr)
			} else {
				result.Error = fmt.Sprintf("Restart command exited with code %d: %s", exitCode, outStr)
			}
			result.Output = outStr
			return result
		}

		result.Success = true
		result.Output = outStr
		return result
	}

	// Local mode
	cmd := exec.CommandContext(ctx, "sh", "-c", `
if command -v systemctl >/dev/null 2>&1 && (systemctl list-unit-files 2>/dev/null | grep -qE '^prometheus(-server)?\.service' || systemctl is-active --quiet prometheus 2>/dev/null || systemctl is-active --quiet prometheus-server 2>/dev/null); then
    systemctl restart prometheus 2>/dev/null || systemctl restart prometheus-server 2>/dev/null
    if systemctl is-active --quiet prometheus 2>/dev/null || systemctl is-active --quiet prometheus-server 2>/dev/null; then
        echo "Prometheus service restarted successfully via systemd."
        exit 0
    fi
    exit 1
fi
`)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err == nil && outStr != "" {
		result.Success = true
		result.Output = outStr
		return result
	}

	// Try reload URL
	if promCfg.ReloadURL != "" {
		if reloadErr := s.ReloadConfig(ctx); reloadErr == nil {
			result.Success = true
			result.Output = fmt.Sprintf("Prometheus configuration reloaded via %s.", promCfg.ReloadURL)
			return result
		} else {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to reload Prometheus: %v", reloadErr)
			return result
		}
	}

	result.Success = false
	result.Error = "No local systemd unit or reload URL available for Prometheus."
	return result
}

// SaveConfigFile writes updated YAML to the remote SSH server or local file system, and triggers service restart
func (s *PrometheusService) SaveConfigFile(ctx context.Context, instanceID string, content string, triggerReload bool) (*PrometheusSaveResult, error) {
	var promCfg *domain.PrometheusConfig
	var err error
	if instanceID != "" {
		promCfg, err = s.configRepo.GetPrometheusByID(ctx, instanceID)
	} else {
		promCfg, err = s.GetActiveConfig(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("Prometheus configuration not found: %w", err)
	}
	defer s.InvalidateCache()

	nameLowerSave := strings.ToLower(promCfg.Name)
	pathLowerSave := strings.ToLower(promCfg.Path)
	if strings.Contains(nameLowerSave, "data prepper") || strings.Contains(nameLowerSave, "dataprepper") || strings.Contains(pathLowerSave, "pipeline") {
		return nil, fmt.Errorf("instance '%s' is a Data Prepper pipeline directory, not a Prometheus config file", promCfg.Name)
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
			return nil, fmt.Errorf("failed to write remote file '%s' via SSH: %w", filePath, err)
		}

		res := &PrometheusSaveResult{
			Success: true,
			Path:    filePath,
		}

		if triggerReload {
			res.Restart = s.RestartService(ctx, promCfg)
			if res.Restart.Success {
				res.Message = fmt.Sprintf("Prometheus configuration saved to '%s' and service restarted successfully.", filePath)
			} else {
				res.Message = fmt.Sprintf("Prometheus configuration saved to '%s', but service restart failed: %s", filePath, res.Restart.Error)
			}
		} else {
			res.Message = fmt.Sprintf("Prometheus configuration saved to '%s'.", filePath)
		}
		return res, nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write local file '%s': %w", filePath, err)
	}

	res := &PrometheusSaveResult{
		Success: true,
		Path:    filePath,
	}

	if triggerReload {
		res.Restart = s.RestartService(ctx, promCfg)
		if res.Restart.Success {
			res.Message = fmt.Sprintf("Prometheus configuration saved to '%s' and service restarted successfully.", filePath)
		} else {
			res.Message = fmt.Sprintf("Prometheus configuration saved to '%s', but service restart failed: %s", filePath, res.Restart.Error)
		}
	} else {
		res.Message = fmt.Sprintf("Prometheus configuration saved to '%s'.", filePath)
	}
	return res, nil
}

// PrometheusValidationIssue represents a problem found during YAML and schema validation
type PrometheusValidationIssue struct {
	Type    string `json:"type"` // "error", "warning", "info"
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

// ValidateYAML performs deep structural, syntax, and Prometheus schema validation
func (s *PrometheusService) ValidateYAML(ctx context.Context, content string, instanceID string) (bool, []PrometheusValidationIssue) {
	var issues []PrometheusValidationIssue

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		issues = append(issues, PrometheusValidationIssue{
			Type:    "error",
			Message: "Configuration YAML is empty.",
		})
		return false, issues
	}

	// 1. Core YAML Parser Validation using yaml.v3
	var body interface{}
	if err := yaml.Unmarshal([]byte(content), &body); err != nil {
		issues = append(issues, PrometheusValidationIssue{
			Type:    "error",
			Message: fmt.Sprintf("YAML syntax error: %v", err),
		})
		return false, issues
	}

	// Parse as top-level map to validate Prometheus sections
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &rawMap); err != nil || rawMap == nil {
		issues = append(issues, PrometheusValidationIssue{
			Type:    "error",
			Message: "Configuration must be a valid YAML mapping at the root level.",
		})
		return false, issues
	}

	// 2. Validate Prometheus top-level keys
	validTopKeys := map[string]bool{
		"global":         true,
		"alerting":       true,
		"rule_files":     true,
		"scrape_configs": true,
		"storage":        true,
		"tracing":        true,
		"remote_write":   true,
		"remote_read":    true,
		"runtime":        true,
	}

	for k := range rawMap {
		if !validTopKeys[k] {
			issues = append(issues, PrometheusValidationIssue{
				Type:    "warning",
				Message: fmt.Sprintf("Unrecognized root directive '%s'. Ensure it is a valid Prometheus top-level key.", k),
			})
		}
	}

	// 3. Validate global section
	if globalRaw, ok := rawMap["global"]; ok {
		if globalMap, ok := globalRaw.(map[string]interface{}); ok {
			durationKeys := []string{"scrape_interval", "evaluation_interval", "scrape_timeout"}
			for _, dk := range durationKeys {
				if val, exists := globalMap[dk]; exists {
					valStr := fmt.Sprintf("%v", val)
					if !isValidPrometheusDuration(valStr) {
						issues = append(issues, PrometheusValidationIssue{
							Type:    "error",
							Message: fmt.Sprintf("global.%s: '%s' is not a valid Prometheus duration (expected e.g. '15s', '1m', '500ms').", dk, valStr),
						})
					}
				}
			}
		}
	}

	// 4. Validate scrape_configs
	if scRaw, ok := rawMap["scrape_configs"]; ok {
		if scList, ok := scRaw.([]interface{}); ok {
			jobNames := make(map[string]bool)
			for idx, item := range scList {
				jobMap, ok := item.(map[string]interface{})
				if !ok {
					issues = append(issues, PrometheusValidationIssue{
						Type:    "error",
						Message: fmt.Sprintf("scrape_configs[%d]: entry is not a valid job mapping.", idx),
					})
					continue
				}

				nameVal, ok := jobMap["job_name"]
				if !ok || strings.TrimSpace(fmt.Sprintf("%v", nameVal)) == "" {
					issues = append(issues, PrometheusValidationIssue{
						Type:    "error",
						Message: fmt.Sprintf("scrape_configs[%d]: missing required 'job_name' field.", idx),
					})
					continue
				}

				jobName := fmt.Sprintf("%v", nameVal)
				if jobNames[jobName] {
					issues = append(issues, PrometheusValidationIssue{
						Type:    "error",
						Message: fmt.Sprintf("Duplicate job_name '%s' detected. Each scrape job must have a unique name.", jobName),
					})
				}
				jobNames[jobName] = true

				for _, dk := range []string{"scrape_interval", "scrape_timeout"} {
					if val, exists := jobMap[dk]; exists {
						valStr := fmt.Sprintf("%v", val)
						if !isValidPrometheusDuration(valStr) {
							issues = append(issues, PrometheusValidationIssue{
								Type:    "error",
								Message: fmt.Sprintf("Job '%s': '%s' has invalid duration '%s'. Expected unit (e.g. '15s').", jobName, dk, valStr),
							})
						}
					}
				}

				if stRaw, ok := jobMap["static_configs"]; ok {
					if stList, ok := stRaw.([]interface{}); ok {
						targetsSeen := make(map[string]bool)
						totalTargets := 0
						for _, stItem := range stList {
							if stMap, ok := stItem.(map[string]interface{}); ok {
								if tRaw, ok := stMap["targets"]; ok {
									if tList, ok := tRaw.([]interface{}); ok {
										for _, tItem := range tList {
											totalTargets++
											tStr := strings.TrimSpace(fmt.Sprintf("%v", tItem))
											if targetsSeen[tStr] {
												issues = append(issues, PrometheusValidationIssue{
													Type:    "error",
													Message: fmt.Sprintf("Job '%s': duplicate target endpoint '%s' found.", jobName, tStr),
												})
											}
											targetsSeen[tStr] = true
										}
									}
								}
							}
						}
						if totalTargets == 0 {
							issues = append(issues, PrometheusValidationIssue{
								Type:    "warning",
								Message: fmt.Sprintf("Job '%s' has 0 static targets configured.", jobName),
							})
						}
					}
				}
			}
		} else {
			issues = append(issues, PrometheusValidationIssue{
				Type:    "error",
				Message: "scrape_configs must be a list of scrape jobs.",
			})
		}
	}

	// 5. If host has promtool installed, run official promtool verification
	if instanceID != "" {
		cfg, err := s.configRepo.GetPrometheusByID(ctx, instanceID)
		if err == nil && cfg != nil {
			mode := strings.ToLower(cfg.Mode)
			if mode == "ssh" && cfg.SSHHost != nil && *cfg.SSHHost != "" {
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

				checkToolCmd := "which promtool 2>/dev/null || which /usr/local/bin/promtool 2>/dev/null"
				toolOut, _, _, err := s.sshService.ExecuteCommand(remoteCfg, checkToolCmd)
				toolPath := strings.TrimSpace(toolOut)
				if err == nil && toolPath != "" {
					tmpFile := fmt.Sprintf("/tmp/prom_check_%d.yml", time.Now().UnixNano())
					if writeErr := s.sshService.WriteFile(remoteCfg, tmpFile, content); writeErr == nil {
						cmd := fmt.Sprintf("%s check config %s", toolPath, tmpFile)
						checkOut, checkErrOut, _, checkErr := s.sshService.ExecuteCommand(remoteCfg, cmd)
						_ = s.sshService.DeleteFile(remoteCfg, tmpFile)

						combinedOutput := strings.TrimSpace(checkOut + "\n" + checkErrOut)
						if checkErr != nil || strings.Contains(strings.ToLower(combinedOutput), "failed") {
							issues = append(issues, PrometheusValidationIssue{
								Type:    "error",
								Message: fmt.Sprintf("promtool check error: %s", combinedOutput),
							})
						} else {
							issues = append(issues, PrometheusValidationIssue{
								Type:    "info",
								Message: "promtool compiler verified: configuration structure is 100% compliant with Prometheus specifications.",
							})
						}
					}
				}
			}
		}
	}

	hasErrors := false
	for _, it := range issues {
		if it.Type == "error" {
			hasErrors = true
			break
		}
	}

	if !hasErrors {
		issues = append([]PrometheusValidationIssue{
			{
				Type:    "info",
				Message: "YAML syntax structure verified and compliant.",
			},
		}, issues...)
	}

	return !hasErrors, issues
}

func isValidPrometheusDuration(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	re := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(ms|s|m|h|d|w|y)$`)
	return re.MatchString(v)
}
