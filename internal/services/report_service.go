package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type ReportService struct {
	reportRepo        *repository.ReportRepository
	configRepo        *repository.ConfigRepository
	openSearchService *OpenSearchService
	httpClient        *http.Client
}

func NewReportService(
	reportRepo *repository.ReportRepository,
	configRepo *repository.ConfigRepository,
	openSearchService *OpenSearchService,
) *ReportService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &ReportService{
		reportRepo:        reportRepo,
		configRepo:        configRepo,
		openSearchService: openSearchService,
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
// Data Query Resolver for Widgets (Grafana & OpenSearch)
// -----------------------------------------------------------------------------

func (s *ReportService) QueryWidgetData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))

	switch sourceType {
	case "grafana":
		return s.queryGrafanaData(ctx, req)
	case "opensearch":
		return s.queryOpenSearchData(ctx, req)
	default:
		return s.queryOpenSearchData(ctx, req)
	}
}

// queryGrafanaData handles fetching or proxying Grafana metrics
func (s *ReportService) queryGrafanaData(ctx context.Context, req domain.ReportQueryDataRequest) (*domain.ReportQueryDataResponse, error) {
	grafanaCfg, err := s.configRepo.GetActiveGrafanaConfig(ctx)
	isConnected := (err == nil && grafanaCfg != nil && grafanaCfg.Host != "")

	// Extract requested metric title or agent
	metricKey := req.MetricKey
	if metricKey == "" {
		if m, ok := req.SourceConfig["module"].(string); ok && m != "" {
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

	// Try querying live Grafana API if connection is configured
	if isConnected {
		livePoints, liveSummary, err := s.fetchGrafanaLive(ctx, grafanaCfg, req)
		if err == nil && len(livePoints) > 0 {
			return &domain.ReportQueryDataResponse{
				Title:       metricKey,
				SourceType:  "grafana",
				Points:      livePoints,
				Summary:     liveSummary,
				IsConnected: true,
				Message:     fmt.Sprintf("Live data from Grafana (%s)", grafanaCfg.Name),
			}, nil
		}
		logger.Warn("ReportService", fmt.Sprintf("Grafana live query fallback triggered: %v", err))
	}

	// High-fidelity fallback generator if Grafana is unreachable or target metric is pending
	points, summary := s.generateTimeSeriesData(metricKey, timeRange, "grafana")
	message := "Demonstration / Fallback Data (Configure Grafana Connection in Add Connections)"
	if isConnected {
		message = fmt.Sprintf("Simulated preview (Connected to Grafana: %s)", grafanaCfg.Name)
	}

	return &domain.ReportQueryDataResponse{
		Title:       metricKey,
		SourceType:  "grafana",
		Points:      points,
		Summary:     summary,
		IsConnected: isConnected,
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

	points, summary := s.generateTimeSeriesData(metricKey, timeRange, "opensearch")
	message := "Demonstration / Fallback Data (Configure OpenSearch Connection in Add Connections)"
	if isConnected {
		message = fmt.Sprintf("Simulated preview (Connected to OpenSearch: %s)", osCfg.Host)
	}

	tableRows := []map[string]any{
		{"timestamp": time.Now().Add(-15 * time.Minute).Format("15:04:05"), "level": "INFO", "message": "Pipeline worker ingested 4,120 events successfully", "source": "dataprepper"},
		{"timestamp": time.Now().Add(-30 * time.Minute).Format("15:04:05"), "level": "WARN", "message": "High heap utilization threshold reached (78%)", "source": "opensearch-node-1"},
		{"timestamp": time.Now().Add(-45 * time.Minute).Format("15:04:05"), "level": "INFO", "message": "Cluster state green, all shards active", "source": "cluster-health"},
		{"timestamp": time.Now().Add(-60 * time.Minute).Format("15:04:05"), "level": "ERROR", "message": "Connection timeout during telemetry export to external gateway", "source": "otelcol"},
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

// fetchGrafanaLive attempts to execute query against Grafana API
func (s *ReportService) fetchGrafanaLive(ctx context.Context, cfg *domain.GrafanaConfig, req domain.ReportQueryDataRequest) ([]domain.ReportDataPoint, domain.ReportWidgetSummary, error) {
	endpoint := strings.TrimRight(cfg.Host, "/") + "/api/ds/query"

	queryPayload := map[string]interface{}{
		"queries": []map[string]interface{}{
			{
				"datasource": map[string]string{
					"uid": cfg.DatasourceUID,
				},
				"expr": req.MetricKey,
			},
		},
		"from": s.parseTimeRangeStart(req.TimeRange),
		"to":   "now",
	}

	bodyBytes, _ := json.Marshal(queryPayload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, domain.ReportWidgetSummary{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.Token))
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.ReportWidgetSummary{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, domain.ReportWidgetSummary{}, fmt.Errorf("grafana status %d", resp.StatusCode)
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
		return nil, domain.ReportWidgetSummary{}, err
	}

	var points []domain.ReportDataPoint
	for _, res := range parsed.Results {
		for _, frame := range res.Frames {
			if len(frame.Data.Values) >= 2 {
				timeVals := frame.Data.Values[0]
				numVals := frame.Data.Values[1]

				for i := 0; i < len(timeVals) && i < len(numVals); i++ {
					tFloat, _ := timeVals[i].(float64)
					vFloat, _ := numVals[i].(float64)

					tSec := int64(tFloat / 1000)
					if tFloat < 1e11 {
						tSec = int64(tFloat)
					}
					tm := time.Unix(tSec, 0)

					points = append(points, domain.ReportDataPoint{
						Timestamp: tm.Format("2006-01-02 15:04"),
						Label:     tm.Format("15:04"),
						Value:     math.Round(vFloat*100) / 100,
					})
				}
			}
		}
	}

	if len(points) == 0 {
		return nil, domain.ReportWidgetSummary{}, fmt.Errorf("no time-series points returned from Grafana")
	}

	summary := s.calculateSummary(points, "%")
	return points, summary, nil
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

	aggsPayload := map[string]interface{}{
		"size": 5,
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": s.parseTimeRangeStart(req.TimeRange),
					"lte": "now",
				},
			},
		},
		"aggs": map[string]interface{}{
			"events_over_time": map[string]interface{}{
				"date_histogram": map[string]interface{}{
					"field":          "@timestamp",
					"fixed_interval": interval,
				},
			},
		},
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
			if k == "@timestamp" || k == "message" || k == "level" || k == "status" || k == "host" {
				row[k] = v
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

func (s *ReportService) generateTimeSeriesData(title, timeRange, source string) ([]domain.ReportDataPoint, domain.ReportWidgetSummary) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

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
	baseVal := 45.0
	variance := 15.0

	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "cpu") {
		baseVal = 48.0
		variance = 20.0
		unit = "%"
	} else if strings.Contains(titleLower, "memory") || strings.Contains(titleLower, "ram") {
		baseVal = 62.0
		variance = 10.0
		unit = "%"
	} else if strings.Contains(titleLower, "storage") || strings.Contains(titleLower, "disk") {
		baseVal = 55.0
		variance = 4.0
		unit = "%"
	} else if strings.Contains(titleLower, "log") || strings.Contains(titleLower, "event") || source == "opensearch" {
		baseVal = 1850.0
		variance = 600.0
		unit = "events/min"
	} else if strings.Contains(titleLower, "network") || strings.Contains(titleLower, "traffic") {
		baseVal = 125.0
		variance = 45.0
		unit = "Mbps"
	}

	startTime := time.Now().Add(-time.Duration(numPoints) * step)
	points := make([]domain.ReportDataPoint, 0, numPoints)

	curVal := baseVal
	for i := 0; i < numPoints; i++ {
		t := startTime.Add(time.Duration(i+1) * step)
		delta := (r.Float64() - 0.48) * variance
		curVal += delta
		if curVal < 2 {
			curVal = 5
		}
		if unit == "%" && curVal > 98 {
			curVal = 92
		}

		rounded := math.Round(curVal*10) / 10
		points = append(points, domain.ReportDataPoint{
			Timestamp: t.Format("2006-01-02 15:04"),
			Label:     t.Format(dateFormat),
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
	sum := 0.0

	for _, p := range points {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
		sum += p.Value
	}

	avgVal := sum / float64(len(points))
	curVal := points[len(points)-1].Value

	return domain.ReportWidgetSummary{
		Min:     math.Round(minVal*100) / 100,
		Max:     math.Round(maxVal*100) / 100,
		Avg:     math.Round(avgVal*100) / 100,
		Current: math.Round(curVal*100) / 100,
		Total:   math.Round(sum*100) / 100,
		Count:   len(points),
		Unit:    unit,
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
