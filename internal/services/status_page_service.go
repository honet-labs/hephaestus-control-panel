package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type StatusPageService struct {
	statusPageRepo    *repository.StatusPageRepository
	topologyRepo      *repository.TopologyRepository
	remoteRepo        *repository.RemoteHostRepository
	configRepo        *repository.ConfigRepository
	openSearchService *OpenSearchService
	promService       *PrometheusService
	sshService        *SSHService
	httpClient        *http.Client
}

func NewStatusPageService(
	statusPageRepo *repository.StatusPageRepository,
	topologyRepo *repository.TopologyRepository,
	remoteRepo *repository.RemoteHostRepository,
	configRepo *repository.ConfigRepository,
	openSearchService *OpenSearchService,
	promService *PrometheusService,
	sshService *SSHService,
) *StatusPageService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
	}
	return &StatusPageService{
		statusPageRepo:    statusPageRepo,
		topologyRepo:      topologyRepo,
		remoteRepo:        remoteRepo,
		configRepo:        configRepo,
		openSearchService: openSearchService,
		promService:       promService,
		sshService:        sshService,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   4 * time.Second,
		},
	}
}

// -----------------------------------------------------------------------------
// Status Page Management
// -----------------------------------------------------------------------------

func (s *StatusPageService) ListPages(ctx context.Context) ([]domain.StatusPage, error) {
	return s.statusPageRepo.ListPages(ctx)
}

func (s *StatusPageService) GetPageByID(ctx context.Context, id string) (*domain.StatusPage, error) {
	return s.statusPageRepo.GetPageByID(ctx, id)
}

func (s *StatusPageService) GetPageBySlug(ctx context.Context, slug string) (*domain.StatusPage, error) {
	return s.statusPageRepo.GetPageBySlug(ctx, slug)
}

