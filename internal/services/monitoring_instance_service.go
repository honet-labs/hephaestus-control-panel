package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type MonitoringInstanceService struct {
	instRepo       *repository.MonitoringInstanceRepository
	remoteHostRepo *repository.RemoteHostRepository
	promService    *PrometheusService
	workerPool     *queue.WorkerPool

	vpsService     *VpsService
	pollCycleCount int

	// Metrics Cache & Polling Engine
	cacheMu         sync.RWMutex
	metricsCache    map[string]*domain.InstanceLiveMetrics
	lastPolledAt    time.Time
	isPolling       bool
	pollInterval    time.Duration
	stopChan        chan struct{}
	pollTriggerChan chan struct{}

	// Docker Containers Cache
	dockerCacheMu    sync.RWMutex
	dockerCache      []*domain.DockerContainerMetric
	dockerLastPolled time.Time
}

func NewMonitoringInstanceService(
	instRepo *repository.MonitoringInstanceRepository,
	remoteHostRepo *repository.RemoteHostRepository,
	promService *PrometheusService,
	workerPool *queue.WorkerPool,
	vpsService *VpsService,
) *MonitoringInstanceService {
	return &MonitoringInstanceService{
		instRepo:        instRepo,
		remoteHostRepo:  remoteHostRepo,
		promService:     promService,
		workerPool:      workerPool,
		vpsService:      vpsService,
		metricsCache:    make(map[string]*domain.InstanceLiveMetrics),
		dockerCache:     make([]*domain.DockerContainerMetric, 0),
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
	lastPolledAt := s.lastPolledAt
	intervalSec := int(s.pollInterval.Seconds())
	isPolling := s.isPolling
	cachedInstances := len(s.metricsCache)
	s.cacheMu.RUnlock()

	s.dockerCacheMu.RLock()
	cachedContainers := len(s.dockerCache)
	dockerLastPolled := s.dockerLastPolled
	s.dockerCacheMu.RUnlock()

	return map[string]interface{}{
		"lastPolledAt":        lastPolledAt,
		"pollIntervalSeconds": intervalSec,
		"isPolling":           isPolling,
		"cachedInstances":     cachedInstances,
		"cachedContainers":    cachedContainers,
		"dockerLastPolledAt":  dockerLastPolled,
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

	// Query with 15-second timeout context to prevent any hang
	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// 1. Poll Docker Containers concurrently
	go func() {
		defer wg.Done()
		s.pollDockerContainersOnce(pollCtx)
	}()

	// 2. Poll Server Instances concurrently
	go func() {
		defer wg.Done()
		instances, err := s.instRepo.List(pollCtx, 0, "ADMIN", "", "")
		if err != nil || len(instances) == 0 {
			return
		}

		start := time.Now()
		s.BatchGetLiveMetrics(pollCtx, instances)

		// Poll SSH metrics for instances linked to Remote Hosts that need SSH telemetry
		s.pollSSHInstances(pollCtx, instances)

		s.cacheMu.Lock()
		for _, inst := range instances {
			if inst.LiveMetrics != nil {
				s.metricsCache[inst.ID] = inst.LiveMetrics

				// Persist live metrics snapshot to DB for quick cold restarts
				_ = s.instRepo.SaveLiveMetrics(pollCtx, inst.ID, inst.LiveMetrics)

				// Save history point into DB
				var cpuVal, memVal, diskVal, netVal float64
				if inst.LiveMetrics.CPUPct != nil {
					cpuVal = *inst.LiveMetrics.CPUPct
				}
				if inst.LiveMetrics.MemPct != nil {
					memVal = *inst.LiveMetrics.MemPct
				}
				if inst.LiveMetrics.DiskPct != nil {
					diskVal = *inst.LiveMetrics.DiskPct
				}
				netVal = inst.LiveMetrics.NetTotalMB

				src := "prometheus"
				if inst.MetricSource == "ssh" || (inst.MetricSource != "prometheus" && inst.Port == 22 && inst.RemoteHostID != nil && *inst.RemoteHostID != "") {
					src = "ssh"
				}
				_ = s.instRepo.SaveMetricsHistory(pollCtx, inst.ID, cpuVal, memVal, diskVal, netVal, inst.LiveMetrics.NetDownloadMB, inst.LiveMetrics.NetUploadMB, src)
			}
		}
		s.lastPolledAt = time.Now()
		s.pollCycleCount++
		// Run DB pruning once every 120 polling cycles (~1 hour if 30s interval)
		if s.pollCycleCount%120 == 0 {
			go func() {
				pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer pruneCancel()
				_ = s.instRepo.PruneMetricsHistory(pruneCtx, 7)
			}()
		}
		s.cacheMu.Unlock()

		logger.Info("MonitoringEngine", fmt.Sprintf("Polled metrics for %d instances in %v", len(instances), time.Since(start)))
	}()

	wg.Wait()
}

func (s *MonitoringInstanceService) pollDockerContainersOnce(ctx context.Context) {
	start := time.Now()

	type qJob struct {
		key string
		q   string
	}

	jobs := []qJob{
		{"cpu_util", "container_cpu_utilization_ratio"},
		{"cpu_ns", "container_cpu_usage_nanoseconds_total"},
		{"cpu_kernel_ns", "container_cpu_usage_kernelmode_nanoseconds_total"},
		{"cpu_user_ns", "container_cpu_usage_usermode_nanoseconds_total"},
		{"mem_pct", "container_memory_percent_ratio"},
		{"mem_usage", "container_memory_usage_total_bytes"},
		{"mem_limit", "container_memory_usage_limit_bytes"},
		{"mem_file", "container_memory_file_bytes"},
		{"net_rx", "container_network_io_usage_rx_bytes_total"},
		{"net_tx", "container_network_io_usage_tx_bytes_total"},
		{"net_rx_rate", "rate(container_network_io_usage_rx_bytes_total[2m])"},
		{"net_tx_rate", "rate(container_network_io_usage_tx_bytes_total[2m])"},
		{"net_rx_dropped", "container_network_io_usage_rx_dropped_total"},
		{"net_tx_dropped", "container_network_io_usage_tx_dropped_total"},
		{"block_io", "container_blockio_io_service_bytes_recursive_total"},
		{"block_io_rate", "rate(container_blockio_io_service_bytes_recursive_total[2m])"},
	}

	results := make(map[string]*promVectorResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j qJob) {
			defer wg.Done()
			res, err := s.executeInstantQuery(ctx, j.q)
			if err == nil && res != nil && res.Status == "success" {
				mu.Lock()
				results[j.key] = res
				mu.Unlock()
			}
		}(job)
	}

	wg.Wait()

	containersMap := make(map[string]*domain.DockerContainerMetric)

	getOrCreate := func(labels map[string]string) *domain.DockerContainerMetric {
		cid := strings.TrimSpace(labels["container_id"])
		cname := strings.TrimSpace(labels["container_name"])
		host := strings.TrimSpace(labels["hostname"])
		ip := strings.TrimSpace(labels["ip_address"])

		key := cid
		if key == "" {
			key = host + ":" + cname
		}
		if key == ":" || key == "" {
			return nil
		}

		c, exists := containersMap[key]
		if !exists {
			c = &domain.DockerContainerMetric{
				ID:                key,
				ContainerID:       cid,
				ContainerName:     cname,
				ContainerHostname: labels["container_hostname"],
				ImageName:         labels["container_image_name"],
				Runtime:           labels["container_runtime"],
				Hostname:          host,
				IPAddress:         ip,
				Environment:       labels["environment"],
				IsOnline:          true,
				LastUpdated:       time.Now(),
			}
			if c.Runtime == "" {
				c.Runtime = "docker"
			}
			containersMap[key] = c
		} else {
			if c.ContainerName == "" && cname != "" {
				c.ContainerName = cname
			}
			if c.ImageName == "" && labels["container_image_name"] != "" {
				c.ImageName = labels["container_image_name"]
			}
			if c.Hostname == "" && host != "" {
				c.Hostname = host
			}
			if c.IPAddress == "" && ip != "" {
				c.IPAddress = ip
			}
			if c.ContainerHostname == "" && labels["container_hostname"] != "" {
				c.ContainerHostname = labels["container_hostname"]
			}
		}
		return c
	}

	// 1. Process CPU Util
	if res, ok := results["cpu_util"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					pct := math.Round(math.Max(0, v)*100) / 100
					c.CPUPct = &pct
				}
			}
		}
	}

	// 2. Process Mem Pct
	if res, ok := results["mem_pct"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					pct := math.Round(math.Max(0, math.Min(100, v))*100) / 100
					c.MemPct = &pct
				}
			}
		}
	}

	// 3. Process Mem Usage
	if res, ok := results["mem_usage"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.MemUsageBytes = v
				}
			}
		}
	}

	// 4. Process Mem Limit
	if res, ok := results["mem_limit"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.MemLimitBytes = v
				}
			}
		}
	}

	// 5. Process Mem File (Cache)
	if res, ok := results["mem_file"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.MemCacheBytes = v
				}
			}
		}
	}

	// 6. Process Net Rx & Tx Total
	if res, ok := results["net_rx"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.NetRxBytes += v
				}
			}
		}
	}
	if res, ok := results["net_tx"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.NetTxBytes += v
				}
			}
		}
	}

	// 7. Process Net Rx & Tx Rate
	if res, ok := results["net_rx_rate"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					c.NetRxRateMB += math.Round((v/(1024*1024))*100) / 100
				}
			}
		}
	}
	if res, ok := results["net_tx_rate"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					c.NetTxRateMB += math.Round((v/(1024*1024))*100) / 100
				}
			}
		}
	}

	// 8. Process Net Dropped
	if res, ok := results["net_rx_dropped"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.NetRxDropped += v
				}
			}
		}
	}
	if res, ok := results["net_tx_dropped"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.NetTxDropped += v
				}
			}
		}
	}

	// 9. Process Block I/O Total
	if res, ok := results["block_io"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			op := strings.ToLower(item.Metric["operation"])
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					if op == "read" {
						c.BlockReadBytes += v
					} else if op == "write" {
						c.BlockWriteBytes += v
					}
				}
			}
		}
	}

	// 10. Process Block I/O Rate
	if res, ok := results["block_io_rate"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			op := strings.ToLower(item.Metric["operation"])
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					mb := math.Round((v/(1024*1024))*100) / 100
					if op == "read" {
						c.BlockReadRateMB += mb
					} else if op == "write" {
						c.BlockWriteRateMB += mb
					}
				}
			}
		}
	}

	// 11. Process CPU nanoseconds
	if res, ok := results["cpu_ns"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.CPUTotalNs = v
				}
			}
		}
	}
	if res, ok := results["cpu_kernel_ns"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.CPUKernelNs = v
				}
			}
		}
	}
	if res, ok := results["cpu_user_ns"]; ok {
		for _, item := range res.Data.Result {
			c := getOrCreate(item.Metric)
			if c == nil {
				continue
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					c.CPUUserNs = v
				}
			}
		}
	}

	// Convert map to slice and sort
	var list []*domain.DockerContainerMetric
	for _, c := range containersMap {
		list = append(list, c)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Hostname != list[j].Hostname {
			return list[i].Hostname < list[j].Hostname
		}
		return strings.ToLower(list[i].ContainerName) < strings.ToLower(list[j].ContainerName)
	})

	s.dockerCacheMu.Lock()
	s.dockerCache = list
	s.dockerLastPolled = time.Now()
	s.dockerCacheMu.Unlock()

	logger.Info("MonitoringEngine", fmt.Sprintf("Polled metrics for %d docker containers in %v", len(list), time.Since(start)))
}

