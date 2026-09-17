package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"

	"gopkg.in/yaml.v3"
)

type OTelPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Content     string `json:"content"`
}

type OTelSaveResult struct {
	Success  bool                 `json:"success"`
	Message  string               `json:"message"`
	BackupID string               `json:"backupId,omitempty"`
	Restart  ServiceRestartResult `json:"restart"`
}

type OTelHostStatusResult struct {
	HostID        string `json:"hostId"`
	IsOnline      bool   `json:"isOnline"`
	ServiceStatus string `json:"serviceStatus"` // active, inactive, failed, not_found, unknown
	ActiveState   string `json:"activeState"`
	SubState      string `json:"subState"`
	Since         string `json:"since,omitempty"`
	Logs          string `json:"logs,omitempty"`
	Error         string `json:"error,omitempty"`
}

type OTelService struct {
	repo       *repository.OTelRepository
	sshService *SSHService
}

func NewOTelService(repo *repository.OTelRepository, sshService *SSHService) *OTelService {
	return &OTelService{
		repo:       repo,
		sshService: sshService,
	}
}

func (s *OTelService) makeRemoteHostConfig(cfg *domain.OpenTelemetryConfig) *domain.RemoteHostConfig {
	return &domain.RemoteHostConfig{
		ID:       cfg.ID,
		Name:     cfg.Name,
		Host:     cfg.SSHHost,
		Port:     cfg.SSHPort,
		Username: cfg.SSHUser,
		AuthType: cfg.SSHAuth,
		Password: cfg.SSHPassword,
		SSHKey:   cfg.SSHKey,
	}
}

// TestHostConnection checks SSH connectivity and queries OpenTelemetry service state
func (s *OTelService) TestHostConnection(ctx context.Context, cfg *domain.OpenTelemetryConfig) (bool, string, string) {
	remoteCfg := s.makeRemoteHostConfig(cfg)
	ok, msg := s.sshService.TestConnection(remoteCfg)
	if !ok {
		_ = s.repo.UpdateStatus(ctx, cfg.ID, "unreachable")
		return false, msg, "unreachable"
	}

	// Connected! Now check systemd service state
	serviceName := cfg.ServiceName
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "otelcol-contrib"
	}

	checkCmd := fmt.Sprintf(`if command -v systemctl >/dev/null 2>&1; then
		systemctl is-active %s 2>/dev/null || echo "inactive"
	else
		echo "unknown"
	fi`, serviceName)

	stdout, _, _, err := s.sshService.ExecuteElevatedCommand(remoteCfg, checkCmd)
	status := "unknown"
	if err == nil {
		out := strings.TrimSpace(stdout)
		if strings.Contains(out, "active") {
			status = "active"
		} else if strings.Contains(out, "failed") {
			status = "failed"
		} else if strings.Contains(out, "inactive") {
			status = "inactive"
		} else {
			status = "stopped"
		}
	}

	_ = s.repo.UpdateStatus(ctx, cfg.ID, status)
	fullMsg := fmt.Sprintf("%s. Service '%s' status: %s", msg, serviceName, status)
	return true, fullMsg, status
}

