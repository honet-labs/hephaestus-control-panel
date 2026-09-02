package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
)

type OpenSearchService struct {
	httpClient *http.Client
}

func NewOpenSearchService() *OpenSearchService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &OpenSearchService{
		httpClient: &http.Client{Transport: tr, Timeout: 12 * time.Second},
	}
}

func (s *OpenSearchService) GetActiveConfig(ctx context.Context) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at 
              FROM opensearch_configs WHERE is_active = true ORDER BY created_at DESC LIMIT 1`
	var c domain.OpenSearchConfig
	err = pool.QueryRow(ctx, query).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.Password, &c.UseSSL, &c.VerifySSL, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *OpenSearchService) SaveConfig(ctx context.Context, cfg domain.OpenSearchConfig) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}

	// Deactivate others if this is active
	if cfg.IsActive {
		_, _ = pool.Exec(ctx, "UPDATE opensearch_configs SET is_active = false")
	}

	query := `
		INSERT INTO opensearch_configs (id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			username = EXCLUDED.username,
			password = EXCLUDED.password,
			use_ssl = EXCLUDED.use_ssl,
			verify_ssl = EXCLUDED.verify_ssl,
			is_active = EXCLUDED.is_active
	`
	_, err = pool.Exec(ctx, query, cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.UseSSL, cfg.VerifySSL, cfg.IsActive)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (s *OpenSearchService) doRequest(ctx context.Context, method, endpoint string) (*http.Response, error) {
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

	return s.httpClient.Do(req)
}

func (s *OpenSearchService) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	resp, err := s.doRequest(ctx, "GET", "/_cluster/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OpenSearchService) GetNodesStats(ctx context.Context) (map[string]interface{}, error) {
	resp, err := s.doRequest(ctx, "GET", "/_nodes/stats")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OpenSearchService) GetNodesInfo(ctx context.Context) (map[string]interface{}, error) {
	resp, err := s.doRequest(ctx, "GET", "/_nodes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OpenSearchService) GetIndices(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.doRequest(ctx, "GET", "/_cat/indices?format=json&bytes=b")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OpenSearchService) GetShards(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.doRequest(ctx, "GET", "/_cat/shards?format=json&bytes=b")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
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
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
