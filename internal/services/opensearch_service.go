package services

import (
	"bytes"
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
		cfg, err := s.GetActiveConfig(ctx)
		if err != nil || cfg == nil || !cfg.IsActive || cfg.Host == "" {
			// Silently skip if OpenSearch is not configured or not active
			return nil
		}

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

// IsMaskedOrEmptyPassword returns true if the password is empty or composed of masking characters (bullets, asterisks).
func IsMaskedOrEmptyPassword(p string) bool {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return true
	}
	if trimmed == "••••••••" || trimmed == "••••••" || trimmed == "********" || trimmed == "******" {
		return true
	}
	isMasked := true
	for _, r := range trimmed {
		if r != '*' && r != '•' && r != '\u2022' && r != '·' {
			isMasked = false
			break
		}
	}
	return isMasked
}

func (s *OpenSearchService) GetActiveConfig(ctx context.Context) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	query := `
		SELECT id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at,
		       user_id, COALESCE(visibility, 'private')
		FROM opensearch_configs
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT 1
	`
	var cfg domain.OpenSearchConfig
	err = pool.QueryRow(ctx, query).Scan(
		&cfg.ID, &cfg.Name, &cfg.Host, &cfg.Port, &cfg.Username,
		&cfg.Password, &cfg.UseSSL, &cfg.VerifySSL, &cfg.IsActive, &cfg.CreatedAt,
		&cfg.UserID, &cfg.Visibility,
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

func (s *OpenSearchService) GetActiveConfigForUser(ctx context.Context, userID int, userRole string) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	isAdmin := domain.IsAdminRole(userRole)
	var query string
	var args []interface{}

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		query = `
			SELECT c.id, c.name, c.host, c.port, c.username, c.password, c.use_ssl, c.verify_ssl, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM opensearch_shares WHERE config_id = c.id) AS shares_count
			FROM opensearch_configs c
			LEFT JOIN users u ON c.user_id = u.id
			WHERE c.is_active = true
			ORDER BY c.created_at DESC
			LIMIT 1
		`
	} else {
		query = `
			SELECT c.id, c.name, c.host, c.port, c.username, c.password, c.use_ssl, c.verify_ssl, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM opensearch_shares WHERE config_id = c.id) AS shares_count,
			       COALESCE(oss.permission, '') AS share_perm
			FROM opensearch_configs c
			LEFT JOIN users u ON c.user_id = u.id
			LEFT JOIN opensearch_shares oss ON c.id = oss.config_id AND oss.user_id = $1
			WHERE c.is_active = true
			  AND (c.visibility = 'public' OR c.user_id = $1 OR oss.user_id = $1)
			ORDER BY (c.user_id = $1) DESC, c.created_at DESC
			LIMIT 1
		`
		args = append(args, userID)
	}

	var cfg domain.OpenSearchConfig
	var sharePerm string
	var errScan error

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		errScan = pool.QueryRow(ctx, query).Scan(
			&cfg.ID, &cfg.Name, &cfg.Host, &cfg.Port, &cfg.Username,
			&cfg.Password, &cfg.UseSSL, &cfg.VerifySSL, &cfg.IsActive, &cfg.CreatedAt,
			&cfg.UserID, &cfg.OwnerUsername, &cfg.Visibility, &cfg.SharesCount,
		)
		cfg.IsOwner = true
		cfg.UserPermission = "manage"
	} else {
		errScan = pool.QueryRow(ctx, query, args...).Scan(
			&cfg.ID, &cfg.Name, &cfg.Host, &cfg.Port, &cfg.Username,
			&cfg.Password, &cfg.UseSSL, &cfg.VerifySSL, &cfg.IsActive, &cfg.CreatedAt,
			&cfg.UserID, &cfg.OwnerUsername, &cfg.Visibility, &cfg.SharesCount, &sharePerm,
		)
		cfg.IsOwner = (cfg.UserID != nil && *cfg.UserID == userID)
		if cfg.IsOwner {
			cfg.UserPermission = "manage"
		} else if sharePerm != "" {
			cfg.UserPermission = sharePerm
		} else {
			cfg.UserPermission = "read"
		}
	}

	if errScan != nil {
		return nil, errScan
	}

	if cfg.Password != "" {
		if decrypted, err := config.DecryptText(cfg.Password); err == nil {
			cfg.Password = decrypted
		}
	}

	return &cfg, nil
}