func (s *StatusPageService) CreatePage(ctx context.Context, page *domain.StatusPage) error {
	if strings.TrimSpace(page.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	// Sanitize slug
	page.Slug = strings.ToLower(strings.TrimSpace(page.Slug))
	page.Slug = strings.ReplaceAll(page.Slug, " ", "-")
	return s.statusPageRepo.CreatePage(ctx, page)
}

func (s *StatusPageService) UpdatePage(ctx context.Context, page *domain.StatusPage) error {
	if strings.TrimSpace(page.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	page.Slug = strings.ToLower(strings.TrimSpace(page.Slug))
	page.Slug = strings.ReplaceAll(page.Slug, " ", "-")
	return s.statusPageRepo.UpdatePage(ctx, page)
}

func (s *StatusPageService) DeletePage(ctx context.Context, id string) error {
	return s.statusPageRepo.DeletePage(ctx, id)
}

func (s *StatusPageService) SaveGroup(ctx context.Context, group *domain.StatusPageGroup) error {
	return s.statusPageRepo.SaveGroup(ctx, group)
}

func (s *StatusPageService) DeleteGroup(ctx context.Context, groupID string) error {
	return s.statusPageRepo.DeleteGroup(ctx, groupID)
}

func (s *StatusPageService) SaveItem(ctx context.Context, item *domain.StatusPageItem) error {
	return s.statusPageRepo.SaveItem(ctx, item)
}

func (s *StatusPageService) DeleteItem(ctx context.Context, itemID string) error {
	return s.statusPageRepo.DeleteItem(ctx, itemID)
}

func (s *StatusPageService) SaveIncident(ctx context.Context, incident *domain.StatusPageIncident) error {
	return s.statusPageRepo.SaveIncident(ctx, incident)
}

func (s *StatusPageService) DeleteIncident(ctx context.Context, incidentID string) error {
	return s.statusPageRepo.DeleteIncident(ctx, incidentID)
}

// -----------------------------------------------------------------------------
// Source Options for Picker
// -----------------------------------------------------------------------------

func (s *StatusPageService) GetSourceOptions(ctx context.Context) ([]domain.StatusPageSourceOption, error) {
	var options []domain.StatusPageSourceOption
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	// 1. Network Topology Devices
	devRows, err := pool.Query(ctx, `
		SELECT d.id, d.name, d.ip_address, COALESCE(d.device_type, 'device'), COALESCE(p.reachable, false)
		FROM topology_devices d
		LEFT JOIN device_ping_results p ON d.id = p.device_id
		ORDER BY d.name ASC
	`)
	if err == nil {
		defer devRows.Close()
		for devRows.Next() {
			var id, name, ip, devType string
			var reachable bool
			if err := devRows.Scan(&id, &name, &ip, &devType, &reachable); err == nil {
				st := "offline"
				if reachable {
					st = "online"
				}
				options = append(options, domain.StatusPageSourceOption{
					ID:         id,
					Name:       fmt.Sprintf("%s (%s)", name, ip),
					SourceType: "topology",
					Detail:     fmt.Sprintf("Type: %s", strings.ToUpper(devType)),
					IPOrHost:   ip,
					Status:     st,
				})
			}
		}
	}

	// 2. OpenSearch Clusters
	osRows, err := pool.Query(ctx, `
		SELECT id, name, host, port, is_active
		FROM opensearch_configs
		ORDER BY name ASC
	`)
	if err == nil {
		defer osRows.Close()
		for osRows.Next() {
			var id, name, host string
			var port int
			var active bool
			if err := osRows.Scan(&id, &name, &host, &port, &active); err == nil {
				options = append(options, domain.StatusPageSourceOption{
					ID:         id,
					Name:       fmt.Sprintf("OpenSearch: %s", name),
					SourceType: "opensearch",
					Detail:     fmt.Sprintf("%s:%d", host, port),
					IPOrHost:   fmt.Sprintf("%s:%d", host, port),
					Status:     "configured",
				})
			}
		}
	}

	// 3. Prometheus Instances
	promRows, err := pool.Query(ctx, `
		SELECT id, name, mode, reload_url, is_active
		FROM prometheus_configs
		WHERE LOWER(name) NOT LIKE '%data prepper%'
		ORDER BY name ASC
	`)
	if err == nil {
		defer promRows.Close()
		for promRows.Next() {
			var id, name, mode, reloadURL string
			var active bool
			if err := promRows.Scan(&id, &name, &mode, &reloadURL, &active); err == nil {
				options = append(options, domain.StatusPageSourceOption{
					ID:         id,
					Name:       fmt.Sprintf("Prometheus: %s", name),
					SourceType: "prometheus",
					Detail:     fmt.Sprintf("Mode: %s", mode),
					IPOrHost:   reloadURL,
					Status:     "configured",
				})
			}
		}
	}

	// 4. Grafana Instances
	grafRows, err := pool.Query(ctx, `
		SELECT id, name, host, is_active
		FROM grafana_configs
		ORDER BY name ASC
	`)
	if err == nil {
		defer grafRows.Close()
		for grafRows.Next() {
			var id, name, host string
			var active bool
			if err := grafRows.Scan(&id, &name, &host, &active); err == nil {
				options = append(options, domain.StatusPageSourceOption{
					ID:         id,
					Name:       fmt.Sprintf("Grafana: %s", name),
					SourceType: "grafana",
					Detail:     host,
					IPOrHost:   host,
					Status:     "configured",
				})
			}
		}
	}

	// 5. Remote Servers (SSH)
	sshRows, err := pool.Query(ctx, `
		SELECT id, name, host, port, username
		FROM remote_host_configs
		ORDER BY name ASC
	`)
	if err == nil {
		defer sshRows.Close()
		for sshRows.Next() {
			var id, name, host, user string
			var port int
			if err := sshRows.Scan(&id, &name, &host, &port, &user); err == nil {
				options = append(options, domain.StatusPageSourceOption{
					ID:         id,
					Name:       fmt.Sprintf("Server: %s (%s)", name, host),
					SourceType: "remote_server",
					Detail:     fmt.Sprintf("%s@%s:%d", user, host, port),
					IPOrHost:   host,
					Status:     "configured",
				})
			}
		}
	}

	return options, nil
}

// -----------------------------------------------------------------------------
// Live Status Evaluation & Report
// -----------------------------------------------------------------------------

func (s *StatusPageService) GetLiveReport(ctx context.Context, slugOrID string, isSlug bool) (*domain.StatusPageLiveReport, error) {
	var page *domain.StatusPage
	var err error

	if isSlug {
		page, err = s.statusPageRepo.GetPageBySlug(ctx, slugOrID)
	} else {
		page, err = s.statusPageRepo.GetPageByID(ctx, slugOrID)
	}
	if err != nil || page == nil {
		return nil, fmt.Errorf("status page not found: %w", err)
	}

	// Fetch active incidents
	incidents, _ := s.statusPageRepo.ListIncidents(ctx, page.ID, true)

	// Evaluate all items concurrently
	items := page.Items
	results := make([]domain.StatusItemLiveResult, len(items))

	var wg sync.WaitGroup
	// Concurrency limiter to avoid connection exhaustion
	sem := make(chan struct{}, 10)

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it domain.StatusPageItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			evalCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			results[idx] = s.evaluateItem(evalCtx, it)
		}(i, item)
	}
	wg.Wait()

	// Organize into groups
	groupMap := make(map[string]*domain.StatusPageGroupReport)
	for _, g := range page.Groups {
		groupMap[g.ID] = &domain.StatusPageGroupReport{
			ID:        g.ID,
			Name:      g.Name,
			SortOrder: g.SortOrder,
			Items:     []domain.StatusItemLiveResult{},
		}
	}

	var ungroupedItems []domain.StatusItemLiveResult
	for _, res := range results {
		if res.GroupID != nil && *res.GroupID != "" {
			if grp, exists := groupMap[*res.GroupID]; exists {
				grp.Items = append(grp.Items, res)
				continue
			}
		}
		ungroupedItems = append(ungroupedItems, res)
	}

	var orderedGroups []domain.StatusPageGroupReport
	for _, g := range page.Groups {
		if grp, exists := groupMap[g.ID]; exists {
			orderedGroups = append(orderedGroups, *grp)
		}
	}

	// Compute overall status
	overallStatus := "operational"
	overallMsg := "All Systems Operational"

	// 1. Check active incidents
	hasCriticalIncident := false
	hasMinorIncident := false
	hasMaintenanceIncident := false

	for _, inc := range incidents {
		if strings.EqualFold(inc.Status, "maintenance") {
			hasMaintenanceIncident = true
		} else if strings.EqualFold(inc.Severity, "critical") || strings.EqualFold(inc.Severity, "major") {
			hasCriticalIncident = true
		} else {
			hasMinorIncident = true
		}
	}

	// 2. Count degraded / down items
	downCount := 0
	degradedCount := 0
	totalItems := len(results)

	for _, res := range results {
		if res.Status == "down" {
			downCount++
		} else if res.Status == "degraded" {
			degradedCount++
		}
	}

	if hasCriticalIncident || downCount > 0 {
		overallStatus = "major_outage"
		if downCount > 0 {
			overallMsg = fmt.Sprintf("Service Outage Detected (%d of %d down)", downCount, totalItems)
		} else {
			overallMsg = "Major Incident in Progress"
		}
	} else if hasMinorIncident || degradedCount > 0 {
		overallStatus = "partial_outage"
		overallMsg = "Partial System Degradation Detected"
	} else if hasMaintenanceIncident {
		overallStatus = "maintenance"
		overallMsg = "Scheduled Maintenance in Progress"
	}

	return &domain.StatusPageLiveReport{
		PageID:          page.ID,
		Title:           page.Title,
		Slug:            page.Slug,
		Description:     page.Description,
		FooterText:      page.FooterText,
		Theme:           page.Theme,
		RefreshInterval: page.RefreshInterval,
		IsPublic:        page.IsPublic,
		OverallStatus:   overallStatus,
		OverallMessage:  overallMsg,
		ActiveIncidents: incidents,
		Groups:          orderedGroups,
		UngroupedItems:  ungroupedItems,
		LastChecked:     time.Now().UTC(),
	}, nil
}

