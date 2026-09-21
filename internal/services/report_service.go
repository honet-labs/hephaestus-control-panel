package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type ReportService struct {
	reportRepo        *repository.ReportRepository
	configRepo        *repository.ConfigRepository
	openSearchService *OpenSearchService
	promService       *PrometheusService
	httpClient        *http.Client
}

func NewReportService(
	reportRepo *repository.ReportRepository,
	configRepo *repository.ConfigRepository,
	openSearchService *OpenSearchService,
	promService *PrometheusService,
) *ReportService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &ReportService{
		reportRepo:        reportRepo,
		configRepo:        configRepo,
		openSearchService: openSearchService,
		promService:       promService,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   12 * time.Second,
		},
	}
}

// -----------------------------------------------------------------------------
// Report Management
// -----------------------------------------------------------------------------

func (s *ReportService) ListReports(ctx context.Context) ([]domain.VisualReport, error) {
	return s.reportRepo.ListReports(ctx)
}

func (s *ReportService) GetReportByID(ctx context.Context, id string) (*domain.VisualReport, error) {
	return s.reportRepo.GetReportByID(ctx, id)
}

func (s *ReportService) CreateReport(ctx context.Context, rep *domain.VisualReport) error {
	if rep.ID == "" {
		rep.ID = fmt.Sprintf("rep-%s", uuid.New().String()[:8])
	}
	if rep.HeaderConfig.Title == "" {
		rep.HeaderConfig.Title = rep.Name
	}
	if rep.HeaderConfig.LogoText == "" {
		rep.HeaderConfig.LogoText = "HEPHAESTUS"
	}
	if rep.Mode == "" {
		rep.Mode = "document"
	}
	if rep.PageOrientation == "" {
		rep.PageOrientation = "portrait"
	}
	return s.reportRepo.CreateReport(ctx, rep)
}

func (s *ReportService) UpdateReport(ctx context.Context, rep *domain.VisualReport) error {
	return s.reportRepo.UpdateReport(ctx, rep)
}

func (s *ReportService) DeleteReport(ctx context.Context, id string) error {
	return s.reportRepo.DeleteReport(ctx, id)
}

// -----------------------------------------------------------------------------
// Widget Management
// -----------------------------------------------------------------------------

func (s *ReportService) CreateWidget(ctx context.Context, w *domain.VisualReportWidget) error {
	if w.ID == "" {
		w.ID = fmt.Sprintf("wid-%s", uuid.New().String()[:8])
	}
	if w.WidthPercent == 0 {
		w.WidthPercent = 50
	}
	if w.TimeRange == "" {
		w.TimeRange = "24h"
	}
	return s.reportRepo.CreateWidget(ctx, w)
}

func (s *ReportService) UpdateWidget(ctx context.Context, w *domain.VisualReportWidget) error {
	return s.reportRepo.UpdateWidget(ctx, w)
}

func (s *ReportService) DeleteWidget(ctx context.Context, id string) error {
	return s.reportRepo.DeleteWidget(ctx, id)
}

// -----------------------------------------------------------------------------
// Data Query Resolver for Widgets (Prometheus, OpenSearch, & Grafana)
// -----------------------------------------------------------------------------

func (s *ReportService) QueryWidgetData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))

	switch sourceType {
	case "prometheus":
		return s.queryPrometheusData(ctx, req)
	case "grafana":
		return s.queryGrafanaData(ctx, req)
	case "opensearch":
		return s.queryOpenSearchData(ctx, req)
	default:
		return s.queryOpenSearchData(ctx, req)
	}
}

