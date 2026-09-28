package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"
)

type MonitoringInstanceService struct {
	instRepo       *repository.MonitoringInstanceRepository
	remoteHostRepo *repository.RemoteHostRepository
	promService    *PrometheusService
	workerPool     *queue.WorkerPool

	// Metrics Cache & Polling Engine
	cacheMu         sync.RWMutex
	metricsCache    map[string]*domain.InstanceLiveMetrics
	lastPolledAt    time.Time
	isPolling       bool
	pollInterval    time.Duration
	stopChan        chan struct{}
	pollTriggerChan chan struct{}
}

func NewMonitoringInstanceService(
	instRepo *repository.MonitoringInstanceRepository,
	remoteHostRepo *repository.RemoteHostRepository,
	promService *PrometheusService,
	workerPool *queue.WorkerPool,
) *MonitoringInstanceService {
	return &MonitoringInstanceService{
		instRepo:        instRepo,
		remoteHostRepo:  remoteHostRepo,
		promService:     promService,
		workerPool:      workerPool,
		metricsCache:    make(map[string]*domain.InstanceLiveMetrics),
		pollInterval:    30 * time.Second, // default 30s as requested
		stopChan:        make(chan struct{}),
		pollTriggerChan: make(chan struct{}, 1),
	}
}

// StartBackgroundEngine starts the periodic polling queue engine to fetch metrics from Prometheus in background
func (s *MonitoringInstanceService) StartBackgroundEngine() {
	if s.workerPool != nil {
		s.workerPool.RegisterHandler("monitoring_instance_poll", func(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error {
			s.pollAllMetricsOnce(ctx)
			return nil
		})
	}

	go func() {
		// Wait 2 seconds on startup before initial poll
		time.Sleep(2 * time.Second)
		s.pollAllMetricsOnce(context.Background())

		s.cacheMu.RLock()
		interval := s.pollInterval
		s.cacheMu.RUnlock()

		if interval <= 0 {
			interval = 30 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-s.pollTriggerChan:
				s.pollAllMetricsOnce(context.Background())
			case <-ticker.C:
				s.cacheMu.RLock()
				curInterval := s.pollInterval
				s.cacheMu.RUnlock()

				if curInterval != interval && curInterval > 0 {
					interval = curInterval
					ticker.Reset(interval)
				}
				if interval > 0 {
					s.pollAllMetricsOnce(context.Background())
				}
			}
		}
	}()
	logger.Info("MonitoringEngine", "Background Prometheus metric polling engine started (Default: 30s)")
}

func (s *MonitoringInstanceService) Stop() {
	close(s.stopChan)
}

func (s *MonitoringInstanceService) TriggerImmediatePoll() {
	select {
	case s.pollTriggerChan <- struct{}{}:
	default:
	}
}

func (s *MonitoringInstanceService) SetPollInterval(d time.Duration) {
	s.cacheMu.Lock()
	s.pollInterval = d
	s.cacheMu.Unlock()
	logger.Info("MonitoringEngine", fmt.Sprintf("Auto-refresh polling interval updated to %v", d))
}

func (s *MonitoringInstanceService) GetEngineStatus() map[string]interface{} {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	return map[string]interface{}{
		"lastPolledAt":        s.lastPolledAt,
		"pollIntervalSeconds": int(s.pollInterval.Seconds()),
		"isPolling":           s.isPolling,
		"cachedInstances":     len(s.metricsCache),
	}
}

// pollAllMetricsOnce runs batch metrics retrieval with a strict timeout and updates the in-memory cache
func (s *MonitoringInstanceService) pollAllMetricsOnce(ctx context.Context) {
	s.cacheMu.Lock()
	if s.isPolling {
		s.cacheMu.Unlock()
		return
	}
	s.isPolling = true
	s.cacheMu.Unlock()

	defer func() {
		s.cacheMu.Lock()
		s.isPolling = false
		s.cacheMu.Unlock()
	}()

	// Query all instances across the system with a 12-second timeout context to prevent any hang
	pollCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	instances, err := s.instRepo.List(pollCtx, 0, "ADMIN", "", "")
	if err != nil || len(instances) == 0 {
		return
	}

	start := time.Now()
	s.BatchGetLiveMetrics(pollCtx, instances)

	s.cacheMu.Lock()
	for _, inst := range instances {
		if inst.LiveMetrics != nil {
			s.metricsCache[inst.ID] = inst.LiveMetrics
		}
	}
	s.lastPolledAt = time.Now()
	s.cacheMu.Unlock()

	logger.Info("MonitoringEngine", fmt.Sprintf("Polled metrics for %d instances in %v", len(instances), time.Since(start)))
}

