package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-hephaestus/internal/core/domain"
)

type OpenSearchService struct {
	httpClient *http.Client
}

func NewOpenSearchService() *OpenSearchService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &OpenSearchService{
		httpClient: &http.Client{Transport: tr, Timeout: 10 * time.Second},
	}
}

func (s *OpenSearchService) GetActiveConfig(ctx context.Context) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at 
              FROM opensearch_configs WHERE is_active = true LIMIT 1`
	var c domain.OpenSearchConfig
	err = pool.QueryRow(ctx, query).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.Password, &c.UseSSL, &c.VerifySSL, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *OpenSearchService) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	cfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/_cluster/health", scheme, cfg.Host, cfg.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
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

func (s *OpenSearchService) GetNodesStats(ctx context.Context) (map[string]interface{}, error) {
	cfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/_nodes/stats", scheme, cfg.Host, cfg.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
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