func (s *StatusPageService) evaluateItem(ctx context.Context, item domain.StatusPageItem) domain.StatusItemLiveResult {
	res := domain.StatusItemLiveResult{
		ItemID:      item.ID,
		Name:        item.Name,
		GroupID:     item.GroupID,
		SourceType:  item.SourceType,
		SourceID:    item.SourceID,
		Status:      "operational",
		Description: item.Description,
		Details:     make(map[string]interface{}),
		CheckedAt:   time.Now().UTC(),
	}

	switch strings.ToLower(item.SourceType) {
	case "topology":
		s.evalTopology(ctx, item, &res)
	case "opensearch":
		s.evalOpenSearch(ctx, item, &res)
	case "prometheus":
		s.evalPrometheus(ctx, item, &res)
	case "grafana":
		s.evalGrafana(ctx, item, &res)
	case "remote_server":
		s.evalRemoteServer(ctx, item, &res)
	default:
		res.Status = "operational"
		res.Message = "Operational"
	}

	return res
}

func (s *StatusPageService) evalTopology(ctx context.Context, item domain.StatusPageItem, res *domain.StatusItemLiveResult) {
	if item.SourceID == nil || *item.SourceID == "" {
		res.Status = "unknown"
		res.Message = "Device not configured"
		return
	}

	pool, err := database.GetPool()
	if err != nil {
		res.Status = "down"
		res.Message = "Database unavailable"
		return
	}

	var reachable bool
	var latencyMs *float64
	var checkedAt time.Time
	err = pool.QueryRow(ctx, `
		SELECT reachable, latency_ms, checked_at 
		FROM device_ping_results 
		WHERE device_id = $1
	`, *item.SourceID).Scan(&reachable, &latencyMs, &checkedAt)

	if err != nil {
		// Device might exist but no ping entry yet
		res.Status = "operational"
		res.Message = "Configured (Awaiting ping sweep)"
		return
	}

	if reachable {
		res.Status = "operational"
		res.LatencyMs = latencyMs
		if latencyMs != nil {
			res.Message = fmt.Sprintf("Operational (%.1f ms)", *latencyMs)
		} else {
			res.Message = "Operational"
		}
	} else {
		res.Status = "down"
		res.Message = "Device unreachable"
	}
}