// GetHostStatus queries the live agent status from systemctl
func (s *OTelService) GetHostStatus(ctx context.Context, id string) (*OTelHostStatusResult, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("host config not found: %w", err)
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)
	serviceName := cfg.ServiceName
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "otelcol-contrib"
	}

	// Check SSH first
	connOk, connMsg := s.sshService.TestConnection(remoteCfg)
	if !connOk {
		_ = s.repo.UpdateStatus(ctx, cfg.ID, "unreachable")
		return &OTelHostStatusResult{
			HostID:        cfg.ID,
			IsOnline:      false,
			ServiceStatus: "unreachable",
			Error:         connMsg,
		}, nil
	}

	statusCmd := fmt.Sprintf(`_run_sudo systemctl status %s --no-pager -n 15 2>&1`, serviceName)
	stdout, _, _, _ := s.sshService.ExecuteElevatedCommand(remoteCfg, statusCmd)

	serviceStatus := "unknown"
	lowerOut := strings.ToLower(stdout)
	if strings.Contains(lowerOut, "active: active") || strings.Contains(lowerOut, "active (running)") {
		serviceStatus = "active"
	} else if strings.Contains(lowerOut, "active: failed") || strings.Contains(lowerOut, "failed (result") {
		serviceStatus = "failed"
	} else if strings.Contains(lowerOut, "active: inactive") || strings.Contains(lowerOut, "inactive (dead)") {
		serviceStatus = "inactive"
	} else if strings.Contains(lowerOut, "could not be found") || strings.Contains(lowerOut, "loaded: not-found") {
		serviceStatus = "not_found"
	} else {
		// Fallback check using systemctl is-active
		isActCmd := fmt.Sprintf(`_run_sudo systemctl is-active %s 2>/dev/null`, serviceName)
		actOut, _, _, _ := s.sshService.ExecuteElevatedCommand(remoteCfg, isActCmd)
		actOut = strings.ToLower(strings.TrimSpace(actOut))
		if actOut == "active" || actOut == "inactive" || actOut == "failed" {
			serviceStatus = actOut
		}
	}

	_ = s.repo.UpdateStatus(ctx, cfg.ID, serviceStatus)

	return &OTelHostStatusResult{
		HostID:        cfg.ID,
		IsOnline:      true,
		ServiceStatus: serviceStatus,
		ActiveState:   serviceStatus,
		Logs:          stdout,
	}, nil
}

// CheckAllHostsStatus queries live agent status for all registered hosts concurrently
func (s *OTelService) CheckAllHostsStatus(ctx context.Context) error {
	hosts, err := s.repo.List(ctx)
	if err != nil || len(hosts) == 0 {
		return err
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, h := range hosts {
		wg.Add(1)
		go func(host domain.OpenTelemetryConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			hostCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()

			_, _ = s.GetHostStatus(hostCtx, host.ID)
		}(h)
	}

	wg.Wait()
	return nil
}

// GetConfigFile reads the remote configuration file from the host
func (s *OTelService) GetConfigFile(ctx context.Context, id string) (string, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("host config not found: %w", err)
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)
	content, err := s.sshService.ReadFile(remoteCfg, cfg.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read remote file '%s': %w", cfg.ConfigPath, err)
	}
	return content, nil
}

// SaveConfigFile validates YAML syntax, backups old configuration, writes new file, and triggers restart
func (s *OTelService) SaveConfigFile(ctx context.Context, id string, content string, username string, changeSummary string, restartAfter bool) (*OTelSaveResult, error) {
	// 1. Validate YAML syntax
	var yamlObj interface{}
	if err := yaml.Unmarshal([]byte(content), &yamlObj); err != nil {
		return nil, fmt.Errorf("YAML validation error: %w", err)
	}

	// 2. Fetch host configuration
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("host config not found: %w", err)
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)

	// 3. Backup current remote file to history and remote .bak
	oldContent, readErr := s.sshService.ReadFile(remoteCfg, cfg.ConfigPath)
	if readErr == nil && strings.TrimSpace(oldContent) != "" {
		summary := changeSummary
		if strings.TrimSpace(summary) == "" {
			summary = fmt.Sprintf("Backup before update by %s", username)
		}
		_ = s.repo.SaveHistory(ctx, domain.OpenTelemetryConfigHistory{
			OTelConfigID:  cfg.ID,
			Content:       oldContent,
			CreatedBy:     &username,
			ChangeSummary: &summary,
		})

		// Also make a timestamped remote backup file on the server
		timestamp := time.Now().Format("20060102_150405")
		bakPath := fmt.Sprintf("%s.bak.%s", cfg.ConfigPath, timestamp)
		backupCmd := fmt.Sprintf("_run_sudo cp '%s' '%s' 2>/dev/null || true", cfg.ConfigPath, bakPath)
		_, _, _, _ = s.sshService.ExecuteElevatedCommand(remoteCfg, backupCmd)
	}

	// 4. Write new config to target path
	if err := s.sshService.WriteFile(remoteCfg, cfg.ConfigPath, content); err != nil {
		return nil, fmt.Errorf("failed to write remote config file '%s': %w", cfg.ConfigPath, err)
	}

	result := &OTelSaveResult{
		Success: true,
		Message: fmt.Sprintf("Configuration successfully deployed to '%s' on %s (%s).", filepath.Base(cfg.ConfigPath), cfg.Name, cfg.SSHHost),
	}

	// 5. Restart/Reload service if requested
	if restartAfter {
		restartRes, rErr := s.RestartService(ctx, id, cfg.ReloadMode)
		if rErr != nil {
			result.Restart = ServiceRestartResult{
				Attempted: true,
				Success:   false,
				Error:     rErr.Error(),
			}
			result.Message += fmt.Sprintf(" Notice: Service restart failed: %s", rErr.Error())
		} else if restartRes != nil {
			result.Restart = *restartRes
			if restartRes.Success {
				modeStr := "restarted"
				if strings.EqualFold(cfg.ReloadMode, "reload") {
					modeStr = "reloaded"
				}
				result.Message += fmt.Sprintf(" Service '%s' %s successfully.", cfg.ServiceName, modeStr)
			} else {
				result.Message += fmt.Sprintf(" Service restart attempted but failed: %s", restartRes.Error)
			}
		}
	}

	return result, nil
}