func (s *OpenSearchService) ListConfigs(ctx context.Context, userID int, userRole string) ([]domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	isAdmin := domain.IsAdminRole(userRole)
	var list []domain.OpenSearchConfig

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		query := `
			SELECT c.id, c.name, c.host, c.port, c.username, c.password, c.use_ssl, c.verify_ssl, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM opensearch_shares WHERE config_id = c.id) AS shares_count
			FROM opensearch_configs c
			LEFT JOIN users u ON c.user_id = u.id
			ORDER BY c.is_active DESC, c.name ASC
		`
		rows, err := pool.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.OpenSearchConfig
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Host, &c.Port, &c.Username,
				&c.Password, &c.UseSSL, &c.VerifySSL, &c.IsActive, &c.CreatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount,
			); err != nil {
				return nil, err
			}
			c.IsOwner = true
			c.UserPermission = "manage"
			list = append(list, c)
		}
	} else {
		query := `
			SELECT c.id, c.name, c.host, c.port, c.username, c.password, c.use_ssl, c.verify_ssl, c.is_active, c.created_at,
			       c.user_id, COALESCE(u.username, 'Admin') AS owner_username, COALESCE(c.visibility, 'private') AS visibility,
			       (SELECT COUNT(*) FROM opensearch_shares WHERE config_id = c.id) AS shares_count,
			       COALESCE(oss.permission, '') AS share_perm
			FROM opensearch_configs c
			LEFT JOIN users u ON c.user_id = u.id
			LEFT JOIN opensearch_shares oss ON c.id = oss.config_id AND oss.user_id = $1
			WHERE c.visibility = 'public'
			   OR c.user_id = $1
			   OR oss.user_id = $1
			ORDER BY c.is_active DESC, c.name ASC
		`
		rows, err := pool.Query(ctx, query, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c domain.OpenSearchConfig
			var sharePerm string
			if err := rows.Scan(
				&c.ID, &c.Name, &c.Host, &c.Port, &c.Username,
				&c.Password, &c.UseSSL, &c.VerifySSL, &c.IsActive, &c.CreatedAt,
				&c.UserID, &c.OwnerUsername, &c.Visibility, &c.SharesCount, &sharePerm,
			); err != nil {
				return nil, err
			}
			c.IsOwner = (c.UserID != nil && *c.UserID == userID)
			if c.IsOwner {
				c.UserPermission = "manage"
			} else if sharePerm != "" {
				c.UserPermission = sharePerm
			} else {
				c.UserPermission = "read"
			}
			list = append(list, c)
		}
	}

	if list == nil {
		list = []domain.OpenSearchConfig{}
	}
	return list, nil
}

func (s *OpenSearchService) GetConfigByID(ctx context.Context, id string) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	query := `
		SELECT id, name, host, port, username, password, use_ssl, verify_ssl, is_active, created_at,
		       user_id, COALESCE(visibility, 'private')
		FROM opensearch_configs
		WHERE id = $1
		LIMIT 1
	`
	var cfg domain.OpenSearchConfig
	err = pool.QueryRow(ctx, query, id).Scan(
		&cfg.ID, &cfg.Name, &cfg.Host, &cfg.Port, &cfg.Username,
		&cfg.Password, &cfg.UseSSL, &cfg.VerifySSL, &cfg.IsActive, &cfg.CreatedAt,
		&cfg.UserID, &cfg.Visibility,
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

func (s *OpenSearchService) DeleteConfig(ctx context.Context, id string, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return fmt.Errorf("database connection unavailable: %w", err)
	}

	if !domain.IsAdminRole(userRole) && userID > 0 {
		// Non-admin can only delete if owner or manage permission
		var ownerID *int
		errCheck := pool.QueryRow(ctx, "SELECT user_id FROM opensearch_configs WHERE id = $1", id).Scan(&ownerID)
		if errCheck != nil {
			return fmt.Errorf("configuration not found")
		}
		if ownerID == nil || *ownerID != userID {
			// Check manage share
			var perm string
			errShare := pool.QueryRow(ctx, "SELECT permission FROM opensearch_shares WHERE config_id = $1 AND user_id = $2", id, userID).Scan(&perm)
			if errShare != nil || perm != "manage" {
				return fmt.Errorf("you do not have permission to delete this configuration")
			}
		}
	}

	if id != "" && id != "opensearch-active" {
		_, err = pool.Exec(ctx, "DELETE FROM opensearch_configs WHERE id = $1", id)
	} else {
		_, err = pool.Exec(ctx, "DELETE FROM opensearch_configs WHERE is_active = true")
	}
	return err
}