// queryPrometheusData handles executing PromQL queries against Prometheus
func (s *ReportService) queryPrometheusData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	promCfg, err := s.configRepo.GetActivePrometheus(ctx)
	isConnected := (err == nil && promCfg != nil && promCfg.ReloadURL != "")

	targetHosts := parseTargetHosts(req.SourceConfig)
	targetHost := targetHosts[0]

	metricPreset, _ := req.SourceConfig["metric"].(string)
	if metricPreset == "" {
		if p, ok := req.SourceConfig["presetKey"].(string); ok {
			metricPreset = p
		}
	}

	customQ, _ := req.SourceConfig["query"].(string)
	promQL, unit, metricTitle := s.buildPromQL(metricPreset, customQ, targetHost)
	if req.MetricKey != "" {
		metricTitle = req.MetricKey
	}

	timeRange := req.TimeRange
	if timeRange == "" {
		timeRange = "24h"
	}

	aggregation := ""
	if agg, ok := req.SourceConfig["aggregation"].(string); ok {
		aggregation = strings.ToLower(agg)
	} else if intv, ok := req.SourceConfig["interval"].(string); ok {
		aggregation = strings.ToLower(intv)
	}

	// Multi-host query resolution
	if len(targetHosts) > 1 && !(len(targetHosts) == 1 && targetHosts[0] == "all") {
		var seriesList []domain.ReportSeries
		var allTableRows []map[string]any
		rowID := 1
		for _, h := range targetHosts {
			promQLForH, u, mTitle := s.buildPromQL(metricPreset, customQ, h)
			if req.MetricKey != "" {
				mTitle = req.MetricKey
			}
			var hPoints []domain.ReportDataPoint
			var hSummary domain.ReportWidgetSummary
			if isConnected && s.promService != nil {
				pts, sum, qErr := s.fetchPrometheusLive(ctx, promCfg, promQLForH, timeRange, aggregation, h)
				if qErr == nil && len(pts) > 0 {
					hPoints = pts
					hSummary = sum
				}
			}
			if len(hPoints) == 0 && !isConnected {
				hPoints, hSummary = s.generateTimeSeriesData(mTitle, timeRange, "prometheus", h)
			}
			if len(hPoints) > 0 {
				hSummary.Unit = u
				seriesList = append(seriesList, domain.ReportSeries{
					Name:    fmt.Sprintf("%s - %s", h, mTitle),
					Host:    h,
					Points:  hPoints,
					Summary: hSummary,
				})
				for _, pt := range hPoints {
					allTableRows = append(allTableRows, map[string]any{
						"id":        rowID,
						"timestamp": pt.Timestamp,
						"source":    "PROMETHEUS",
						"host":      h,
						"level":     "DATA",
						"metric":    mTitle,
						"value":     fmt.Sprintf("%.2f %s", pt.Value, u),
						"message":   fmt.Sprintf("%s on %s: %.2f %s", mTitle, h, pt.Value, u),
					})
					rowID++
				}
			}
		}

		if len(seriesList) > 0 {
			return &domain.ReportQueryDataResponse{
				Title:       metricTitle,
				SourceType:  "prometheus",
				Points:      seriesList[0].Points,
				Summary:     seriesList[0].Summary,
				Series:      seriesList,
				TableRows:   allTableRows,
				IsConnected: isConnected,
				Message:     fmt.Sprintf("Live data across %d hosts from Prometheus", len(seriesList)),
			}, nil
		}
	}

	if isConnected && s.promService != nil {
		livePoints, liveSummary, err := s.fetchPrometheusLive(ctx, promCfg, promQL, timeRange, aggregation, targetHost)
		if err == nil && len(livePoints) > 0 {
			liveSummary.Unit = unit
			tableRows := make([]map[string]any, 0, len(livePoints))
			for idx, pt := range livePoints {
				tableRows = append(tableRows, map[string]any{
					"id":        idx + 1,
					"timestamp": pt.Timestamp,
					"source":    "PROMETHEUS",
					"host":      targetHost,
					"level":     "DATA",
					"metric":    metricTitle,
					"value":     fmt.Sprintf("%.2f %s", pt.Value, unit),
					"message":   fmt.Sprintf("%s on %s: %.2f %s", metricTitle, targetHost, pt.Value, unit),
				})
			}
			return &domain.ReportQueryDataResponse{
				Title:       metricTitle,
				SourceType:  "prometheus",
				Points:      livePoints,
				Summary:     liveSummary,
				Series: []domain.ReportSeries{
					{
						Name:    fmt.Sprintf("%s - %s", targetHost, metricTitle),
						Host:    targetHost,
						Points:  livePoints,
						Summary: liveSummary,
					},
				},
				TableRows:   tableRows,
				IsConnected: true,
				Message:     fmt.Sprintf("Live data from Prometheus (%s) [%s]", promCfg.Name, promQL),
			}, nil
		}
		logger.Warn("ReportService", fmt.Sprintf("Prometheus live query failed (promql: %s): %v", promQL, err))

		// If connected, never show fake baseline numbers that mislead the user
		errMsg := "No data points returned for this time range and target host"
		if err != nil {
			errMsg = fmt.Sprintf("Query error: %v", err)
		}
		return &domain.ReportQueryDataResponse{
			Title:       metricTitle,
			SourceType:  "prometheus",
			Points:      []domain.ReportDataPoint{},
			Summary:     domain.ReportWidgetSummary{Unit: unit},
			TableRows:   []map[string]any{},
			IsConnected: true,
			Message:     fmt.Sprintf("Prometheus: %s", errMsg),
		}, nil
	}

	// Demonstration data only when Prometheus is not configured at all
	points, summary := s.generateTimeSeriesData(metricTitle, timeRange, "prometheus", targetHost)
	summary.Unit = unit
	message := "Simulated Demonstration Data (Prometheus not configured)"

	tableRows := make([]map[string]any, 0, len(points))
	for idx, pt := range points {
		tableRows = append(tableRows, map[string]any{
			"id":        idx + 1,
			"timestamp": pt.Timestamp,
			"source":    "PROMETHEUS",
			"host":      targetHost,
			"level":     "DATA",
			"metric":    metricTitle,
			"value":     fmt.Sprintf("%.2f %s", pt.Value, unit),
			"message":   fmt.Sprintf("%s on %s: %.2f %s", metricTitle, targetHost, pt.Value, unit),
		})
	}

	return &domain.ReportQueryDataResponse{
		Title:       metricTitle,
		SourceType:  "prometheus",
		Points:      points,
		Summary:     summary,
		Series: []domain.ReportSeries{
			{
				Name:    fmt.Sprintf("%s - %s", targetHost, metricTitle),
				Host:    targetHost,
				Points:  points,
				Summary: summary,
			},
		},
		TableRows:   tableRows,
		IsConnected: isConnected,
		Message:     message,
	}, nil
}