func (s *MonitoringInstanceService) ListDockerContainers(ctx context.Context, hostFilter string) ([]*domain.DockerContainerMetric, error) {
	s.dockerCacheMu.RLock()
	cacheLen := len(s.dockerCache)
	var result []*domain.DockerContainerMetric
	for _, c := range s.dockerCache {
		if hostFilter != "" && !strings.EqualFold(c.Hostname, hostFilter) && !strings.EqualFold(c.IPAddress, hostFilter) {
			continue
		}
		result = append(result, c)
	}
	s.dockerCacheMu.RUnlock()

	if cacheLen == 0 {
		pollCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		s.pollDockerContainersOnce(pollCtx)

		s.dockerCacheMu.RLock()
		result = nil
		for _, c := range s.dockerCache {
			if hostFilter != "" && !strings.EqualFold(c.Hostname, hostFilter) && !strings.EqualFold(c.IPAddress, hostFilter) {
				continue
			}
			result = append(result, c)
		}
		s.dockerCacheMu.RUnlock()
	}

	return result, nil
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
			} else if inst.LiveMetrics != nil {
				// Keep metrics pre-loaded from DB (last_metrics)
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
		} else if inst.LiveMetrics != nil {
			// Keep metrics loaded from DB (last_metrics)
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
		MetricSource:     req.MetricSource,
		UserID:           &userID,
		Visibility:       req.Visibility,
		AlertEnabled:     req.AlertEnabled,
		Notes:            req.Notes,
	}

	if inst.MetricSource == "" {
		if inst.Port == 8889 || inst.Port == 9100 || inst.PrometheusTarget != "" {
			inst.MetricSource = "prometheus"
		} else {
			inst.MetricSource = "ssh"
		}
	}
	if inst.MetricSource == "prometheus" {
		inst.RemoteHostID = nil
	}

	if inst.IPAddress == "" {
		inst.IPAddress = inst.Host
	}

	if inst.MetricSource == "ssh" {
		if req.SSHPort > 0 {
			inst.Port = req.SSHPort
		} else if inst.Port == 0 || inst.Port == 8889 {
			inst.Port = 22
		}

		// If no remote host linked, but user entered SSH credentials, auto-create and link a RemoteHostConfig
		if (inst.RemoteHostID == nil || *inst.RemoteHostID == "") && req.SSHUsername != "" {
			rhID := fmt.Sprintf("rhc-%s", uuid.New().String()[:8])
			sshPort := req.SSHPort
			if sshPort <= 0 {
				sshPort = 22
			}
			authType := req.SSHAuthType
			if authType == "" {
				authType = "password"
			}
			var passPtr, keyPtr *string
			if req.SSHPassword != "" {
				passPtr = &req.SSHPassword
			}
			if req.SSHKey != "" {
				keyPtr = &req.SSHKey
			}
			targetHost := inst.IPAddress
			if targetHost == "" {
				targetHost = inst.Host
			}
			rh := domain.RemoteHostConfig{
				ID:        rhID,
				Name:      inst.Name,
				Host:      targetHost,
				Port:      sshPort,
				Username:  req.SSHUsername,
				AuthType:  authType,
				Password:  passPtr,
				SSHKey:    keyPtr,
				GroupName: inst.GroupName,
				Tags:      inst.Tags,
			}
			if err := s.remoteHostRepo.Save(ctx, rh, userID, "ADMIN"); err == nil {
				inst.RemoteHostID = &rhID
			}
		} else if inst.RemoteHostID != nil && *inst.RemoteHostID != "" && (req.SSHPassword != "" || req.SSHKey != "" || req.SSHUsername != "") {
			if rh, err := s.remoteHostRepo.GetByID(ctx, *inst.RemoteHostID, userID, "ADMIN"); err == nil && rh != nil {
				if req.SSHPort > 0 {
					rh.Port = req.SSHPort
				}
				if req.SSHUsername != "" {
					rh.Username = req.SSHUsername
				}
				if req.SSHAuthType != "" {
					rh.AuthType = req.SSHAuthType
				}
				if req.SSHPassword != "" {
					rh.Password = &req.SSHPassword
				}
				if req.SSHKey != "" {
					rh.SSHKey = &req.SSHKey
				}
				_ = s.remoteHostRepo.Save(ctx, *rh, userID, "ADMIN")
			}
		}
	} else {
		if inst.Port == 0 {
			inst.Port = 8889
		}
	}

	if inst.MetricSource == "ssh" {
		inst.PrometheusTarget = ""
	} else if inst.PrometheusTarget == "" {
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
	if req.MetricSource != "" {
		inst.MetricSource = req.MetricSource
	}
	if inst.MetricSource == "" {
		if inst.RemoteHostID != nil && *inst.RemoteHostID != "" {
			inst.MetricSource = "ssh"
		} else if inst.Port == 8889 || inst.Port == 9100 || inst.PrometheusTarget != "" {
			inst.MetricSource = "prometheus"
		} else {
			inst.MetricSource = "ssh"
		}
	}
	if inst.MetricSource == "prometheus" {
		inst.RemoteHostID = nil
	} else if inst.MetricSource == "ssh" {
		inst.PrometheusTarget = ""
	}
	inst.Visibility = req.Visibility
	inst.AlertEnabled = req.AlertEnabled
	inst.Notes = req.Notes

	if inst.IPAddress == "" {
		inst.IPAddress = inst.Host
	}

	if inst.MetricSource == "ssh" {
		if req.SSHPort > 0 {
			inst.Port = req.SSHPort
		} else if inst.Port == 0 || inst.Port == 8889 {
			inst.Port = 22
		}

		if (inst.RemoteHostID == nil || *inst.RemoteHostID == "") && req.SSHUsername != "" {
			rhID := fmt.Sprintf("rhc-%s", uuid.New().String()[:8])
			sshPort := req.SSHPort
			if sshPort <= 0 {
				sshPort = 22
			}
			authType := req.SSHAuthType
			if authType == "" {
				authType = "password"
			}
			var passPtr, keyPtr *string
			if req.SSHPassword != "" {
				passPtr = &req.SSHPassword
			}
			if req.SSHKey != "" {
				keyPtr = &req.SSHKey
			}
			targetHost := inst.IPAddress
			if targetHost == "" {
				targetHost = inst.Host
			}
			rh := domain.RemoteHostConfig{
				ID:        rhID,
				Name:      inst.Name,
				Host:      targetHost,
				Port:      sshPort,
				Username:  req.SSHUsername,
				AuthType:  authType,
				Password:  passPtr,
				SSHKey:    keyPtr,
				GroupName: inst.GroupName,
				Tags:      inst.Tags,
			}
			if err := s.remoteHostRepo.Save(ctx, rh, userID, userRole); err == nil {
				inst.RemoteHostID = &rhID
			}
		} else if inst.RemoteHostID != nil && *inst.RemoteHostID != "" && (req.SSHPassword != "" || req.SSHKey != "" || req.SSHUsername != "") {
			if rh, err := s.remoteHostRepo.GetByID(ctx, *inst.RemoteHostID, userID, userRole); err == nil && rh != nil {
				if req.SSHPort > 0 {
					rh.Port = req.SSHPort
				}
				if req.SSHUsername != "" {
					rh.Username = req.SSHUsername
				}
				if req.SSHAuthType != "" {
					rh.AuthType = req.SSHAuthType
				}
				if req.SSHPassword != "" {
					rh.Password = &req.SSHPassword
				}
				if req.SSHKey != "" {
					rh.SSHKey = &req.SSHKey
				}
				_ = s.remoteHostRepo.Save(ctx, *rh, userID, userRole)
			}
		}
	} else {
		if inst.Port == 0 {
			inst.Port = 8889
		}
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

	if err := s.instRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.cacheMu.Lock()
	delete(s.metricsCache, id)
	s.cacheMu.Unlock()

	return nil
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

	if len(synced) > 0 {
		s.TriggerImmediatePoll()
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

type hostMetricCollector struct {
	hosts  []*hostMetricData
	byIP   map[string]*hostMetricData
	byHost map[string]*hostMetricData
	byInst map[string]*hostMetricData
}

func newHostMetricCollector() *hostMetricCollector {
	return &hostMetricCollector{
		byIP:   make(map[string]*hostMetricData),
		byHost: make(map[string]*hostMetricData),
		byInst: make(map[string]*hostMetricData),
	}
}

type hostMetricData struct {
	ipAddress        string
	hostname         string
	instanceTarget   string
	agentVersion     string
	isOnline         bool
	hasOTel          bool
	cpuPct           *float64
	cpuCount         int
	cpuPhysicalCount int
	cpuLoad1m        *float64
	cpuLoad5m        *float64
	cpuLoad15m       *float64
	memPct           *float64
	memUsed          float64
	memFree          float64
	memTotal         float64
	disks            map[string]*domain.InstanceDiskMetric
	netDown          float64
	netUp            float64
	uptimeSeconds    *float64
	osVersion        string
}

func (c *hostMetricCollector) getOrCreate(labels map[string]string) *hostMetricData {
	ip := strings.TrimSpace(labels["ip_address"])
	host := strings.TrimSpace(labels["hostname"])
	inst := strings.TrimSpace(labels["instance"])

	var h *hostMetricData
	if ip != "" {
		h = c.byIP[ip]
	}
	if h == nil && host != "" {
		h = c.byHost[strings.ToLower(host)]
	}
	if h == nil && inst != "" {
		h = c.byInst[inst]
	}

	if h != nil {
		if ip != "" && h.ipAddress == "" {
			h.ipAddress = ip
			c.byIP[ip] = h
		}
		if host != "" && h.hostname == "" {
			h.hostname = host
			c.byHost[strings.ToLower(host)] = h
		}
		if inst != "" && h.instanceTarget == "" {
			h.instanceTarget = inst
			c.byInst[inst] = h
		}
		if ver := labels["otel_scope_version"]; ver != "" && (h.agentVersion == "" || h.agentVersion == "N/A") {
			h.agentVersion = ver
		}
		return h
	}

	h = &hostMetricData{
		ipAddress:      ip,
		hostname:       host,
		instanceTarget: inst,
		agentVersion:   labels["otel_scope_version"],
		disks:          make(map[string]*domain.InstanceDiskMetric),
	}
	if ip != "" {
		c.byIP[ip] = h
	}
	if host != "" {
		c.byHost[strings.ToLower(host)] = h
	}
	if inst != "" {
		c.byInst[inst] = h
	}
	c.hosts = append(c.hosts, h)
	return h
}

func (s *MonitoringInstanceService) BatchGetLiveMetrics(ctx context.Context, instances []*domain.MonitoringInstance) {
	if len(instances) == 0 {
		return
	}

	collector := newHostMetricCollector()

	// 1. Query Up status (Populates upMap ONLY, does NOT create dummy host records)
	upMap := make(map[string]bool)
	upRes, err := s.executeInstantQuery(ctx, "up")
	if err == nil && upRes != nil && upRes.Status == "success" {
		for _, item := range upRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && v > 0 {
					upMap[item.Metric["instance"]] = true
				}
			}
		}
	}

	// 2. Query CPU %: OpenTelemetry hostmetrics receiver
	// Primary: (1 - avg by (ip_address, hostname, instance) (host_system_cpu_utilization_ratio{state="idle"})) * 100
	cpuRes, err := s.executeInstantQuery(ctx, `(1 - avg by (ip_address, hostname, instance) (host_system_cpu_utilization_ratio{state="idle"})) * 100`)
	if err != nil || cpuRes == nil || len(cpuRes.Data.Result) == 0 {
		cpuRes, _ = s.executeInstantQuery(ctx, `(1 - avg by (ip_address, hostname, instance) (system_cpu_utilization_ratio{state="idle"})) * 100`)
	}
	if cpuRes == nil || len(cpuRes.Data.Result) == 0 {
		cpuRes, _ = s.executeInstantQuery(ctx, `(1 - (sum by (ip_address, hostname, instance) (rate(host_system_cpu_time_seconds_total{state="idle"}[2m])) / sum by (ip_address, hostname, instance) (rate(host_system_cpu_time_seconds_total[2m])))) * 100`)
	}
	if cpuRes == nil || len(cpuRes.Data.Result) == 0 {
		cpuRes, _ = s.executeInstantQuery(ctx, `(1 - (sum by (instance) (rate(system_cpu_time_seconds_total{state="idle"}[2m])) / sum by (instance) (rate(system_cpu_time_seconds_total[2m])))) * 100`)
	}
	if cpuRes != nil && cpuRes.Status == "success" {
		for _, item := range cpuRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					pct := math.Round(math.Max(0, math.Min(100, v))*10) / 10
					h := collector.getOrCreate(item.Metric)
					h.cpuPct = &pct
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 3. Query CPU Logical Count: host_system_cpu_logical_count
	coreRes, err := s.executeInstantQuery(ctx, `host_system_cpu_logical_count`)
	if err != nil || coreRes == nil || len(coreRes.Data.Result) == 0 {
		coreRes, _ = s.executeInstantQuery(ctx, `system_cpu_logical_count`)
	}
	if coreRes == nil || len(coreRes.Data.Result) == 0 {
		coreRes, _ = s.executeInstantQuery(ctx, `count by (ip_address, hostname, instance) (host_system_cpu_time_seconds_total{state="idle"})`)
	}
	if coreRes == nil || len(coreRes.Data.Result) == 0 {
		coreRes, _ = s.executeInstantQuery(ctx, `count by (instance) (system_cpu_time_seconds_total{state="idle"})`)
	}
	if coreRes != nil && coreRes.Status == "success" {
		for _, item := range coreRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && v > 0 {
					h := collector.getOrCreate(item.Metric)
					h.cpuCount = int(v)
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 3b. Query CPU Physical Count: host_system_cpu_physical_count
	physRes, err := s.executeInstantQuery(ctx, `host_system_cpu_physical_count`)
	if err != nil || physRes == nil || len(physRes.Data.Result) == 0 {
		physRes, _ = s.executeInstantQuery(ctx, `system_cpu_physical_count`)
	}
	if physRes != nil && physRes.Status == "success" {
		for _, item := range physRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && v > 0 {
					h := collector.getOrCreate(item.Metric)
					h.cpuPhysicalCount = int(v)
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 3c. Query CPU Load Averages (1m, 5m, 15m)
	load1Res, _ := s.executeInstantQuery(ctx, `host_system_cpu_load_average_1m`)
	if load1Res == nil || len(load1Res.Data.Result) == 0 {
		load1Res, _ = s.executeInstantQuery(ctx, `system_cpu_load_average_1m`)
	}
	if load1Res != nil && load1Res.Status == "success" {
		for _, item := range load1Res.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					val := math.Round(v*100) / 100
					h := collector.getOrCreate(item.Metric)
					h.cpuLoad1m = &val
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	load5Res, _ := s.executeInstantQuery(ctx, `host_system_cpu_load_average_5m`)
	if load5Res == nil || len(load5Res.Data.Result) == 0 {
		load5Res, _ = s.executeInstantQuery(ctx, `system_cpu_load_average_5m`)
	}
	if load5Res != nil && load5Res.Status == "success" {
		for _, item := range load5Res.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					val := math.Round(v*100) / 100
					h := collector.getOrCreate(item.Metric)
					h.cpuLoad5m = &val
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	load15Res, _ := s.executeInstantQuery(ctx, `host_system_cpu_load_average_15m`)
	if load15Res == nil || len(load15Res.Data.Result) == 0 {
		load15Res, _ = s.executeInstantQuery(ctx, `system_cpu_load_average_15m`)
	}
	if load15Res != nil && load15Res.Status == "success" {
		for _, item := range load15Res.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					val := math.Round(v*100) / 100
					h := collector.getOrCreate(item.Metric)
					h.cpuLoad15m = &val
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 4. Query Memory %
	memPctRes, err := s.executeInstantQuery(ctx, `host_system_memory_utilization_ratio{state="used"} * 100`)
	if err != nil || memPctRes == nil || len(memPctRes.Data.Result) == 0 {
		memPctRes, _ = s.executeInstantQuery(ctx, `system_memory_utilization_ratio{state="used"} * 100`)
	}
	if memPctRes != nil && memPctRes.Status == "success" {
		for _, item := range memPctRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
					pct := math.Round(math.Max(0, math.Min(100, v))*10) / 10
					h := collector.getOrCreate(item.Metric)
					h.memPct = &pct
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 4b. Query Memory Bytes: Used, Free, Limit
	memUsedRes, _ := s.executeInstantQuery(ctx, `sum by (ip_address, hostname, instance) (host_system_memory_usage_bytes{state="used"})`)
	if memUsedRes == nil || len(memUsedRes.Data.Result) == 0 {
		memUsedRes, _ = s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes{state="used"})`)
	}
	if memUsedRes != nil && memUsedRes.Status == "success" {
		for _, item := range memUsedRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					h := collector.getOrCreate(item.Metric)
					h.memUsed = v
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	memFreeRes, _ := s.executeInstantQuery(ctx, `sum by (ip_address, hostname, instance) (host_system_memory_usage_bytes{state="free"})`)
	if memFreeRes == nil || len(memFreeRes.Data.Result) == 0 {
		memFreeRes, _ = s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes{state="free"})`)
	}
	if memFreeRes != nil && memFreeRes.Status == "success" {
		for _, item := range memFreeRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					h := collector.getOrCreate(item.Metric)
					h.memFree = v
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	memLimitRes, _ := s.executeInstantQuery(ctx, `host_system_memory_limit_bytes`)
	if memLimitRes == nil || len(memLimitRes.Data.Result) == 0 {
		memLimitRes, _ = s.executeInstantQuery(ctx, `sum by (ip_address, hostname, instance) (host_system_memory_usage_bytes)`)
	}
	if memLimitRes == nil || len(memLimitRes.Data.Result) == 0 {
		memLimitRes, _ = s.executeInstantQuery(ctx, `sum by (instance) (system_memory_usage_bytes)`)
	}
	if memLimitRes != nil && memLimitRes.Status == "success" {
		for _, item := range memLimitRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					h := collector.getOrCreate(item.Metric)
					h.memTotal = v
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// Calculate memory percentages or used/free bytes consistency
	for _, h := range collector.hosts {
		if h.memTotal > 0 {
			if h.memPct == nil && h.memUsed > 0 {
				calc := math.Round((h.memUsed/h.memTotal)*1000) / 10
				h.memPct = &calc
			} else if h.memPct != nil && h.memUsed == 0 {
				h.memUsed = (*h.memPct / 100.0) * h.memTotal
				h.memFree = h.memTotal - h.memUsed
			}
		}
	}

	// 5. Query Filesystem: Utilization %, Used bytes, Free bytes
	fsPctRes, _ := s.executeInstantQuery(ctx, `host_system_filesystem_utilization_ratio * 100`)
	if fsPctRes == nil || len(fsPctRes.Data.Result) == 0 {
		fsPctRes, _ = s.executeInstantQuery(ctx, `system_filesystem_utilization_ratio * 100`)
	}
	if fsPctRes != nil && fsPctRes.Status == "success" {
		for _, item := range fsPctRes.Data.Result {
			mp := item.Metric["mountpoint"]
			if mp == "" {
				mp = "/"
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					pct := math.Round(math.Max(0, math.Min(100, v))*10) / 10
					h := collector.getOrCreate(item.Metric)
					h.hasOTel = true
					h.isOnline = true
					d, exists := h.disks[mp]
					if !exists {
						d = &domain.InstanceDiskMetric{
							Mountpoint: mp,
							Device:     item.Metric["device"],
							FSType:     item.Metric["type"],
						}
						h.disks[mp] = d
					}
					d.UsagePct = pct
				}
			}
		}
	}

	fsUsedRes, _ := s.executeInstantQuery(ctx, `host_system_filesystem_usage_bytes{state="used"}`)
	if fsUsedRes == nil || len(fsUsedRes.Data.Result) == 0 {
		fsUsedRes, _ = s.executeInstantQuery(ctx, `system_filesystem_usage_bytes{state="used"}`)
	}
	if fsUsedRes != nil && fsUsedRes.Status == "success" {
		for _, item := range fsUsedRes.Data.Result {
			mp := item.Metric["mountpoint"]
			if mp == "" {
				mp = "/"
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					h := collector.getOrCreate(item.Metric)
					h.hasOTel = true
					h.isOnline = true
					d, exists := h.disks[mp]
					if !exists {
						d = &domain.InstanceDiskMetric{
							Mountpoint: mp,
							Device:     item.Metric["device"],
							FSType:     item.Metric["type"],
						}
						h.disks[mp] = d
					}
					d.UsedBytes = v
				}
			}
		}
	}

	fsFreeRes, _ := s.executeInstantQuery(ctx, `host_system_filesystem_usage_bytes{state="free"}`)
	if fsFreeRes == nil || len(fsFreeRes.Data.Result) == 0 {
		fsFreeRes, _ = s.executeInstantQuery(ctx, `system_filesystem_usage_bytes{state="free"}`)
	}
	if fsFreeRes != nil && fsFreeRes.Status == "success" {
		for _, item := range fsFreeRes.Data.Result {
			mp := item.Metric["mountpoint"]
			if mp == "" {
				mp = "/"
			}
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok {
					h := collector.getOrCreate(item.Metric)
					h.hasOTel = true
					h.isOnline = true
					d, exists := h.disks[mp]
					if !exists {
						d = &domain.InstanceDiskMetric{
							Mountpoint: mp,
							Device:     item.Metric["device"],
							FSType:     item.Metric["type"],
						}
						h.disks[mp] = d
					}
					d.FreeBytes = v
				}
			}
		}
	}

	// Format disk sizes and human strings
	for _, h := range collector.hosts {
		for _, d := range h.disks {
			total := d.UsedBytes + d.FreeBytes
			if total > 0 {
				d.TotalBytes = total
				d.UsageHuman = fmt.Sprintf("%s / %s", formatBytes(d.UsedBytes), formatBytes(total))
				if d.UsagePct == 0 && d.UsedBytes > 0 {
					d.UsagePct = math.Round((d.UsedBytes/total)*1000) / 10
				}
			} else if d.UsagePct > 0 {
				d.UsageHuman = fmt.Sprintf("%.1f%%", d.UsagePct)
			}
		}
	}

	// 6. Query Network (MB/s)
	netDownRes, _ := s.executeInstantQuery(ctx, `sum by (ip_address, hostname, instance) (rate(host_system_network_io_bytes_total{direction="receive"}[2m])) / 1048576`)
	if netDownRes == nil || len(netDownRes.Data.Result) == 0 {
		netDownRes, _ = s.executeInstantQuery(ctx, `sum by (instance) (rate(system_network_io_bytes_total{direction="receive"}[2m])) / 1048576`)
	}
	if netDownRes != nil && netDownRes.Status == "success" {
		for _, item := range netDownRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					h := collector.getOrCreate(item.Metric)
					h.netDown = math.Round(math.Max(0, v)*100) / 100
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	netUpRes, _ := s.executeInstantQuery(ctx, `sum by (ip_address, hostname, instance) (rate(host_system_network_io_bytes_total{direction="transmit"}[2m])) / 1048576`)
	if netUpRes == nil || len(netUpRes.Data.Result) == 0 {
		netUpRes, _ = s.executeInstantQuery(ctx, `sum by (instance) (rate(system_network_io_bytes_total{direction="transmit"}[2m])) / 1048576`)
	}
	if netUpRes != nil && netUpRes.Status == "success" {
		for _, item := range netUpRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && !math.IsNaN(v) {
					h := collector.getOrCreate(item.Metric)
					h.netUp = math.Round(math.Max(0, v)*100) / 100
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	// 7. Query Uptime (seconds)
	uptimeRes, _ := s.executeInstantQuery(ctx, `max by (ip_address, hostname, instance) (host_process_uptime_seconds)`)
	if uptimeRes == nil || len(uptimeRes.Data.Result) == 0 {
		uptimeRes, _ = s.executeInstantQuery(ctx, `max by (instance) (host_process_uptime_seconds)`)
	}
	if uptimeRes != nil && uptimeRes.Status == "success" {
		for _, item := range uptimeRes.Data.Result {
			if len(item.Value) > 1 {
				if v, ok := parseFloat(item.Value[1]); ok && v > 0 {
					h := collector.getOrCreate(item.Metric)
					h.uptimeSeconds = &v
					h.hasOTel = true
					h.isOnline = true
				}
			}
		}
	}

	now := time.Now()

	for _, inst := range instances {
		// Never query or match Prometheus metrics for instances designated for SSH
		if inst.MetricSource == "ssh" || (inst.RemoteHostID != nil && *inst.RemoteHostID != "" && inst.MetricSource != "prometheus") {
			continue
		}

		var matched *hostMetricData
		instIP := strings.TrimSpace(inst.IPAddress)
		if instIP == "" {
			instIP = strings.TrimSpace(inst.Host)
		}
		instTarget := strings.TrimSpace(inst.PrometheusTarget)
		instNameLower := strings.ToLower(inst.Name)
		instHostLower := strings.ToLower(inst.Host)

		// Tier 1: Exact PrometheusTarget match (e.g. 10.20.3.36:8889)
		if instTarget != "" {
			for _, h := range collector.hosts {
				if h.hasOTel && h.instanceTarget == instTarget {
					matched = h
					break
				}
			}
		}

		// Tier 2: Hostname + IP match
		if matched == nil {
			for _, h := range collector.hosts {
				if !h.hasOTel {
					continue
				}
				hLower := strings.ToLower(h.hostname)
				if hLower != "" && (hLower == instHostLower || strings.Contains(instNameLower, hLower)) {
					if h.ipAddress != "" && (h.ipAddress == instIP || h.ipAddress == inst.Host) {
						matched = h
						break
					}
				}
			}
		}

		// Tier 3: Hostname match alone
		if matched == nil {
			for _, h := range collector.hosts {
				if !h.hasOTel {
					continue
				}
				hLower := strings.ToLower(h.hostname)
				if hLower != "" && (hLower == instHostLower || strings.Contains(instNameLower, hLower)) {
					matched = h
					break
				}
			}
		}

		// Tier 4: IP match alone (with port check to avoid matching e.g. port 9090 when exporter is on 8889)
		if matched == nil {
			for _, h := range collector.hosts {
				if !h.hasOTel {
					continue
				}
				if h.ipAddress != "" && (h.ipAddress == instIP || h.ipAddress == inst.Host) {
					if instTarget != "" && h.instanceTarget != "" && !strings.EqualFold(instTarget, h.instanceTarget) {
						continue
					}
					matched = h
					break
				}
			}
		}

		if matched == nil || !matched.hasOTel {
			inst.LiveMetrics = &domain.InstanceLiveMetrics{
				IsOnline:         false,
				AgentVersion:     "N/A",
				HasOTel:          false,
				CPUPct:           nil,
				CPUCount:         0,
				CPUPhysicalCount: 0,
				MemPct:           nil,
				MemUsedBytes:     0,
				MemFreeBytes:     0,
				MemTotalBytes:    0,
				DiskPct:          nil,
				Disks:            []domain.InstanceDiskMetric{},
				NetDownloadMB:    0,
				NetUploadMB:      0,
				NetTotalMB:       0,
				UptimeHuman:      "N/A",
				OSVersion:        "N/A",
				LastUpdated:      now,
			}
			continue
		}

		// Save detected hostname if inst doesn't have an explicit hostname
		if inst.Hostname == "" && matched.hostname != "" {
			inst.Hostname = matched.hostname
		}

		ver := matched.agentVersion
		if ver == "" {
			ver = "0.159.0"
		}

		// Overall disk calculation across all storage partitions
		var overallDiskPct *float64
		var diskList []domain.InstanceDiskMetric
		var sumUsed, sumTotal float64
		for _, d := range matched.disks {
			diskList = append(diskList, *d)
			sumUsed += d.UsedBytes
			sumTotal += d.TotalBytes
		}
		if sumTotal > 0 {
			pct := math.Round((sumUsed/sumTotal)*1000) / 10
			overallDiskPct = &pct
		} else if len(diskList) > 0 {
			pct := diskList[0].UsagePct
			overallDiskPct = &pct
		}

		cpuCount := matched.cpuCount
		if cpuCount == 0 && matched.cpuPct != nil {
			cpuCount = 1
		}

		netTotal := math.Round((matched.netDown+matched.netUp)*100) / 100

		// Check online status against upMap or if metrics are live
		isOnline := false
		if upMap[matched.instanceTarget] || upMap[inst.PrometheusTarget] || upMap[fmt.Sprintf("%s:%d", instIP, inst.Port)] || matched.isOnline || matched.hasOTel {
			isOnline = true
		}

		// Format Uptime Human
		uptimeHuman := "N/A"
		if matched.uptimeSeconds != nil && *matched.uptimeSeconds > 0 {
			uptimeHuman = formatUptimeHuman(*matched.uptimeSeconds)
		}

		// Detect OS Version / Platform
		osVer := matched.osVersion
		if osVer == "" {
			isLinux := false
			isWindows := false
			for _, d := range matched.disks {
				if d.FSType == "ext4" || d.FSType == "xfs" || strings.HasPrefix(d.Mountpoint, "/") {
					isLinux = true
				}
				if d.FSType == "ntfs" || strings.HasPrefix(d.Mountpoint, "C:") {
					isWindows = true
				}
			}
			if isLinux {
				osVer = "Linux"
			} else if isWindows {
				osVer = "Windows"
			} else if matched.hasOTel {
				osVer = "Linux"
			} else {
				osVer = "N/A"
			}
		}

		inst.LiveMetrics = &domain.InstanceLiveMetrics{
			IsOnline:         isOnline,
			DetectedHostname: matched.hostname,
			AgentVersion:     ver,
			HasOTel:          true,
			CPUPct:           matched.cpuPct,
			CPUCount:         cpuCount,
			CPUPhysicalCount: matched.cpuPhysicalCount,
			CPULoad1m:        matched.cpuLoad1m,
			CPULoad5m:        matched.cpuLoad5m,
			CPULoad15m:       matched.cpuLoad15m,
			MemPct:           matched.memPct,
			MemUsedBytes:     matched.memUsed,
			MemFreeBytes:     matched.memFree,
			MemTotalBytes:    matched.memTotal,
			DiskPct:          overallDiskPct,
			DiskUsedBytes:    sumUsed,
			DiskFreeBytes:    math.Max(0, sumTotal-sumUsed),
			DiskTotalBytes:   sumTotal,
			Disks:            diskList,
			NetDownloadMB:    matched.netDown,
			NetUploadMB:      matched.netUp,
			NetTotalMB:       netTotal,
			UptimeSeconds:    matched.uptimeSeconds,
			UptimeHuman:      uptimeHuman,
			OSVersion:        osVer,
			LastUpdated:      now,
		}
	}
}

func formatUptimeHuman(seconds float64) string {
	if seconds <= 0 {
		return "N/A"
	}
	totalSec := int64(seconds)
	days := totalSec / 86400
	hours := (totalSec % 86400) / 3600
	minutes := (totalSec % 3600) / 60
	secs := totalSec % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
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

	ip := strings.TrimSpace(inst.IPAddress)
	if ip == "" {
		ip = strings.TrimSpace(inst.Host)
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

	// For SSH instances, retrieve historical telemetry directly from PostgreSQL table (instance_metrics_history)
	isSSH := inst.MetricSource == "ssh" || (inst.RemoteHostID != nil && *inst.RemoteHostID != "") || (inst.Port == 22 && (inst.PrometheusTarget == "" || inst.MetricSource != "prometheus"))
	if isSSH {
		dbHistory, err := s.instRepo.GetMetricsHistoryFromDB(ctx, inst.ID, startTime)
		if err != nil {
			return nil, err
		}
		if dbHistory != nil {
			dbHistory.TimeRange = timeRange
			return dbHistory, nil
		}
		return &domain.InstanceHistoryResponse{
			InstanceID: inst.ID,
			TimeRange:  timeRange,
			CPU:        []domain.MetricHistoryPoint{},
			Memory:     []domain.MetricHistoryPoint{},
			Disk:       []domain.MetricHistoryPoint{},
			NetIn:      []domain.MetricHistoryPoint{},
			NetOut:     []domain.MetricHistoryPoint{},
		}, nil
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
		var pts []domain.MetricHistoryPoint
		if ip != "" {
			qHost := fmt.Sprintf(`(1 - avg(host_system_cpu_utilization_ratio{ip_address="%s",state="idle"})) * 100`, ip)
			r, err := s.executeRangeQuery(ctx, qHost, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 {
			qFallback := fmt.Sprintf(`(1 - (sum(rate(system_cpu_time_seconds_total{instance=~"%s",state="idle"}[2m])) / sum(rate(system_cpu_time_seconds_total{instance=~"%s"}[2m])))) * 100`, targetPattern, targetPattern)
			r, err := s.executeRangeQuery(ctx, qFallback, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.CPU = pts
		mu.Unlock()
	}()

	// 2. Memory History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if ip != "" {
			qHost := fmt.Sprintf(`avg(host_system_memory_utilization_ratio{ip_address="%s",state="used"}) * 100`, ip)
			r, err := s.executeRangeQuery(ctx, qHost, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 {
			qFallback := fmt.Sprintf(`(sum(system_memory_usage_bytes{instance=~"%s",state="used"}) / sum(system_memory_usage_bytes{instance=~"%s"})) * 100`, targetPattern, targetPattern)
			r, err := s.executeRangeQuery(ctx, qFallback, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.Memory = pts
		mu.Unlock()
	}()

	// 3. Disk History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if ip != "" {
			qHost := fmt.Sprintf(`avg(host_system_filesystem_utilization_ratio{ip_address="%s",mountpoint="/"}) * 100`, ip)
			r, err := s.executeRangeQuery(ctx, qHost, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 {
			qFallback := fmt.Sprintf(`(sum(system_filesystem_usage_bytes{instance=~"%s",state="used"}) / (sum(system_filesystem_usage_bytes{instance=~"%s",state="used"}) + sum(system_filesystem_usage_bytes{instance=~"%s",state="free"}))) * 100`, targetPattern, targetPattern, targetPattern)
			r, err := s.executeRangeQuery(ctx, qFallback, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.Disk = pts
		mu.Unlock()
	}()

	// 4. Net In (Download MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if ip != "" {
			qHost := fmt.Sprintf(`sum(rate(host_system_network_io_bytes_total{ip_address="%s",direction="receive"}[2m])) / 1048576`, ip)
			r, err := s.executeRangeQuery(ctx, qHost, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 {
			qFallback := fmt.Sprintf(`sum(rate(system_network_io_bytes_total{instance=~"%s",direction="receive"}[2m])) / 1048576`, targetPattern)
			r, err := s.executeRangeQuery(ctx, qFallback, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.NetIn = pts
		mu.Unlock()
	}()

	// 5. Net Out (Upload MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if ip != "" {
			qHost := fmt.Sprintf(`sum(rate(host_system_network_io_bytes_total{ip_address="%s",direction="transmit"}[2m])) / 1048576`, ip)
			r, err := s.executeRangeQuery(ctx, qHost, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 {
			qFallback := fmt.Sprintf(`sum(rate(system_network_io_bytes_total{instance=~"%s",direction="transmit"}[2m])) / 1048576`, targetPattern)
			r, err := s.executeRangeQuery(ctx, qFallback, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.NetOut = pts
		mu.Unlock()
	}()

	wg.Wait()

	// If Prometheus returned no data, fallback to PostgreSQL database history
	if len(resp.CPU) == 0 && len(resp.Memory) == 0 {
		dbHistory, err := s.instRepo.GetMetricsHistoryFromDB(ctx, inst.ID, startTime)
		if err == nil && dbHistory != nil && (len(dbHistory.CPU) > 0 || len(dbHistory.Memory) > 0) {
			dbHistory.TimeRange = timeRange
			return dbHistory, nil
		}
	}

	return resp, nil
}

// GetContainerHistory retrieves historical series data for interactive line charts for a Docker container
func (s *MonitoringInstanceService) GetContainerHistory(
	ctx context.Context,
	containerID string,
	containerName string,
	hostname string,
	ipAddress string,
	timeRange string,
) (*domain.InstanceHistoryResponse, error) {
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

	instID := containerID
	if instID == "" {
		instID = containerName
	}

	resp := &domain.InstanceHistoryResponse{
		InstanceID: instID,
		TimeRange:  timeRange,
		CPU:        []domain.MetricHistoryPoint{},
		Memory:     []domain.MetricHistoryPoint{},
		Disk:       []domain.MetricHistoryPoint{},
		NetIn:      []domain.MetricHistoryPoint{},
		NetOut:     []domain.MetricHistoryPoint{},
	}

	var matchers []string
	if containerID != "" {
		matchers = append(matchers, fmt.Sprintf(`container_id=~"^%s.*"`, regexp.QuoteMeta(containerID)))
	} else if containerName != "" {
		matchers = append(matchers, fmt.Sprintf(`container_name="%s"`, containerName))
	}
	if hostname != "" {
		matchers = append(matchers, fmt.Sprintf(`hostname="%s"`, hostname))
	}

	filter := strings.Join(matchers, ",")
	if filter != "" {
		filter = "{" + filter + "}"
	}

	nameFilter := ""
	if containerName != "" {
		nameFilter = fmt.Sprintf(`{container_name="%s"}`, containerName)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	parsePoints := func(res *promMatrixResponse) []domain.MetricHistoryPoint {
		var points []domain.MetricHistoryPoint
		if res == nil || res.Status != "success" || len(res.Data.Result) == 0 {
			return points
		}
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
		var pts []domain.MetricHistoryPoint
		if filter != "" {
			q := fmt.Sprintf(`avg(container_cpu_utilization_ratio%s)`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && nameFilter != "" {
			q := fmt.Sprintf(`avg(container_cpu_utilization_ratio%s)`, nameFilter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && filter != "" {
			q := fmt.Sprintf(`sum(rate(container_cpu_usage_nanoseconds_total%s[2m])) / 10000000`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.CPU = pts
		mu.Unlock()
	}()

	// 2. Memory History %
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if filter != "" {
			q := fmt.Sprintf(`avg(container_memory_percent_ratio%s)`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && nameFilter != "" {
			q := fmt.Sprintf(`avg(container_memory_percent_ratio%s)`, nameFilter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && filter != "" {
			q := fmt.Sprintf(`(sum(container_memory_usage_total_bytes%s) / sum(container_memory_usage_limit_bytes%s)) * 100`, filter, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.Memory = pts
		mu.Unlock()
	}()

	// 3. Disk / Block I/O (MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if filter != "" {
			q := fmt.Sprintf(`sum(rate(container_blockio_io_service_bytes_recursive_total%s[2m])) / 1048576`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && nameFilter != "" {
			q := fmt.Sprintf(`sum(rate(container_blockio_io_service_bytes_recursive_total%s[2m])) / 1048576`, nameFilter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.Disk = pts
		mu.Unlock()
	}()

	// 4. Net In (Download MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if filter != "" {
			q := fmt.Sprintf(`sum(rate(container_network_io_usage_rx_bytes_total%s[2m])) / 1048576`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && nameFilter != "" {
			q := fmt.Sprintf(`sum(rate(container_network_io_usage_rx_bytes_total%s[2m])) / 1048576`, nameFilter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.NetIn = pts
		mu.Unlock()
	}()

	// 5. Net Out (Upload MB/s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var pts []domain.MetricHistoryPoint
		if filter != "" {
			q := fmt.Sprintf(`sum(rate(container_network_io_usage_tx_bytes_total%s[2m])) / 1048576`, filter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		if len(pts) == 0 && nameFilter != "" {
			q := fmt.Sprintf(`sum(rate(container_network_io_usage_tx_bytes_total%s[2m])) / 1048576`, nameFilter)
			r, err := s.executeRangeQuery(ctx, q, startTime, now, step)
			if err == nil {
				pts = parsePoints(r)
			}
		}
		mu.Lock()
		resp.NetOut = pts
		mu.Unlock()
	}()

	wg.Wait()

	return resp, nil
}

// ==================== SSH METRICS POLLING ENGINE ====================

// pollSSHInstances polls telemetry for instances that are configured for SSH metric collection
func (s *MonitoringInstanceService) pollSSHInstances(ctx context.Context, instances []*domain.MonitoringInstance) {
	if s.vpsService == nil {
		return
	}

	var targets []*domain.MonitoringInstance
	for _, inst := range instances {
		if inst.MetricSource == "prometheus" {
			continue
		}
		if inst.RemoteHostID == nil || *inst.RemoteHostID == "" {
			continue
		}
		targets = append(targets, inst)
	}

	if len(targets) == 0 {
		return
	}

	// Concurrency limiter: max 10 concurrent SSH handshakes to protect local daemon and remote targets
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, inst := range targets {
		wg.Add(1)
		go func(target *domain.MonitoringInstance) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Strict 8-second context timeout per host
			hostCtx, hostCancel := context.WithTimeout(ctx, 8*time.Second)
			defer hostCancel()

			liveMetrics, err := s.fetchInstanceSSHMetrics(hostCtx, target)
			if err == nil && liveMetrics != nil {
				target.LiveMetrics = liveMetrics
			} else if err != nil {
				logger.Warn("MonitoringSSH", fmt.Sprintf("Failed to fetch SSH metrics for %s (%s): %v", target.Name, target.Host, err))
			}
		}(inst)
	}

	wg.Wait()
}

// fetchInstanceSSHMetrics fetches system utilization from remote host via SSH and maps to InstanceLiveMetrics
func (s *MonitoringInstanceService) fetchInstanceSSHMetrics(ctx context.Context, inst *domain.MonitoringInstance) (*domain.InstanceLiveMetrics, error) {
	if s.vpsService == nil || inst.RemoteHostID == nil || *inst.RemoteHostID == "" {
		return nil, fmt.Errorf("no remote host linked")
	}

	data, err := s.vpsService.GetMetrics(ctx, *inst.RemoteHostID)
	if err != nil {
		return nil, err
	}

	lm := s.mapVpsMetricsToLiveMetrics(data)
	if lm.DetectedHostname == "" {
		lm.DetectedHostname = inst.Name
	}
	return lm, nil
}

// FetchSSHMetricsNow forces an immediate SSH telemetry pull for a single instance
func (s *MonitoringInstanceService) FetchSSHMetricsNow(ctx context.Context, id string, userID int, userRole string) (*domain.InstanceLiveMetrics, error) {
	inst, err := s.instRepo.GetByID(ctx, id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if inst.RemoteHostID == nil || *inst.RemoteHostID == "" {
		return nil, fmt.Errorf("instance is not linked to any Remote Host SSH connection")
	}

	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	metrics, err := s.fetchInstanceSSHMetrics(callCtx, inst)
	if err != nil {
		return nil, err
	}

	// Update memory cache
	s.cacheMu.Lock()
	s.metricsCache[inst.ID] = metrics
	s.cacheMu.Unlock()

	// Persist live snapshot
	_ = s.instRepo.SaveLiveMetrics(ctx, inst.ID, metrics)

	// Persist history point
	var cpuVal, memVal, diskVal, netVal float64
	if metrics.CPUPct != nil {
		cpuVal = *metrics.CPUPct
	}
	if metrics.MemPct != nil {
		memVal = *metrics.MemPct
	}
	if metrics.DiskPct != nil {
		diskVal = *metrics.DiskPct
	}
	netVal = metrics.NetTotalMB
	_ = s.instRepo.SaveMetricsHistory(ctx, inst.ID, cpuVal, memVal, diskVal, netVal, metrics.NetDownloadMB, metrics.NetUploadMB, "ssh")

	return metrics, nil
}

// mapVpsMetricsToLiveMetrics transforms raw VPS metrics map into standard InstanceLiveMetrics
func (s *MonitoringInstanceService) mapVpsMetricsToLiveMetrics(data map[string]interface{}) *domain.InstanceLiveMetrics {
	lm := &domain.InstanceLiveMetrics{
		IsOnline:     true,
		AgentVersion: "SSH (Agentless)",
		HasOTel:      false,
		LastUpdated:  time.Now(),
		Disks:        []domain.InstanceDiskMetric{},
	}

	if h, ok := data["hostname"].(string); ok {
		lm.DetectedHostname = h
	}

	if osName, ok := data["osName"].(string); ok {
		kernel, _ := data["kernel"].(string)
		if kernel != "" && kernel != "-" {
			lm.OSVersion = fmt.Sprintf("%s (%s)", osName, kernel)
		} else {
			lm.OSVersion = osName
		}
	}

	if upt, ok := data["uptime"].(string); ok {
		lm.UptimeHuman = upt
	}

	if cores, ok := data["cpuCores"].(int); ok {
		lm.CPUCount = cores
	}

	if cpu, ok := data["cpuUsage"].(float64); ok {
		val := math.Round(cpu*10) / 10
		lm.CPUPct = &val
	}

	if loadStr, ok := data["loadAverage"].(string); ok && loadStr != "" {
		l1, l5, l15 := parseLoadAverage(loadStr)
		lm.CPULoad1m = l1
		lm.CPULoad5m = l5
		lm.CPULoad15m = l15
	}

	if memPct, ok := data["memPercent"].(float64); ok {
		val := math.Round(memPct*10) / 10
		lm.MemPct = &val
	}
	if memUsed, ok := data["memUsed"].(string); ok {
		lm.MemUsedBytes = parseHumanBytes(memUsed)
	}
	if memTotal, ok := data["memTotal"].(string); ok {
		lm.MemTotalBytes = parseHumanBytes(memTotal)
	}
	if memFree, ok := data["memFree"].(string); ok {
		lm.MemFreeBytes = parseHumanBytes(memFree)
	}

	if rawDisks, ok := data["disks"].([]map[string]interface{}); ok {
		var maxUsage float64
		var totalUsed, totalCap, totalFree float64
		foundRoot := false

		for _, d := range rawDisks {
			mount, _ := d["mount"].(string)
			fs, _ := d["filesystem"].(string)
			totStr, _ := d["total"].(string)
			usedStr, _ := d["used"].(string)
			availStr, _ := d["avail"].(string)

			pctVal := 0.0
			switch p := d["percent"].(type) {
			case int:
				pctVal = float64(p)
			case float64:
				pctVal = p
			}

			totBytes := parseHumanBytes(totStr)
			usedBytes := parseHumanBytes(usedStr)
			availBytes := parseHumanBytes(availStr)

			totalUsed += usedBytes
			totalCap += totBytes
			totalFree += availBytes

			diskMetric := domain.InstanceDiskMetric{
				Mountpoint: mount,
				Device:     fs,
				UsagePct:   pctVal,
				UsedBytes:  usedBytes,
				TotalBytes: totBytes,
				FreeBytes:  availBytes,
				UsageHuman: fmt.Sprintf("%s / %s (%.0f%%)", usedStr, totStr, pctVal),
			}
			lm.Disks = append(lm.Disks, diskMetric)

			if mount == "/" {
				foundRoot = true
				rootPct := pctVal
				lm.DiskPct = &rootPct
				lm.DiskUsedBytes = usedBytes
				lm.DiskTotalBytes = totBytes
				lm.DiskFreeBytes = availBytes
			} else if !foundRoot && pctVal > maxUsage {
				maxUsage = pctVal
			}
		}

		if !foundRoot && len(lm.Disks) > 0 {
			if totalCap > 0 {
				calcPct := math.Round((totalUsed/totalCap)*1000) / 10
				lm.DiskPct = &calcPct
				lm.DiskUsedBytes = totalUsed
				lm.DiskTotalBytes = totalCap
				lm.DiskFreeBytes = totalFree
			} else {
				lm.DiskPct = &maxUsage
			}
		}
	}

	return lm
}

// parseHumanBytes converts human readable strings like '15.2 GB', '500 MB', '15G', '2.0 TB' to bytes
func parseHumanBytes(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	re := regexp.MustCompile(`^([0-9.]+)\s*([A-Za-z]+)?$`)
	match := re.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	unit := ""
	if len(match) > 2 {
		unit = strings.ToUpper(strings.TrimSpace(match[2]))
	}
	switch {
	case strings.HasPrefix(unit, "T"):
		return val * 1024 * 1024 * 1024 * 1024
	case strings.HasPrefix(unit, "G"):
		return val * 1024 * 1024 * 1024
	case strings.HasPrefix(unit, "M"):
		return val * 1024 * 1024
	case strings.HasPrefix(unit, "K"):
		return val * 1024
	default:
		return val
	}
}

// parseLoadAverage parses 1m, 5m, 15m load numbers from strings like '0.10, 0.15, 0.20'
func parseLoadAverage(loadStr string) (*float64, *float64, *float64) {
	cleaned := strings.ReplaceAll(loadStr, "/", " ")
	cleaned = strings.ReplaceAll(cleaned, ",", " ")
	fields := strings.Fields(cleaned)
	var l1, l5, l15 *float64
	if len(fields) > 0 {
		if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
			l1 = &v
		}
	}
	if len(fields) > 1 {
		if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
			l5 = &v
		}
	}
	if len(fields) > 2 {
		if v, err := strconv.ParseFloat(fields[2], 64); err == nil {
			l15 = &v
		}
	}
	return l1, l5, l15
}