// Prometheus API response structures
type promVectorResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [unix_timestamp, "value_string"]
		} `json:"result"`
	} `json:"data"`
}

type promMatrixResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"` // [[unix_timestamp, "value_string"], ...]
		} `json:"result"`
	} `json:"data"`
}

func (s *MonitoringInstanceService) ListInstances(
	ctx context.Context,
	userID int,
	userRole string,
	groupFilter string,
	tagFilter string,
	fetchMetrics bool,
) ([]*domain.MonitoringInstance, error) {
	instances, err := s.instRepo.List(ctx, userID, userRole, groupFilter, tagFilter)
	if err != nil {
		return nil, err
	}

	if fetchMetrics && len(instances) > 0 {
		s.cacheMu.RLock()
		cacheLen := len(s.metricsCache)
		for _, inst := range instances {
			if cached, exists := s.metricsCache[inst.ID]; exists {
				inst.LiveMetrics = cached
			} else {
				// Clean N/A placeholder
				inst.LiveMetrics = &domain.InstanceLiveMetrics{
					IsOnline:      false,
					AgentVersion:  "N/A",
					HasOTel:       false,
					CPUPct:        nil,
					CPUCount:      0,
					MemPct:        nil,
					MemUsedBytes:  0,
					MemFreeBytes:  0,
					MemTotalBytes: 0,
					DiskPct:       nil,
					Disks:         []domain.InstanceDiskMetric{},
					NetDownloadMB: 0,
					NetUploadMB:   0,
					NetTotalMB:    0,
					LastUpdated:   s.lastPolledAt,
				}
			}
		}
		s.cacheMu.RUnlock()

		// Trigger background poll if cache was empty
		if cacheLen == 0 {
			s.TriggerImmediatePoll()
		}
	}

	return instances, nil
}

func (s *MonitoringInstanceService) GetInstance(
	ctx context.Context,
	id string,
	userID int,
	userRole string,
	fetchMetrics bool,
) (*domain.MonitoringInstance, error) {
	inst, err := s.instRepo.GetByID(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}

	if fetchMetrics {
		s.cacheMu.RLock()
		if cached, exists := s.metricsCache[inst.ID]; exists {
			inst.LiveMetrics = cached
		} else {
			inst.LiveMetrics = &domain.InstanceLiveMetrics{
				IsOnline:      false,
				AgentVersion:  "N/A",
				HasOTel:       false,
				CPUPct:        nil,
				CPUCount:      0,
				MemPct:        nil,
				MemUsedBytes:  0,
				MemFreeBytes:  0,
				MemTotalBytes: 0,
				DiskPct:       nil,
				Disks:         []domain.InstanceDiskMetric{},
				NetDownloadMB: 0,
				NetUploadMB:   0,
				NetTotalMB:    0,
				LastUpdated:   s.lastPolledAt,
			}
		}
		s.cacheMu.RUnlock()
	}

	return inst, nil
}

func (s *MonitoringInstanceService) CreateInstance(
	ctx context.Context,
	req *domain.CreateMonitoringInstanceRequest,
	userID int,
) (*domain.MonitoringInstance, error) {
	inst := &domain.MonitoringInstance{
		Name:             req.Name,
		Host:             req.Host,
		IPAddress:        req.IPAddress,
		Port:             req.Port,
		InstanceType:     req.InstanceType,
		GroupName:        req.GroupName,
		Tags:             req.Tags,
		PrometheusTarget: req.PrometheusTarget,
		RemoteHostID:     req.RemoteHostID,
		UserID:           &userID,
		Visibility:       req.Visibility,
		AlertEnabled:     req.AlertEnabled,
		Notes:            req.Notes,
	}

	if inst.IPAddress == "" {
		inst.IPAddress = inst.Host
	}
	if inst.Port == 0 {
		inst.Port = 8889
	}
	if inst.PrometheusTarget == "" {
		inst.PrometheusTarget = fmt.Sprintf("%s:%d", inst.IPAddress, inst.Port)
	}

	if err := s.instRepo.Create(ctx, inst); err != nil {
		return nil, err
	}

	return inst, nil
}