// queryGrafanaData handles fetching or proxying Grafana metrics
func (s *ReportService) queryGrafanaData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	var grafanaCfg *domain.GrafanaConfig
	var err error

	if gID, ok := req.SourceConfig["grafanaId"].(string); ok && strings.TrimSpace(gID) != "" {
		grafanaCfg, err = s.configRepo.GetGrafanaByID(ctx, strings.TrimSpace(gID))
	}
	if grafanaCfg == nil {
		grafanaCfg, err = s.configRepo.GetActiveGrafana(ctx)
	}

	isConnected := (err == nil && grafanaCfg != nil && grafanaCfg.Host != "")

	// Extract requested metric title or agent
	metricKey := req.MetricKey
	if metricKey == "" {
		if m, ok := req.SourceConfig["module"].(string); ok && m != "" {
			metricKey = m
		} else if m, ok := req.SourceConfig["metric"].(string); ok && m != "" {
			metricKey = m
		} else if a, ok := req.SourceConfig["agent"].(string); ok && a != "" {
			metricKey = a
		} else {
			metricKey = "CPU Load"
		}
	}

	timeRange := req.TimeRange
	if timeRange == "" {
		timeRange = "24h"
	}

	targetHosts := parseTargetHosts(req.SourceConfig)
	targetHost := targetHosts[0]

	// Multi-host support for Grafana
	if len(targetHosts) > 1 && !(len(targetHosts) == 1 && targetHosts[0] == "all") {
		var seriesList []domain.ReportSeries
		var allTableRows []map[string]any
		rowID := 1
		for _, h := range targetHosts {
			var hPoints []domain.ReportDataPoint
			var hSummary domain.ReportWidgetSummary
			if isConnected {
				reqCopy := req
				reqCopy.SourceConfig = make(map[string]interface{})
				for k, v := range req.SourceConfig {
					reqCopy.SourceConfig[k] = v
				}
				reqCopy.SourceConfig["targetHost"] = h
				pts, sum, gErr := s.fetchGrafanaLive(ctx, grafanaCfg, reqCopy)
				if gErr == nil && len(pts) > 0 {
					hPoints = pts
					hSummary = sum
				}
			}
			if len(hPoints) == 0 && !isConnected {
				hPoints, hSummary = s.generateTimeSeriesData(metricKey, timeRange, "grafana", h)
			}
			if len(hPoints) > 0 {
				hSummary.Unit = "%"
				seriesList = append(seriesList, domain.ReportSeries{
					Name:    fmt.Sprintf("%s - %s", h, metricKey),
					Host:    h,
					Points:  hPoints,
					Summary: hSummary,
				})
				for _, pt := range hPoints {
					allTableRows = append(allTableRows, map[string]any{
						"id":        rowID,
						"timestamp": pt.Timestamp,
						"source":    "GRAFANA",
						"host":      h,
						"level":     "DATA",
						"metric":    metricKey,
						"value":     fmt.Sprintf("%.2f %%", pt.Value),
						"message":   fmt.Sprintf("%s on %s: %.2f %%", metricKey, h, pt.Value),
					})
					rowID++
				}
			}
		}

		if len(seriesList) > 0 {
			return &domain.ReportQueryDataResponse{
				Title:       metricKey,
				SourceType:  "grafana",
				Points:      seriesList[0].Points,
				Summary:     seriesList[0].Summary,
				Series:      seriesList,
				TableRows:   allTableRows,
				IsConnected: isConnected,
				Message:     fmt.Sprintf("Live data across %d hosts from Grafana", len(seriesList)),
			}, nil
		}
	}

	// Try querying live Grafana API if connection is configured
	if isConnected {
		livePoints, liveSummary, err := s.fetchGrafanaLive(ctx, grafanaCfg, req)
		if err == nil && len(livePoints) > 0 {
			tableRows := make([]map[string]any, 0, len(livePoints))
			for idx, pt := range livePoints {
				tableRows = append(tableRows, map[string]any{
					"id":        idx + 1,
					"timestamp": pt.Timestamp,
					"source":    "GRAFANA",
					"host":      targetHost,
					"level":     "DATA",
					"metric":    metricKey,
					"value":     fmt.Sprintf("%.2f %%", pt.Value),
					"message":   fmt.Sprintf("%s on %s: %.2f %%", metricKey, targetHost, pt.Value),
				})
			}
			return &domain.ReportQueryDataResponse{
				Title:       metricKey,
				SourceType:  "grafana",
				Points:      livePoints,
				Summary:     liveSummary,
				Series: []domain.ReportSeries{
					{
						Name:    fmt.Sprintf("%s - %s", targetHost, metricKey),
						Host:    targetHost,
						Points:  livePoints,
						Summary: liveSummary,
					},
				},
				TableRows:   tableRows,
				IsConnected: true,
				Message:     fmt.Sprintf("Live data from Grafana (%s)", grafanaCfg.Name),
			}, nil
		}
		logger.Warn("ReportService", fmt.Sprintf("Grafana live query failed: %v", err))

		// If connected, never show fake baseline numbers that mislead the user
		errMsg := "No data points returned for this time range and target host"
		if err != nil {
			errMsg = fmt.Sprintf("Query error: %v", err)
		}
		return &domain.ReportQueryDataResponse{
			Title:       metricKey,
			SourceType:  "grafana",
			Points:      []domain.ReportDataPoint{},
			Summary:     domain.ReportWidgetSummary{Unit: "%"},
			TableRows:   []map[string]any{},
			IsConnected: true,
			Message:     fmt.Sprintf("Grafana: %s", errMsg),
		}, nil
	}

	// Demonstration data only when Grafana is not configured at all
	points, summary := s.generateTimeSeriesData(metricKey, timeRange, "grafana", targetHost)
	message := "Simulated Demonstration Data (Grafana not configured)"

	return &domain.ReportQueryDataResponse{
		Title:       metricKey,
		SourceType:  "grafana",
		Points:      points,
		Summary:     summary,
		Series: []domain.ReportSeries{
			{
				Name:    fmt.Sprintf("%s - %s", targetHost, metricKey),
				Host:    targetHost,
				Points:  points,
				Summary: summary,
			},
		},
		IsConnected: false,
		Message:     message,
	}, nil
}