func (s *OpenSearchService) SaveConfig(ctx context.Context, cfg domain.OpenSearchConfig, userID int, userRole string) (*domain.OpenSearchConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable: %w", err)
	}

	isAdmin := domain.IsAdminRole(userRole)

	// Check if updating existing
	if cfg.ID != "" && cfg.ID != "opensearch-active" {
		var existingOwner *int
		errExist := pool.QueryRow(ctx, "SELECT user_id FROM opensearch_configs WHERE id = $1", cfg.ID).Scan(&existingOwner)
		if errExist == nil {
			// Updating existing config: verify permission
			if !isAdmin && userID > 0 {
				if existingOwner == nil || *existingOwner != userID {
					var perm string
					errShare := pool.QueryRow(ctx, "SELECT permission FROM opensearch_shares WHERE config_id = $1 AND user_id = $2", cfg.ID, userID).Scan(&perm)
					if errShare != nil || perm != "manage" {
						return nil, fmt.Errorf("you do not have permission to edit this configuration")
					}
				}
			}
		} else {
			// Config with ID does not exist, will create
		}
	} else {
		// New config
		if cfg.ID == "" || cfg.ID == "opensearch-active" {
			cfg.ID = fmt.Sprintf("osc-%d", time.Now().UnixNano())
		}
	}

	if cfg.Visibility == "" {
		cfg.Visibility = "private"
	}

	var encPassword string
	if !IsMaskedOrEmptyPassword(cfg.Password) {
		if enc, err := config.EncryptText(cfg.Password); err == nil {
			encPassword = enc
		} else {
			encPassword = cfg.Password
		}
	}

	var assignedUserID *int
	if userID > 0 {
		assignedUserID = &userID
	}

	query := `
		INSERT INTO opensearch_configs (id, name, host, port, username, password, use_ssl, verify_ssl, is_active, user_id, visibility, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			username = EXCLUDED.username,
			password = CASE WHEN $6 != '' THEN $6 ELSE opensearch_configs.password END,
			use_ssl = EXCLUDED.use_ssl,
			verify_ssl = EXCLUDED.verify_ssl,
			is_active = EXCLUDED.is_active,
			visibility = COALESCE(NULLIF(EXCLUDED.visibility, ''), opensearch_configs.visibility)
	`
	_, err = pool.Exec(ctx, query, cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.Username, encPassword, cfg.UseSSL, cfg.VerifySSL, cfg.IsActive, assignedUserID, cfg.Visibility)
	if err != nil {
		return nil, err
	}

	cfg.UserID = assignedUserID
	cfg.IsOwner = true
	cfg.UserPermission = "manage"

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

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.VerifySSL},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Do(req)
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
	body, err := s.doRequest(ctx, "GET", "/_cat/shards?h=index,shard,prirep,state,docs,store,ip,node,unassigned.reason,unassigned.for&format=json")
	if err != nil {
		body, err = s.doRequest(ctx, "GET", "/_cat/shards?format=json")
		if err != nil {
			return nil, err
		}
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_cat/shards: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) GetRecovery(ctx context.Context) ([]map[string]interface{}, error) {
	body, err := s.doRequest(ctx, "GET", "/_cat/recovery?active_only=true&format=json")
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from /_cat/recovery: %w", err)
	}
	return result, nil
}

func (s *OpenSearchService) TestConnection(ctx context.Context, host string, port int, username, password string, useSSL bool, verifySSL bool) (map[string]interface{}, error) {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/_cluster/health", scheme, host, port)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifySSL},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
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

// ExecuteQuery executes a direct query or command against the active OpenSearch cluster (Dev Tools / Query Workbench)
func (s *OpenSearchService) ExecuteQuery(ctx context.Context, method, endpoint string, body []byte) (*domain.OpenSearchQueryResult, error) {
	cfg, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active OpenSearch configuration found: %w", err)
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}

	endpoint = strings.TrimSpace(endpoint)
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, cfg.Host, cfg.Port, endpoint)

	var reqBody io.Reader
	if len(body) > 0 && method != "GET" && method != "HEAD" {
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 && method != "GET" && method != "HEAD" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.VerifySSL},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   35 * time.Second,
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	latencyMs := time.Since(startTime).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to reach OpenSearch cluster at %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenSearch response: %w", err)
	}

	res := &domain.OpenSearchQueryResult{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		LatencyMs:  latencyMs,
		Raw:        string(bodyBytes),
	}

	// Try to decode JSON
	var parsed interface{}
	if jsonErr := json.Unmarshal(bodyBytes, &parsed); jsonErr == nil {
		res.Data = parsed
	} else {
		res.Data = string(bodyBytes)
	}

	return res, nil
}
