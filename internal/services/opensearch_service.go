package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"
)

type OpenSearchService struct {
	httpClient *http.Client
}

func NewOpenSearchService() *OpenSearchService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &OpenSearchService{
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
	}
}

func (s *OpenSearchService) RegisterWorker(wp *queue.WorkerPool) {
	wp.RegisterHandler("opensearch_poll", func(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error {
		health, err := s.GetClusterHealth(ctx)
		if err != nil {
			logger.Warn("OpenSearch", fmt.Sprintf("Auto-refresh poll failed: %v", err))
			return nil
		}
		status, _ := health["status"].(string)
		clusterName, _ := health["cluster_name"].(string)
		activeShards := health["active_shards"]
		unassigned := health["unassigned_shards"]
		logger.Info("OpenSearch", fmt.Sprintf("Background Poll: Cluster '%s' status: %s (active shards: %v, unassigned: %v)", clusterName, strings.ToUpper(status), activeShards, unassigned))
		return nil
	})
}

func (s *OpenSearchService) GetActiveConfig(ctx context.Context) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	query := `
		SELECT id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at
		FROM opensearch_configs
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT 1
	`
	var cfg domain.OpenSearchConfig
	err = pool.QueryRow(ctx, query).Scan(
		&cfg.ID, &cfg.Name, &cfg.Host, &cfg.Port, &cfg.Username,
		&cfg.Password, &cfg.UseSSL, &cfg.VerifySSL, &cfg.IsActive, &cfg.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if cfg.Password != "" {
		if decrypted, err := config.DecryptText(cfg.Password); err == nil {
			cfg.Password = decrypted
		}
	}

	return &cfg, nil
}

func (s *OpenSearchService) SaveConfig(ctx context.Context, cfg domain.OpenSearchConfig) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	if cfg.ID == "" {
		cfg.ID = "osc-primary"
	}

	var encPassword string
	if cfg.Password != "" && cfg.Password != "••••••••" && cfg.Password != "********" {
		if enc, err := config.EncryptText(cfg.Password); err == nil {
			encPassword = enc
		} else {
			encPassword = cfg.Password
		}
	}

	query := `
		INSERT INTO opensearch_configs (id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			username = EXCLUDED.username,
			password = CASE WHEN $6 != '' THEN $6 ELSE opensearch_configs.password END,
			use_ssl = EXCLUDED.use_ssl,
			verify_ssl = EXCLUDED.verify_ssl,
			is_active = EXCLUDED.is_active
	`
	_, err = pool.Exec(ctx, query, cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.Username, encPassword, cfg.UseSSL, cfg.VerifySSL, cfg.IsActive)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (s *OpenSearchService) doRequest(ctx context.Context, method, endpoint string) ([]byte, error) {
	cfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active OpenSearch configuration found: %w", err)
	}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, cfg.Host, cfg.Port, endpoint)

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach OpenSearch cluster at %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenSearch response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HTTP 401 Unauthorized: Invalid OpenSearch username or password (body: %s)", strings.TrimSpace(string(bodyBytes)))
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("HTTP 403 Forbidden: OpenSearch security plugin denied access")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d Error from OpenSearch: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	return bodyBytes, nil
}

func (s *OpenSearchService) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_cluster/health")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_cluster/health: %w (body: %s)", err, string(body))
	}
	return result, nil
}

func (s *OpenSearchService) GetNodesStats(ctx context.Context) (map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_nodes/stats")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_nodes/stats: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) GetNodesInfo(ctx context.Context) (map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_nodes")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_nodes: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) GetIndices(ctx context.Context) ([]map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_cat/indices?format=json")
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_cat/indices: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) GetShards(ctx context.Context) ([]map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_cat/shards?format=json")
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_cat/shards: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) TestConnection(ctx context.Context, host string, port int, username, password string, useSSL bool) (map[string]interface{}, error) {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/_cluster/health", scheme, host, port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HTTP 401 Unauthorized: Invalid username or password")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("HTTP 403 Forbidden: Access denied by OpenSearch security plugin")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d error: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("OpenSearch returned non-JSON response (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}
	return result, nil
}