// queryOpenSearchData handles fetching or aggregating OpenSearch logs and metrics
func (s *ReportService) queryOpenSearchData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	osCfg, err := s.openSearchService.GetActiveConfig(ctx)
	isConnected := (err == nil && osCfg != nil && osCfg.Host != "")

	metricKey := req.MetricKey
	if metricKey == "" {
		if index, ok := req.SourceConfig["indexPattern"].(string); ok && index != "" {
			metricKey = fmt.Sprintf("Log Volume (%s)", index)
		} else {
			metricKey = "OpenSearch Event Volume"
		}
	}

	timeRange := req.TimeRange
	if timeRange == "" {
		timeRange = "24h"
	}

	if isConnected {
		livePoints, liveSummary, tableRows, err := s.fetchOpenSearchLive(ctx, osCfg, req)
		if err == nil && len(livePoints) > 0 {
			return &domain.ReportQueryDataResponse{
				Title:       metricKey,
				SourceType:  "opensearch",
				Points:      livePoints,
				Summary:     liveSummary,
				TableRows:   tableRows,
				IsConnected: true,
				Message:     fmt.Sprintf("Live data from OpenSearch Cluster (%s)", osCfg.Host),
			}, nil
		}
		logger.Warn("ReportService", fmt.Sprintf("OpenSearch live query fallback triggered: %v", err))
	}

	points, summary := s.generateTimeSeriesData(metricKey, timeRange, "opensearch", "")
	message := "Demonstration / Fallback Data (Configure OpenSearch Connection in Add Connections)"
	if isConnected {
		message = fmt.Sprintf("Simulated preview (Connected to OpenSearch: %s)", osCfg.Host)
	}

	tableRows := []map[string]any{
		{"timestamp": time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339), "level": "INFO", "message": "Pipeline worker ingested 4,120 events successfully", "source": "dataprepper"},
		{"timestamp": time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339), "level": "WARN", "message": "High heap utilization threshold reached (78%)", "source": "opensearch-node-1"},
		{"timestamp": time.Now().UTC().Add(-45 * time.Minute).Format(time.RFC3339), "level": "INFO", "message": "Cluster state green, all shards active", "source": "cluster-health"},
		{"timestamp": time.Now().UTC().Add(-60 * time.Minute).Format(time.RFC3339), "level": "ERROR", "message": "Connection timeout during telemetry export to external gateway", "source": "otelcol"},
	}

	return &domain.ReportQueryDataResponse{
		Title:       metricKey,
		SourceType:  "opensearch",
		Points:      points,
		Summary:     summary,
		TableRows:   tableRows,
		IsConnected: isConnected,
		Message:     message,
	}, nil
}

// fetchPrometheusLive executes range query against Prometheus server with smart host resolution
func (s *ReportService) fetchPrometheusLive(ctx context.Context, cfg *domain.PrometheusConfig, promQL, timeRange, aggregation, targetHost string) ([]domain.ReportDataPoint, domain.ReportWidgetSummary, error) {
	baseURL := strings.TrimSuffix(cfg.ReloadURL, "/-/reload")
	baseURL = strings.TrimRight(baseURL, "/")

	now := time.Now()
	var startTime time.Time
	step := "2m"

	switch timeRange {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
		step = "30s"
	case "6h":
		startTime = now.Add(-6 * time.Hour)
		step = "1m"
	case "24h":
		startTime = now.Add(-24 * time.Hour)
		step = "2m"
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
		step = "15m"
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
		step = "1h"
	default:
		startTime = now.Add(-24 * time.Hour)
		step = "2m"
	}

	if strings.Contains(aggregation, "daily") || aggregation == "day" {
		if timeRange == "7d" || timeRange == "30d" {
			step = "24h"
		} else {
			step = "1h"
		}
	} else if strings.Contains(aggregation, "weekly") || aggregation == "week" {
		if timeRange == "30d" {
			step = "168h"
		} else {
			step = "6h"
		}
	} else if strings.Contains(aggregation, "monthly") || aggregation == "month" {
		step = "24h"
	}

	labelFmt := "15:04"
	if timeRange == "7d" || timeRange == "30d" || step == "24h" || step == "168h" || step == "720h" {
		if step == "24h" || step == "168h" || step == "720h" {
			labelFmt = "01/02"
		} else {
			labelFmt = "01/02 15:04"
		}
	}

	// Build candidate endpoints to probe in order
	candidates := []string{baseURL}
	if u, parseErr := url.Parse(baseURL); parseErr == nil {
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
			if cfg.SSHHost != nil && *cfg.SSHHost != "" && *cfg.SSHHost != "localhost" && *cfg.SSHHost != "127.0.0.1" {
				candidates = append(candidates, fmt.Sprintf("http://%s:%s", *cfg.SSHHost, port))
			}
			if targetHost != "" && targetHost != "all" && targetHost != "localhost" && targetHost != "127.0.0.1" {
				candidates = append(candidates, fmt.Sprintf("http://%s:%s", targetHost, port))
			}
			if pool, dbErr := database.GetPool(); dbErr == nil && pool != nil {
				rows, qErr := pool.Query(ctx, "SELECT DISTINCT host FROM remote_host_configs WHERE host IS NOT NULL AND host != '' AND host != 'localhost' AND host != '127.0.0.1'")
				if qErr == nil {
					for rows.Next() {
						var rHost string
						if err := rows.Scan(&rHost); err == nil && rHost != "" {
							candidates = append(candidates, fmt.Sprintf("http://%s:%s", rHost, port))
						}
					}
					rows.Close()
				}
			}
		}
	}

	var uniqueCandidates []string
	seen := make(map[string]bool)
	for _, c := range candidates {
		if !seen[c] && c != "" {
			seen[c] = true
			uniqueCandidates = append(uniqueCandidates, c)
		}
	}

	var lastErr error
	for _, cand := range uniqueCandidates {
		queryURL := fmt.Sprintf(
			"%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
			cand,
			url.QueryEscape(promQL),
			startTime.Unix(),
			now.Unix(),
			step,
		)

		req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("prometheus returned status %d from %s", resp.StatusCode, cand)
			continue
		}

		var pResp struct {
			Status string `json:"status"`
			Data   struct {
				ResultType string `json:"resultType"`
				Result     []struct {
					Metric map[string]string `json:"metric"`
					Values [][]interface{}   `json:"values"`
					Value  []interface{}     `json:"value"`
				} `json:"result"`
			} `json:"data"`
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&pResp)
		resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}

		var points []domain.ReportDataPoint
		if len(pResp.Data.Result) > 0 {
			firstSeries := pResp.Data.Result[0]
			if len(firstSeries.Values) > 0 {
				for _, valPair := range firstSeries.Values {
					if len(valPair) >= 2 {
						tsFloat, _ := valPair[0].(float64)
						valStr, _ := valPair[1].(string)
						valFloat, _ := strconv.ParseFloat(valStr, 64)
						if math.IsNaN(valFloat) || math.IsInf(valFloat, 0) {
							continue
						}

						t := time.Unix(int64(tsFloat), 0)
						points = append(points, domain.ReportDataPoint{
							Timestamp: t.UTC().Format(time.RFC3339),
							Label:     t.UTC().Format(labelFmt),
							Value:     math.Round(valFloat*100) / 100,
						})
					}
				}
			} else if len(firstSeries.Value) >= 2 {
				tsFloat, _ := firstSeries.Value[0].(float64)
				valStr, _ := firstSeries.Value[1].(string)
				valFloat, _ := strconv.ParseFloat(valStr, 64)
				if math.IsNaN(valFloat) || math.IsInf(valFloat, 0) {
					continue
				}
				t := time.Unix(int64(tsFloat), 0)
				points = append(points, domain.ReportDataPoint{
					Timestamp: t.UTC().Format(time.RFC3339),
					Label:     t.UTC().Format(labelFmt),
					Value:     math.Round(valFloat*100) / 100,
				})
			}
		}

		if len(points) > 0 {
			summary := s.calculateSummary(points, "")
			logger.Info("ReportService", fmt.Sprintf("Live Prometheus query succeeded via %s (points: %d)", cand, len(points)))
			return points, summary, nil
		}
	}

	if lastErr != nil {
		return nil, domain.ReportWidgetSummary{}, lastErr
	}
	return nil, domain.ReportWidgetSummary{}, fmt.Errorf("empty series returned for query across all endpoint candidates")
}