// RestartService restarts or reloads the OpenTelemetry systemd service
func (s *OTelService) RestartService(ctx context.Context, id string, mode string) (*ServiceRestartResult, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("host config not found: %w", err)
	}

	remoteCfg := s.makeRemoteHostConfig(cfg)
	serviceName := cfg.ServiceName
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "otelcol-contrib"
	}

	action := "restart"
	if strings.EqualFold(mode, "reload") {
		action = "reload"
	}

	cmd := fmt.Sprintf(`
		if command -v systemctl >/dev/null 2>&1; then
			if [ "%s" = "reload" ]; then
				restart_output=$(_run_sudo systemctl reload %s 2>&1) || restart_output=$(_run_sudo systemctl restart %s 2>&1)
				cmd_status=$?
			else
				restart_output=$(_run_sudo systemctl restart %s 2>&1)
				cmd_status=$?
			fi

			if [ $cmd_status -ne 0 ]; then
				echo "RESTART_CMD_FAILED: $restart_output"
				_run_sudo systemctl status %s --no-pager -n 15 2>&1
				exit 1
			fi

			sleep 2
			if systemctl is-active --quiet %s; then
				active_pid=$(systemctl show --property MainPID --value %s 2>/dev/null || echo 0)
				echo "STATUS_OK: %s is active (PID: $active_pid)"
				exit 0
			else
				echo "STATUS_FAILED: %s is not active"
				_run_sudo systemctl status %s --no-pager -n 15 2>&1
				exit 1
			fi
		else
			echo "systemctl command not found on host"
			exit 1
		fi
	`, action, serviceName, serviceName, serviceName, serviceName, serviceName, serviceName, serviceName, serviceName, serviceName)

	stdout, stderr, exitCode, execErr := s.sshService.ExecuteElevatedCommand(remoteCfg, cmd)
	res := &ServiceRestartResult{
		Attempted: true,
		Success:   exitCode == 0,
		Output:    stdout,
	}

	if exitCode == 0 && strings.Contains(stdout, "STATUS_OK") {
		res.Success = true
		_ = s.repo.UpdateStatus(ctx, cfg.ID, "active")
	} else {
		res.Success = false
		errMsg := strings.TrimSpace(stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout)
		}
		if execErr != nil {
			errMsg = fmt.Sprintf("%v: %s", execErr, errMsg)
		}
		res.Error = errMsg
		_ = s.repo.UpdateStatus(ctx, cfg.ID, "failed")
	}

	return res, nil
}