func (s *MonitoringInstanceService) UpdateInstance(
	ctx context.Context,
	id string,
	req *domain.UpdateMonitoringInstanceRequest,
	userID int,
	userRole string,
) (*domain.MonitoringInstance, error) {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return nil, fmt.Errorf("permission denied: manage access required")
	}

	inst, err := s.instRepo.GetByID(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}

	inst.Name = req.Name
	inst.Host = req.Host
	inst.IPAddress = req.IPAddress
	inst.Port = req.Port
	inst.InstanceType = req.InstanceType
	inst.GroupName = req.GroupName
	inst.Tags = req.Tags
	inst.PrometheusTarget = req.PrometheusTarget
	inst.RemoteHostID = req.RemoteHostID
	inst.Visibility = req.Visibility
	inst.AlertEnabled = req.AlertEnabled
	inst.Notes = req.Notes

	if inst.IPAddress == "" {
		inst.IPAddress = inst.Host
	}
	if inst.Port == 0 {
		inst.Port = 8889
	}

	if err := s.instRepo.Update(ctx, inst); err != nil {
		return nil, err
	}

	return inst, nil
}

func (s *MonitoringInstanceService) DeleteInstance(
	ctx context.Context,
	id string,
	userID int,
	userRole string,
) error {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, userID, userRole)
	if err != nil {
		return err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return fmt.Errorf("permission denied: manage access required to delete instance")
	}

	return s.instRepo.Delete(ctx, id)
}

func (s *MonitoringInstanceService) ToggleAlert(
	ctx context.Context,
	id string,
	alertEnabled bool,
	userID int,
	userRole string,
) error {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, userID, userRole)
	if err != nil {
		return err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return fmt.Errorf("permission denied")
	}

	return s.instRepo.ToggleAlert(ctx, id, alertEnabled)
}

func (s *MonitoringInstanceService) SyncFromRemoteHosts(
	ctx context.Context,
	req *domain.SyncRemoteHostsRequest,
	userID int,
	userRole string,
) ([]*domain.MonitoringInstance, error) {
	remoteHosts, err := s.remoteHostRepo.List(ctx, userID, userRole)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve remote hosts: %w", err)
	}

	filterMap := make(map[string]bool)
	for _, id := range req.HostIDs {
		filterMap[id] = true
	}

	var synced []*domain.MonitoringInstance
	for _, rh := range remoteHosts {
		if len(filterMap) > 0 && !filterMap[rh.ID] {
			continue
		}
		inst, err := s.instRepo.UpsertFromRemoteHost(ctx, &rh, userID)
		if err != nil {
			continue
		}
		synced = append(synced, inst)
	}

	return synced, nil
}

func (s *MonitoringInstanceService) ListShares(
	ctx context.Context,
	id string,
	userID int,
	userRole string,
) ([]domain.MonitoringInstanceShare, error) {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return nil, fmt.Errorf("permission denied")
	}

	return s.instRepo.ListShares(ctx, id)
}

func (s *MonitoringInstanceService) AddShare(
	ctx context.Context,
	id string,
	req *domain.ShareMonitoringInstanceRequest,
	currentUserID int,
	userRole string,
) error {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, currentUserID, userRole)
	if err != nil {
		return err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return fmt.Errorf("permission denied: only owner or manager can share instance")
	}

	share := &domain.MonitoringInstanceShare{
		InstanceID: id,
		UserID:     req.UserID,
		Permission: req.Permission,
		SharedBy:   &currentUserID,
	}

	return s.instRepo.AddShare(ctx, share)
}