// fetchGrafanaLive attempts to execute query against Grafana API
func (s *ReportService) fetchGrafanaLive(ctx context.Context, cfg *domain.GrafanaConfig, req domain.ReportQueryDataRequest) ([]domain.ReportDataPoint, domain.ReportWidgetSummary, error) {
	dsUID := cfg.DatasourceUID
	if customUID, ok := req.SourceConfig["datasourceUid"].(string); ok && strings.TrimSpace(customUID) != "" {
		dsUID = strings.TrimSpace(customUID)
	}

	targetHost, _ := req.SourceConfig["targetHost"].(string)
	targetHost = strings.TrimSpace(targetHost)

	expr := ""
	customQ, _ := req.SourceConfig["query"].(string)
	customQ = strings.TrimSpace(customQ)
	if customQ != "" && customQ != "*" {
		expr = customQ
	} else {
		metric := ""
		if m, ok := req.SourceConfig["metric"].(string); ok && m != "" {
			metric = m
		} else if m, ok := req.SourceConfig["module"].(string); ok && m != "" {
			metric = m
		}

		hostFilter := ""
		if targetHost != "" && targetHost != "all" {
			hostFilter = fmt.Sprintf(`, instance=~".*%s.*"`, targetHost)
		}

		switch strings.ToLower(metric) {
		case "cpu", "cpu load", "cpu usage":
			expr = fmt.Sprintf(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"%s}[5m])) * 100)`, hostFilter)
		case "memory", "memory used":
			if hostFilter != "" {
				expr = fmt.Sprintf(`(1 - (node_memory_MemAvailable_bytes{instance=~".*%s.*"} / node_memory_MemTotal_bytes{instance=~".*%s.*"})) * 100`, targetHost, targetHost)
			} else {
				expr = `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`
			}
		case "disk", "disk storage":
			if hostFilter != "" {
				expr = fmt.Sprintf(`(1 - (node_filesystem_free_bytes{mountpoint="/"%s} / node_filesystem_size_bytes{mountpoint="/"%s})) * 100`, hostFilter, hostFilter)
			} else {
				expr = `(1 - (node_filesystem_free_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100`
			}
		case "network", "network throughput", "network traffic":
			if hostFilter != "" {
				expr = fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{instance=~".*%s.*"}[5m])) * 8`, targetHost)
			} else {
				expr = `sum(rate(node_network_receive_bytes_total[5m])) * 8`
			}
		case "load", "system load":
			if hostFilter != "" {
				expr = fmt.Sprintf(`node_load1{instance=~".*%s.*"}`, targetHost)
			} else {
				expr = `node_load1`
			}
		default:
			if req.MetricKey != "" {
				expr = req.MetricKey
			} else {
				expr = fmt.Sprintf(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"%s}[5m])) * 100)`, hostFilter)
			}
		}
	}

	stepMs := 120000 // default 2m
	switch req.TimeRange {
	case "1h":
		stepMs = 30000 // 30s
	case "6h":
		stepMs = 60000 // 1m
	case "24h":
		stepMs = 120000 // 2m
	case "7d":
		stepMs = 900000 // 15m
	case "30d":
		stepMs = 3600000 // 1h
	}

	queryPayload := map[string]interface{}{
		"queries": []map[string]interface{}{
			{
				"refId": "A",
				"datasource": map[string]string{
					"uid": dsUID,
				},
				"expr":          expr,
				"format":        "time_series",
				"instant":       false,
				"range":         true,
				"intervalMs":    stepMs,
				"maxDataPoints": 1000,
			},
		},
		"from": s.parseTimeRangeStart(req.TimeRange),
		"to":   "now",
	}

	bodyBytes, _ := json.Marshal(queryPayload)

	grafanaHosts := []string{strings.TrimRight(cfg.Host, "/")}
	if u, parseErr := url.Parse(cfg.Host); parseErr == nil {
		h := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "3000"
		}
		if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "" {
			grafanaHosts = append(grafanaHosts,
				fmt.Sprintf("http://host.docker.internal:%s", port),
				fmt.Sprintf("http://172.17.0.1:%s", port),
			)
			if targetHost != "" && targetHost != "all" && targetHost != "localhost" && targetHost != "127.0.0.1" {
				grafanaHosts = append(grafanaHosts, fmt.Sprintf("http://%s:%s", targetHost, port))
			}
			if pool, dbErr := database.GetPool(); dbErr == nil && pool != nil {
				rows, qErr := pool.Query(ctx, "SELECT DISTINCT host FROM remote_host_configs WHERE host IS NOT NULL AND host != '' AND host != 'localhost' AND host != '127.0.0.1'")
				if qErr == nil {
					for rows.Next() {
						var rHost string
						if err := rows.Scan(&rHost); err == nil && rHost != "" {
							grafanaHosts = append(grafanaHosts, fmt.Sprintf("http://%s:%s", rHost, port))
						}
					}
					rows.Close()
				}
			}
		}
	}

	var uniqueHosts []string
	seen := make(map[string]bool)
	for _, h := range grafanaHosts {
		if !seen[h] && h != "" {
			seen[h] = true
			uniqueHosts = append(uniqueHosts, h)
		}
	}

	var lastErr error
	for _, baseHost := range uniqueHosts {
		endpoint := baseHost + "/api/ds/query"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
		if err != nil {
			lastErr = err
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if cfg.Token != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.Token))
		}

		resp, err := s.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("grafana returned status %d from %s", resp.StatusCode, endpoint)
			continue
		}

		var parsed struct {
			Results map[string]struct {
				Frames []struct {
					Schema struct {
						Fields []struct {
							Name string `json:"name"`
							Type string `json:"type"`
						} `json:"fields"`
					} `json:"schema"`
					Data struct {
						Values [][]interface{} `json:"values"`
					} `json:"data"`
				} `json:"frames"`
			} `json:"results"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}
		resp.Body.Close()

		var points []domain.ReportDataPoint
		for _, res := range parsed.Results {
			for _, frame := range res.Frames {
				if len(frame.Data.Values) >= 2 {
					timeVals := frame.Data.Values[0]
					numVals := frame.Data.Values[1]

					for i := 0; i < len(timeVals) && i < len(numVals); i++ {
						tFloat, _ := timeVals[i].(float64)
						vFloat, _ := numVals[i].(float64)
						if math.IsNaN(vFloat) || math.IsInf(vFloat, 0) {
							continue
						}

						tSec := int64(tFloat / 1000)
						if tFloat < 1e11 {
							tSec = int64(tFloat)
						}
						tm := time.Unix(tSec, 0)

						points = append(points, domain.ReportDataPoint{
							Timestamp: tm.UTC().Format(time.RFC3339),
							Label:     tm.UTC().Format("15:04"),
							Value:     math.Round(vFloat*100) / 100,
						})
					}
				}
			}
		}

		if len(points) > 0 {
			summary := s.calculateSummary(points, "%")
			return points, summary, nil
		}
	}

	if lastErr != nil {
		return nil, domain.ReportWidgetSummary{}, lastErr
	}
	return nil, domain.ReportWidgetSummary{}, fmt.Errorf("no time-series points returned from Grafana")
}

// fetchOpenSearchLive attempts to query OpenSearch index aggregation
func (s *ReportService) fetchOpenSearchLive(ctx context.Context, cfg *domain.OpenSearchConfig, req domain.ReportQueryDataRequest) ([]domain.ReportDataPoint, domain.ReportWidgetSummary, []map[string]any, error) {
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}

	indexPattern := "*"
	if p, ok := req.SourceConfig["indexPattern"].(string); ok && p != "" {
		indexPattern = p
	}

	endpoint := fmt.Sprintf("%s://%s:%d/%s/_search", scheme, cfg.Host, cfg.Port, indexPattern)

	interval := "1h"
	if req.TimeRange == "1h" {
		interval = "5m"
	} else if req.TimeRange == "7d" || req.TimeRange == "30d" {
		interval = "1d"
	}

	// Custom Query DSL or Lucene String support
	rawQuery := ""
	if qdsl, ok := req.SourceConfig["queryDsl"].(string); ok && strings.TrimSpace(qdsl) != "" {
		rawQuery = strings.TrimSpace(qdsl)
	} else if q, ok := req.SourceConfig["query"].(string); ok && strings.TrimSpace(q) != "" {
		rawQuery = strings.TrimSpace(q)
	} else if qk, ok := req.SourceConfig["queryKeyword"].(string); ok && strings.TrimSpace(qk) != "" {
		rawQuery = strings.TrimSpace(qk)
	}

	rangeFilter := map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": s.parseTimeRangeStart(req.TimeRange),
				"lte": "now",
			},
		},
	}

	var rootQuery map[string]interface{}
	userAggs := make(map[string]interface{})
	querySize := 25

	isDSL := false
	trimmed := strings.TrimSpace(rawQuery)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var parsedDSL map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsedDSL); err == nil {
			isDSL = true
			if sz, ok := parsedDSL["size"].(float64); ok && sz > 0 {
				querySize = int(sz)
			}
			if aggs, ok := parsedDSL["aggs"].(map[string]interface{}); ok {
				userAggs = aggs
			} else if aggs, ok := parsedDSL["aggregations"].(map[string]interface{}); ok {
				userAggs = aggs
			}

			if userQ, ok := parsedDSL["query"].(map[string]interface{}); ok {
				rootQuery = map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							rangeFilter,
							userQ,
						},
					},
				}
			} else {
				rootQuery = map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							rangeFilter,
							parsedDSL,
						},
					},
				}
			}
		}
	}

	if !isDSL {
		if rawQuery != "" && rawQuery != "*" {
			rootQuery = map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						rangeFilter,
						map[string]interface{}{
							"query_string": map[string]interface{}{
								"query": rawQuery,
							},
						},
					},
				},
			}
		} else {
			rootQuery = rangeFilter
		}
	}

	aggsPayload := map[string]interface{}{
		"size":  querySize,
		"query": rootQuery,
		"aggs": map[string]interface{}{
			"events_over_time": map[string]interface{}{
				"date_histogram": map[string]interface{}{
					"field":          "@timestamp",
					"fixed_interval": interval,
				},
			},
		},
	}

	// Merge user custom aggs if any
	for k, v := range userAggs {
		if k != "events_over_time" {
			aggsPayload["aggs"].(map[string]interface{})[k] = v
		}
	}

	bodyBytes, _ := json.Marshal(aggsPayload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, domain.ReportWidgetSummary{}, nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Username != "" && cfg.Password != "" {
		httpReq.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.ReportWidgetSummary{}, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, domain.ReportWidgetSummary{}, nil, fmt.Errorf("opensearch returned status %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var osResp struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations struct {
			EventsOverTime struct {
				Buckets []struct {
					KeyAsString string  `json:"key_as_string"`
					DocCount    float64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"events_over_time"`
		} `json:"aggregations"`
	}

	if err := json.Unmarshal(respBody, &osResp); err != nil {
		return nil, domain.ReportWidgetSummary{}, nil, err
	}

	var points []domain.ReportDataPoint
	for _, b := range osResp.Aggregations.EventsOverTime.Buckets {
		label := b.KeyAsString
		if len(label) >= 16 {
			label = label[11:16] // Extract HH:MM
		}
		points = append(points, domain.ReportDataPoint{
			Timestamp: b.KeyAsString,
			Label:     label,
			Value:     b.DocCount,
		})
	}

	if len(points) == 0 {
		return nil, domain.ReportWidgetSummary{}, nil, fmt.Errorf("no buckets returned from OpenSearch aggregation")
	}

	var tableRows []map[string]any
	for _, hit := range osResp.Hits.Hits {
		row := make(map[string]any)
		for k, v := range hit.Source {
			row[k] = v
		}
		if _, ok := row["timestamp"]; !ok {
			if ts, ok := row["@timestamp"]; ok {
				row["timestamp"] = ts
			}
		}
		if _, ok := row["level"]; !ok {
			if lvl, ok := row["log.level"]; ok {
				row["level"] = lvl
			} else if st, ok := row["status"]; ok {
				row["level"] = fmt.Sprintf("HTTP %v", st)
			} else {
				row["level"] = "INFO"
			}
		}
		if _, ok := row["message"]; !ok {
			if msg, ok := row["log"]; ok {
				row["message"] = msg
			} else if body, ok := row["body"]; ok {
				row["message"] = body
			}
		}
		tableRows = append(tableRows, row)
	}

	summary := s.calculateSummary(points, "events")
	return points, summary, tableRows, nil
}