func (s *StatusPageService) evalOpenSearch(ctx context.Context, item domain.StatusPageItem, res *domain.StatusItemLiveResult) {
	pool, err := database.GetPool()
	if err != nil {
		res.Status = "down"
		res.Message = "Database unavailable"
		return
	}

	var host, username, password string
	var port int
	var useSSL, verifySSL bool

	query := `SELECT host, port, username, password, use_ssl, verify_ssl FROM opensearch_configs WHERE is_active = true LIMIT 1`
	if item.SourceID != nil && *item.SourceID != "" {
		query = fmt.Sprintf(`SELECT host, port, username, password, use_ssl, verify_ssl FROM opensearch_configs WHERE id = '%s'`, *item.SourceID)
	}

	err = pool.QueryRow(ctx, query).Scan(&host, &port, &username, &password, &useSSL, &verifySSL)
	if err != nil {
		res.Status = "down"
		res.Message = "OpenSearch connection not configured"
		return
	}

	protocol := "http"
	if useSSL {
		protocol = "https"
	}
	healthURL := fmt.Sprintf("%s://%s:%d/_cluster/health", protocol, host, port)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		res.Status = "down"
		res.Message = err.Error()
		return
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	durationMs := float64(time.Since(start).Milliseconds())
	res.LatencyMs = &durationMs

	if err != nil {
		res.Status = "down"
		res.Message = "Cluster unreachable"
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res.Status = "operational"
		res.Message = fmt.Sprintf("Cluster Healthy (HTTP %d, %.0f ms)", resp.StatusCode, durationMs)
	} else {
		res.Status = "degraded"
		res.Message = fmt.Sprintf("Cluster Warning (HTTP %d)", resp.StatusCode)
	}
}

