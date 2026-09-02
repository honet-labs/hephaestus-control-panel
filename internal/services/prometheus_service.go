package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