// -----------------------------------------------------------------------------
// Helper Generators & Statistics
// -----------------------------------------------------------------------------

func fnv1a64(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func (s *ReportService) generateTimeSeriesData(title, timeRange, source, host string) ([]domain.ReportDataPoint, domain.ReportWidgetSummary) {
	numPoints := 12
	step := 2 * time.Hour
	dateFormat := "15:04"
	unit := "%"

	switch timeRange {
	case "1h":
		numPoints = 12
		step = 5 * time.Minute
		dateFormat = "15:04"
	case "6h":
		numPoints = 12
		step = 30 * time.Minute
		dateFormat = "15:04"
	case "24h":
		numPoints = 12
		step = 2 * time.Hour
		dateFormat = "15:04"
	case "7d":
		numPoints = 7
		step = 24 * time.Hour
		dateFormat = "Jan 02"
	case "30d":
		numPoints = 15
		step = 48 * time.Hour
		dateFormat = "Jan 02"
	}

	// Base values per title
	baseVal := 48.0
	unit = "%"

	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "cpu") {
		baseVal = 52.0
		unit = "%"
	} else if strings.Contains(titleLower, "memory") || strings.Contains(titleLower, "ram") {
		baseVal = 64.0
		unit = "%"
	} else if strings.Contains(titleLower, "storage") || strings.Contains(titleLower, "disk") {
		baseVal = 58.0
		unit = "%"
	} else if strings.Contains(titleLower, "log") || strings.Contains(titleLower, "event") || source == "opensearch" {
		baseVal = 1850.0
		unit = "events/min"
	} else if strings.Contains(titleLower, "network") || strings.Contains(titleLower, "traffic") {
		baseVal = 125.0
		unit = "Mbps"
	}

	// Anchor time to step boundary so refreshing within the same interval produces identical timestamps
	now := time.Now().UTC().Truncate(step)
	startTime := now.Add(-time.Duration(numPoints-1) * step)
	points := make([]domain.ReportDataPoint, 0, numPoints)

	for i := 0; i < numPoints; i++ {
		t := startTime.Add(time.Duration(i) * step)

		// Deterministic hash based on title + source + host + exact timestamp
		h := fnv1a64(fmt.Sprintf("%s|%s|%s|%d", title, source, host, t.Unix()))
		jitter := float64(h%1000) / 1000.0 // 0.0 to 1.0 (100% stable per timestamp)
		sine := math.Sin(float64(t.Unix()) / (86400.0 * 2.8))

		var val float64
		if unit == "%" {
			val = baseVal + (sine * 14.0) + ((jitter - 0.5) * 10.0)
			if val < 5.0 {
				val = 5.0
			}
			if val > 96.0 {
				val = 94.0
			}
		} else if unit == "Mbps" {
			val = baseVal + (sine * 35.0) + ((jitter - 0.5) * 20.0)
			if val < 1.0 {
				val = 1.0
			}
		} else {
			val = baseVal + (sine * 450.0) + ((jitter - 0.5) * 300.0)
			if val < 10.0 {
				val = 10.0
			}
		}

		rounded := math.Round(val*10) / 10
		points = append(points, domain.ReportDataPoint{
			Timestamp: t.UTC().Format(time.RFC3339),
			Label:     t.UTC().Format(dateFormat),
			Value:     rounded,
		})
	}

	summary := s.calculateSummary(points, unit)
	return points, summary
}