func (s *StatusPageService) evalPrometheus(ctx context.Context, item domain.StatusPageItem, res *domain.StatusItemLiveResult) {
	pool, err := database.GetPool()
	if err != nil {
		res.Status = "down"
		res.Message = "Database unavailable"
		return
	}

	var reloadURL, mode string
	query := `SELECT reload_url, mode FROM prometheus_configs WHERE is_active = true LIMIT 1`
	if item.SourceID != nil && *item.SourceID != "" {
		query = fmt.Sprintf(`SELECT reload_url, mode FROM prometheus_configs WHERE id = '%s'`, *item.SourceID)
	}

	err = pool.QueryRow(ctx, query).Scan(&reloadURL, &mode)
	if err != nil {
		res.Status = "operational"
		res.Message = "Prometheus service configured"
		return
	}

	// Check base URL
	baseURL := strings.TrimSuffix(reloadURL, "/-/reload")
	if baseURL == "" {
		baseURL = "http://localhost:9090"
	}
	healthURL := baseURL + "/-/healthy"

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		res.Status = "operational"
		res.Message = "Prometheus target ready"
		return
	}

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	durationMs := float64(time.Since(start).Milliseconds())
	res.LatencyMs = &durationMs

	if err != nil || (resp.StatusCode < 200 || resp.StatusCode >= 400) {
		// Fallback to checking root
		reqRoot, _ := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
		respRoot, errRoot := s.httpClient.Do(reqRoot)
		if errRoot == nil && respRoot.StatusCode < 500 {
			res.Status = "operational"
			res.Message = fmt.Sprintf("Prometheus Online (%.0f ms)", durationMs)
			respRoot.Body.Close()
			return
		}
		res.Status = "degraded"
		res.Message = "Prometheus check degraded"
		return
	}
	defer resp.Body.Close()

	res.Status = "operational"
	res.Message = fmt.Sprintf("Prometheus Healthy (%.0f ms)", durationMs)
}

func (s *StatusPageService) evalGrafana(ctx context.Context, item domain.StatusPageItem, res *domain.StatusItemLiveResult) {
	pool, err := database.GetPool()
	if err != nil {
		res.Status = "down"
		res.Message = "Database unavailable"
		return
	}

	var host, token string
	query := `SELECT host, token FROM grafana_configs WHERE is_active = true LIMIT 1`
	if item.SourceID != nil && *item.SourceID != "" {
		query = fmt.Sprintf(`SELECT host, token FROM grafana_configs WHERE id = '%s'`, *item.SourceID)
	}

	err = pool.QueryRow(ctx, query).Scan(&host, &token)
	if err != nil {
		res.Status = "operational"
		res.Message = "Grafana configured"
		return
	}

	healthURL := strings.TrimRight(host, "/") + "/api/health"
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		res.Status = "down"
		res.Message = err.Error()
		return
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	durationMs := float64(time.Since(start).Milliseconds())
	res.LatencyMs = &durationMs

	if err != nil {
		res.Status = "down"
		res.Message = "Grafana unreachable"
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		res.Status = "operational"
		res.Message = fmt.Sprintf("Grafana Healthy (HTTP %d, %.0f ms)", resp.StatusCode, durationMs)
	} else {
		res.Status = "degraded"
		res.Message = fmt.Sprintf("Grafana returned HTTP %d", resp.StatusCode)
	}
}

func (s *StatusPageService) evalRemoteServer(ctx context.Context, item domain.StatusPageItem, res *domain.StatusItemLiveResult) {
	if item.SourceID == nil || *item.SourceID == "" {
		res.Status = "unknown"
		res.Message = "Server not selected"
		return
	}

	pool, err := database.GetPool()
	if err != nil {
		res.Status = "down"
		res.Message = "Database unavailable"
		return
	}

	var host string
	var port int
	err = pool.QueryRow(ctx, `SELECT host, port FROM remote_host_configs WHERE id = $1`, *item.SourceID).Scan(&host, &port)
	if err != nil {
		res.Status = "down"
		res.Message = "Host profile not found"
		return
	}

	// Perform fast TCP dial to SSH port
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	durationMs := float64(time.Since(start).Milliseconds())
	res.LatencyMs = &durationMs

	if err != nil {
		res.Status = "down"
		res.Message = fmt.Sprintf("SSH Port %d unreachable", port)
		return
	}
	conn.Close()

	res.Status = "operational"
	res.Message = fmt.Sprintf("SSH Accessible (%.0f ms)", durationMs)
}