// GetPresets returns ready-to-use OpenTelemetry Collector configurations
func (s *OTelService) GetPresets() []OTelPreset {
	return []OTelPreset{
		{
			ID:          "host-metrics",
			Name:        "Host Metrics & System Telemetry",
			Description: "Collects system CPU, memory, load, disk, filesystem, and network metrics and exports them via Prometheus and OTLP.",
			Category:    "Infrastructure Monitoring",
			Content: `receivers:
  hostmetrics:
    collection_interval: 15s
    scrapers:
      cpu:
      disk:
      filesystem:
      load:
      memory:
      network:
      paging:
      process:
      processes:

processors:
  batch:
    send_batch_size: 1000
    timeout: 10s
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
  resourcedetection:
    detectors: [system]
    timeout: 2s

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: "hephaestus"
  otlp:
    endpoint: "hephaestus-collector:4317"
    tls:
      insecure: true
  debug:
    verbosity: basic

service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [memory_limiter, resourcedetection, batch]
      exporters: [prometheus, debug]
  telemetry:
    logs:
      level: "info"
    metrics:
      address: "0.0.0.0:8888"
`,
		},
		{
			ID:          "otlp-gateway",
			Name:        "OTLP Ingestion Gateway (gRPC & HTTP)",
			Description: "High-throughput OTLP gateway receiving traces, metrics, and logs from microservices on ports 4317 and 4318.",
			Category:    "Application Observability",
			Content: `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 25
  batch:
    send_batch_max_size: 1024
    send_batch_size: 512
    timeout: 5s

exporters:
  otlp/backend:
    endpoint: "observability-backend:4317"
    tls:
      insecure: true
  debug:
    verbosity: normal

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  zpages:
    endpoint: 0.0.0.0:55679

service:
  extensions: [health_check, zpages]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend, debug]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend, debug]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend, debug]
  telemetry:
    logs:
      level: "info"
`,
		},
		{
			ID:          "filelog-syslog",
			Name:        "Linux System & Auth Logs Tailer",
			Description: "Tails /var/log/syslog, /var/log/messages, and /var/log/auth.log with timestamp parsing and multi-destination log forwarding.",
			Category:    "Log Management",
			Content: `receivers:
  filelog:
    include:
      - /var/log/syslog
      - /var/log/messages
      - /var/log/auth.log
    start_at: end
    include_file_name: true
    include_file_path: true
    operators:
      - type: regex_parser
        regex: '^(?P<timestamp>\w+\s+\d+\s+\d+:\d+:\d+)\s+(?P<host>[^\s]+)\s+(?P<service>[^\[:]+)(?:\[(?P<pid>\d+)\])?:\s+(?P<message>.*)$'
        timestamp:
          parse_from: attributes.timestamp
          layout: '%b %d %H:%M:%S'

processors:
  batch:
    send_batch_size: 500
    timeout: 5s
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75

exporters:
  otlp:
    endpoint: "dataprepper:21890"
    tls:
      insecure: true
  debug:
    verbosity: basic

service:
  pipelines:
    logs:
      receivers: [filelog]
      processors: [memory_limiter, batch]
      exporters: [otlp, debug]
`,
		},
		{
			ID:          "prometheus-scraper",
			Name:        "Prometheus Endpoint Scraper",
			Description: "Periodically scrapes Prometheus-formatted endpoints (e.g. Node Exporter, Kubernetes, Cadvisor) and routes them.",
			Category:    "Metrics Collection",
			Content: `receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'node_exporter'
          scrape_interval: 15s
          static_configs:
            - targets: ['127.0.0.1:9100']
        - job_name: 'hephaestus-self'
          scrape_interval: 30s
          static_configs:
            - targets: ['127.0.0.1:8888']

processors:
  batch:
    send_batch_size: 1000
    timeout: 10s
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  debug:
    verbosity: basic

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [memory_limiter, batch]
      exporters: [prometheus, debug]
`,
		},
		{
			ID:          "all-in-one-pipeline",
			Name:        "Production All-In-One Telemetry Suite",
			Description: "Full-featured OpenTelemetry pipeline handling metrics, logs, and traces with health check, pprof, zpages, and resilient batching.",
			Category:    "Production Profiles",
			Content: `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      disk:
      filesystem:
      load:
      memory:
      network:
      process:

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
  batch:
    send_batch_size: 1024
    timeout: 5s

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  pprof:
    endpoint: 0.0.0.0:1777
  zpages:
    endpoint: 0.0.0.0:55679

exporters:
  otlp:
    endpoint: "central-collector:4317"
    tls:
      insecure: true
  prometheus:
    endpoint: "0.0.0.0:8889"
  debug:
    verbosity: basic

service:
  extensions: [health_check, pprof, zpages]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp, debug]
    metrics:
      receivers: [otlp, hostmetrics]
      processors: [memory_limiter, batch]
      exporters: [prometheus, otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp, debug]
  telemetry:
    logs:
      level: "info"
    metrics:
      address: "0.0.0.0:8888"
`,
		},
	}
}