func (s *ReportService) calculateSummary(points []domain.ReportDataPoint, unit string) domain.ReportWidgetSummary {
	if len(points) == 0 {
		return domain.ReportWidgetSummary{Unit: unit}
	}

	minVal := points[0].Value
	maxVal := points[0].Value
	peakTime := points[0].Timestamp
	sum := 0.0

	for _, p := range points {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
			peakTime = p.Timestamp
		}
		sum += p.Value
	}

	avgVal := sum / float64(len(points))
	curVal := points[len(points)-1].Value

	return domain.ReportWidgetSummary{
		Min:      math.Round(minVal*100) / 100,
		Max:      math.Round(maxVal*100) / 100,
		Avg:      math.Round(avgVal*100) / 100,
		Current:  math.Round(curVal*100) / 100,
		Total:    math.Round(sum*100) / 100,
		Count:    len(points),
		Unit:     unit,
		PeakTime: peakTime,
	}
}

func (s *ReportService) parseTimeRangeStart(tr string) string {
	switch tr {
	case "1h":
		return "now-1h"
	case "6h":
		return "now-6h"
	case "24h":
		return "now-24h"
	case "7d":
		return "now-7d"
	case "30d":
		return "now-30d"
	default:
		return "now-24h"
	}
}

func parseTargetHosts(config map[string]interface{}) []string {
	var targetHosts []string
	if thList, ok := config["targetHosts"].([]interface{}); ok && len(thList) > 0 {
		for _, v := range thList {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				targetHosts = append(targetHosts, strings.TrimSpace(s))
			}
		}
	} else if ths, ok := config["targetHosts"].([]string); ok && len(ths) > 0 {
		for _, s := range ths {
			if strings.TrimSpace(s) != "" {
				targetHosts = append(targetHosts, strings.TrimSpace(s))
			}
		}
	} else if th, ok := config["targetHost"].(string); ok && th != "" {
		for _, s := range strings.Split(th, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				targetHosts = append(targetHosts, s)
			}
		}
	}
	if len(targetHosts) == 0 {
		targetHosts = []string{"all"}
	}
	return targetHosts
}