func (s *MonitoringInstanceService) DeleteShare(
	ctx context.Context,
	id string,
	targetUserID int,
	currentUserID int,
	userRole string,
) error {
	hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, currentUserID, userRole)
	if err != nil {
		return err
	}
	if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
		return fmt.Errorf("permission denied: only owner or manager can revoke shares")
	}

	return s.instRepo.DeleteShare(ctx, id, targetUserID)
}

func (s *MonitoringInstanceService) GetDistinctGroups(ctx context.Context, userID int, userRole string) ([]string, error) {
	return s.instRepo.GetDistinctGroups(ctx, userID, userRole)
}

// ============================================================================
// METRICS ENGINE: OpenTelemetry & Prometheus Collection
// ============================================================================

func (s *MonitoringInstanceService) executeInstantQuery(ctx context.Context, promQL string) (*promVectorResponse, error) {
	raw, err := s.promService.QueryPromQLRaw(ctx, promQL)
	if err != nil {
		return nil, err
	}

	var res promVectorResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *MonitoringInstanceService) executeRangeQuery(ctx context.Context, promQL string, start, end time.Time, step string) (*promMatrixResponse, error) {
	raw, err := s.promService.QueryRangePromQL(ctx, promQL, start, end, step)
	if err != nil {
		return nil, err
	}

	var res promMatrixResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func parseFloat(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func formatBytes(bytes float64) string {
	const unit = 1024.0
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp < len(units) {
		return fmt.Sprintf("%.1f %s", bytes/div, units[exp])
	}
	return fmt.Sprintf("%.1f GB", bytes/(1024*1024*1024))
}

func getInstancePattern(inst *domain.MonitoringInstance) string {
	if inst.PrometheusTarget != "" {
		return strings.TrimSpace(inst.PrometheusTarget)
	}
	targetHost := inst.IPAddress
	if targetHost == "" {
		targetHost = inst.Host
	}
	return fmt.Sprintf("%s(:%d)?", targetHost, inst.Port)
}

func (s *MonitoringInstanceService) BatchGetLiveMetrics(ctx context.Context, instances []*domain.MonitoringInstance) {
	if len(instances) == 0 {
		return
	}

	// 1. Query Up status
	upMap := make(map[string]bool)
	upRes, err := s.executeInstantQuery(ctx, "up")
	if err == nil && upRes.Status == "success" {
		for _, item := range upRes.Data.Result {
			target := item.Metric["instance"]
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && v > 0 {
					upMap[target] = true
				}
			}
		}
	}

	// 2. Query CPU %: OpenTelemetry hostmetrics receiver
	// (1 - sum by (instance) (rate(system_cpu_time_seconds_total{state="idle"}[2m])) / sum by (instance) (rate(system_cpu_time_seconds_total[2m]))) * 100
	cpuPctMap := make(map[string]float64)
	cpuRes, err := s.executeInstantQuery(ctx, `(1 - (sum by (instance) (rate(system_cpu_time_seconds_total{state="idle"}[2m])) / sum by (instance) (rate(system_cpu_time_seconds_total[2m])))) * 100`)
	if err == nil && cpuRes.Status == "success" {
		for _, item := range cpuRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					cpuPctMap[item.Metric["instance"]] = math.Round(math.Max(0, math.Min(100, v))*10) / 10
				}
			}
		}
	}

	// 3. Query CPU Cores Count: count by (instance) (system_cpu_time_seconds_total{state="idle"})
	cpuCountMap := make(map[string]int)
	coreRes, err := s.executeInstantQuery(ctx, `count by (instance) (system_cpu_time_seconds_total{state="idle"})`)
	if err == nil && coreRes.Status == "success" {
		for _, item := range coreRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					cpuCountMap[item.Metric["instance"]] = int(v)
				}
			}
		}
	}

	// 4. Query Memory: system_memory_usage_bytes
	memUsedMap := make(map[string]float64)
	memTotalMap := make(map[string]float64)
	memFreeMap := make(map[string]float64)

	memUsedRes, _ := s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes{state="used"})`)
	if memUsedRes != nil && memUsedRes.Status == "success" {
		for _, item := range memUsedRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					memUsedMap[item.Metric["instance"]] = v
				}
			}
		}
	}

	memTotalRes, _ := s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes)`)
	if memTotalRes != nil && memTotalRes.Status == "success" {
		for _, item := range memTotalRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					memTotalMap[item.Metric["instance"]] = v
				}
			}
		}
	}

	memFreeRes, _ := s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes{state="free"})`)
	if memFreeRes != nil && memFreeRes.Status == "success" {
		for _, item := range memFreeRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					memFreeMap[item.Metric["instance"]] = v
				}
			}
		}
	}

	// 5. Query Filesystem: system_filesystem_usage_bytes
	type diskRecord struct {
		instance   string
		mountpoint string
		device     string
		fsType     string
		used       float64
		free       float64
	}
	diskRecords := make(map[string]*diskRecord)

	fsUsedRes, _ := s.executeInstantQuery(ctx, `system_filesystem_usage_bytes{state="used"}`)
	if fsUsedRes != nil && fsUsedRes.Status == "success" {
		for _, item := range fsUsedRes.Data.Result {
			instTarget := item.Metric["instance"]
			mp := item.Metric["mountpoint"]
			dev := item.Metric["device"]
			key := fmt.Sprintf("%s|%s", instTarget, mp)
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					diskRecords[key] = &diskRecord{
						instance:   instTarget,
						mountpoint: mp,
						device:     dev,
						fsType:     item.Metric["type"],
						used:       v,
					}
				}
			}
		}
	}

	fsFreeRes, _ := s.executeInstantQuery(ctx, `system_filesystem_usage_bytes{state="free"}`)
	if fsFreeRes != nil && fsFreeRes.Status == "success" {
		for _, item := range fsFreeRes.Data.Result {
			instTarget := item.Metric["instance"]
			mp := item.Metric["mountpoint"]
			key := fmt.Sprintf("%s|%s", instTarget, mp)
			if rec, exists := diskRecords[key]; exists && len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					rec.free = v
				}
			}
		}
	}

	// 6. Query Network (MB/s): rate(system_network_io_bytes_total[2m]) / 1048576
	netDownMap := make(map[string]float64)
	netUpMap := make(map[string]float64)

	netDownRes, _ := s.executeInstantQuery(ctx, `sum by (instance) (rate(system_network_io_bytes_total{direction="receive"}[2m])) / 1048576`)
	if netDownRes != nil && netDownRes.Status == "success" {
		for _, item := range netDownRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					netDownMap[item.Metric["instance"]] = math.Round(math.Max(0, v)*100) / 100
				}
			}
		}
	}

	netUpRes, _ := s.executeInstantQuery(ctx, `sum by (instance) (rate(system_network_io_bytes_total{direction="transmit"}[2m])) / 1048576`)
	if netUpRes != nil && netUpRes.Status == "success" {
		for _, item := range netUpRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					netUpMap[item.Metric["instance"]] = math.Round(math.Max(0, v)*100) / 100
				}
			}
		}
	}

	// Helper function to find best match among map keys
	matchTarget := func(inst *domain.MonitoringInstance, mKeys []string) string {
		targetHost := inst.IPAddress
		if targetHost == "" {
			targetHost = inst.Host
		}
		expectedTarget := fmt.Sprintf("%s:%d", targetHost, inst.Port)
		if inst.PrometheusTarget != "" {
			expectedTarget = inst.PrometheusTarget
		}

		for _, k := range mKeys {
			if strings.EqualFold(k, expectedTarget) {
				return k
			}
		}
		for _, k := range mKeys {
			if strings.HasPrefix(k, targetHost) || strings.HasPrefix(k, inst.Host) {
				return k
			}
		}
		return ""
	}

	// Map keys collection
	var allMetricInstances []string
	seenInst := make(map[string]bool)
	for k := range upMap {
		if !seenInst[k] {
			seenInst[k] = true
			allMetricInstances = append(allMetricInstances, k)
		}
	}
	for k := range cpuPctMap {
		if !seenInst[k] {
			seenInst[k] = true
			allMetricInstances = append(allMetricInstances, k)
		}
	}
	for k := range memTotalMap {
		if !seenInst[k] {
			seenInst[k] = true
			allMetricInstances = append(allMetricInstances, k)
		}
	}

	now := time.Now()

	for _, inst := range instances {
		matched := matchTarget(inst, allMetricInstances)

		// If no matching metrics found at all in OpenTelemetry data:
		if matched == "" {
			inst.LiveMetrics = &domain.InstanceLiveMetrics{
				IsOnline:      false,
				AgentVersion:  "N/A",
				HasOTel:       false,
				CPUPct:        nil,
				CPUCount:      0,
				MemPct:        nil,
				MemUsedBytes:  0,
				MemFreeBytes:  0,
				MemTotalBytes: 0,
				DiskPct:       nil,
				Disks:         []domain.InstanceDiskMetric{},
				NetDownloadMB: 0,
				NetUploadMB:   0,
				NetTotalMB:    0,
				LastUpdated:   now,
			}
			continue
		}

		// Host HAS OpenTelemetry metrics reporting!
		isOnline := upMap[matched]
		hasOtel := true
		agentVersion := "0.11.1" // Standard reported OpenTelemetry Collector agent version

		// CPU
		var cpuPct *float64
		if val, exists := cpuPctMap[matched]; exists {
			cpuPct = &val
		}
		cpuCount := cpuCountMap[matched]
		if cpuCount == 0 && cpuPct != nil {
			cpuCount = 1
		}

		// Memory
		var memPct *float64
		memUsed := memUsedMap[matched]
		memTotal := memTotalMap[matched]
		memFree := memFreeMap[matched]
		if memTotal > 0 {
			calcPct := math.Round((memUsed/memTotal)*1000) / 10
			memPct = &calcPct
		}

		// Disks
		var disks []domain.InstanceDiskMetric
		var primaryDiskPct *float64
		var totalDiskUsed float64
		var totalDiskCapacity float64

		for _, rec := range diskRecords {
			if rec.instance == matched {
				total := rec.used + rec.free
				if total <= 0 {
					continue
				}
				pct := math.Round((rec.used/total)*1000) / 10
				diskItem := domain.InstanceDiskMetric{
					Mountpoint: rec.mountpoint,
					Device:     rec.device,
					FSType:     rec.fsType,
					UsagePct:   pct,
					UsedBytes:  rec.used,
					TotalBytes: total,
					FreeBytes:  rec.free,
					UsageHuman: fmt.Sprintf("%s / %s", formatBytes(rec.used), formatBytes(total)),
				}
				disks = append(disks, diskItem)
				totalDiskUsed += rec.used
				totalDiskCapacity += total

				// Root mountpoint preference for top card metric
				if rec.mountpoint == "/" || rec.mountpoint == "C:" || rec.mountpoint == "C:\\" {
					rootPct := pct
					primaryDiskPct = &rootPct
				}
			}
		}

		if primaryDiskPct == nil && totalDiskCapacity > 0 {
			overallPct := math.Round((totalDiskUsed/totalDiskCapacity)*1000) / 10
			primaryDiskPct = &overallPct
		}

		// Network
		netDown := netDownMap[matched]
		netUp := netUpMap[matched]
		netTotal := math.Round((netDown+netUp)*100) / 100

		inst.LiveMetrics = &domain.InstanceLiveMetrics{
			IsOnline:      isOnline,
			AgentVersion:  agentVersion,
			HasOTel:       hasOtel,
			CPUPct:        cpuPct,
			CPUCount:      cpuCount,
			MemPct:        memPct,
			MemUsedBytes:  memUsed,
			MemFreeBytes:  memFree,
			MemTotalBytes: memTotal,
			DiskPct:       primaryDiskPct,
			Disks:         disks,
			NetDownloadMB: netDown,
			NetUploadMB:   netUp,
			NetTotalMB:    netTotal,
			LastUpdated:   now,
		}
	}
}

func (s *MonitoringInstanceService) GetLiveMetrics(ctx context.Context, inst *domain.MonitoringInstance) (*domain.InstanceLiveMetrics, error) {
	instances := []*domain.MonitoringInstance{inst}
	s.BatchGetLiveMetrics(ctx, instances)
	return inst.LiveMetrics, nil
}

// GetInstanceHistory retrieves historical series data for interactive line charts (View History)
func (s *MonitoringInstanceService) GetInstanceHistory(
	ctx context.Context,
	id string,
	timeRange string,
	userID int,
	userRole string,
) (*domain.InstanceHistoryResponse, error) {
	inst, err := s.instRepo.GetByID(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}

	targetPattern := getInstancePattern(inst)

	now := time.Now()
	var startTime time.Time
	var step string

	switch timeRange {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
		step = "30s"
	case "6h":
		startTime = now.Add(-6 * time.Hour)
		step = "2m"
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
		step = "30m"
	case "24h":
		fallthrough
	default:
		timeRange = "24h"
		startTime = now.Add(-24 * time.Hour)
		step = "5m"
	}

	resp := &domain.InstanceHistoryResponse{
		InstanceID: inst.ID,
		TimeRange:  timeRange,
		CPU:        []domain.MetricHistoryPoint{},
		Memory:     []domain.MetricHistoryPoint{},
		Disk:       []domain.MetricHistoryPoint{},
		NetIn:      []domain.MetricHistoryPoint{},
		NetOut:     []domain.MetricHistoryPoint{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	parsePoints := func(res *promMatrixResponse) []domain.MetricHistoryPoint {
		var points []domain.MetricHistoryPoint
		if res == nil || res.Status != "success" || len(res.Data.Result) == 0 {
			return points
		}
		// Pick first series
		for _, rawPoint := range res.Data.Result[0].Values {
			if len(rawPoint) > 1 {
				var ts int64
				switch t := rawPoint[0].(type) {
				case float64:
					ts = int64(t)
				case int64:
					ts = t
				}
				if val, ok := parseFloat(rawPoint[1]); ok && !math.IsNaN(val) && !math.IsInf(val, 0) {
					points = append(points, domain.MetricHistoryPoint{
						Timestamp: ts,
						Value:     math.Round(val*100) / 100,
					})
				}
			}
		}
		return points
	}

	// 1. CPU History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`(1 - (sum(rate(system_cpu_time_seconds_total{instance=~"%s",state="idle"}[2m])) / sum(rate(system_cpu_time_seconds_total{instance=~"%s"}[2m])))) * 100`, targetPattern, targetPattern)
		r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
		if err == nil {
			pts := parsePoints(r)
			mu.Lock()
			resp.CPU = pts
			mu.Unlock()
		}
	}()

	// 2. Memory History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`(sum(system_memory_usage_bytes{instance=~"%s",state="used"}) / sum(system_memory_usage_bytes{instance=~"%s"})) * 100`, targetPattern, targetPattern)
		r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
		if err == nil {
			pts := parsePoints(r)
			mu.Lock()
			resp.Memory = pts
			mu.Unlock()
		}
	}()

	// 3. Disk History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`(sum(system_filesystem_usage_bytes{instance=~"%s",state="used"}) / (sum(system_filesystem_usage_bytes{instance=~"%s",state="used"}) + sum(system_filesystem_usage_bytes{instance=~"%s",state="free"}))) * 100`, targetPattern, targetPattern, targetPattern)
		r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
		if err == nil {
			pts := parsePoints(r)
			mu.Lock()
			resp.Disk = pts
			mu.Unlock()
		}
	}()

	// 4. Net In (Download MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`sum(rate(system_network_io_bytes_total{instance=~"%s",direction="receive"}[2m])) / 1048576`, targetPattern)
		r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
		if err == nil {
			pts := parsePoints(r)
			mu.Lock()
			resp.NetIn = pts
			mu.Unlock()
		}
	}()

	// 5. Net Out (Upload MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`sum(rate(system_network_io_bytes_total{instance=~"%s",direction="transmit"}[2m])) / 1048576`, targetPattern)
		r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
		if err == nil {
			pts := parsePoints(r)
			mu.Lock()
			resp.NetOut = pts
			mu.Unlock()
		}
	}()

	wg.Wait()

	return resp, nil
}