func (s *ReportService) buildPromQL(metricPreset, customQ, targetHost string) (string, string, string) {
	customQ = strings.TrimSpace(customQ)
	if customQ != "" && customQ != "*" {
		return customQ, "%", "PromQL Query"
	}

	hostFilter := ""
	if targetHost != "" && targetHost != "all" {
		hostFilter = fmt.Sprintf(`, instance=~".*%s.*"`, targetHost)
	}

	switch strings.ToLower(metricPreset) {
	case "cpu", "cpu usage", "cpu utilization", "cpu_util", "log_volume":
		if hostFilter != "" {
			return fmt.Sprintf(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"%s}[5m])) * 100)`, hostFilter), "%", "CPU Usage"
		}
		return `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, "%", "CPU Usage"
	case "memory", "memory usage", "memory used", "mem_util", "ram":
		if hostFilter != "" {
			return fmt.Sprintf(`(1 - (node_memory_MemAvailable_bytes{instance=~".*%s.*"} / node_memory_MemTotal_bytes{instance=~".*%s.*"})) * 100`, targetHost, targetHost), "%", "Memory Used"
		}
		return `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`, "%", "Memory Used"
	case "disk", "disk storage", "storage", "disk_util", "all storage":
		if hostFilter != "" {
			return fmt.Sprintf(`(1 - (node_filesystem_free_bytes{mountpoint="/"%s} / node_filesystem_size_bytes{mountpoint="/"%s})) * 100`, hostFilter, hostFilter), "%", "Disk Storage"
		}
		return `(1 - (node_filesystem_free_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100`, "%", "Disk Storage"
	case "network", "network traffic", "net_traffic":
		if hostFilter != "" {
			return fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{instance=~".*%s.*"}[5m])) * 8`, targetHost), "bps", "Network Traffic"
		}
		return `sum(rate(node_network_receive_bytes_total[5m])) * 8`, "bps", "Network Traffic"
	case "load", "system load", "sys_load":
		if hostFilter != "" {
			return fmt.Sprintf(`node_load1{instance=~".*%s.*"}`, targetHost), "load", "System Load"
		}
		return `node_load1`, "load", "System Load"
	default:
		if hostFilter != "" {
			return fmt.Sprintf(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"%s}[5m])) * 100)`, hostFilter), "%", "CPU Usage"
		}
		return `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, "%", "CPU Usage"
	}
}
