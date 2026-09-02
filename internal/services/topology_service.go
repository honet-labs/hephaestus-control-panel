package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type TopologyService struct {
	topologyRepo *repository.TopologyRepository
	configRepo   *repository.ConfigRepository
	remoteRepo   *repository.RemoteHostRepository
	httpClient   *http.Client
}

func NewTopologyService(topologyRepo *repository.TopologyRepository, configRepo *repository.ConfigRepository, remoteRepo *repository.RemoteHostRepository) *TopologyService {
	return &TopologyService{
		topologyRepo: topologyRepo,
		configRepo:   configRepo,
		remoteRepo:   remoteRepo,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

type GraphData struct {
	Nodes []domain.TopologyDevice `json:"nodes"`
	Edges []domain.TopologyEdge   `json:"edges"`
	Meta  map[string]interface{}  `json:"meta"`
}

func (s *TopologyService) GetGraph(ctx context.Context, sheetID *int) (*GraphData, error) {
	devices, err := s.topologyRepo.ListDevices(ctx, sheetID)
	if err != nil {
		return nil, err
	}

	edges, err := s.topologyRepo.ListEdges(ctx, sheetID)
	if err != nil {
		return nil, err
	}

	meta := map[string]interface{}{
		"totalNodes": len(devices),
		"totalEdges": len(edges),
		"sheetId":    sheetID,
	}

	return &GraphData{
		Nodes: devices,
		Edges: edges,
		Meta:  meta,
	}, nil
}

// DiscoverFromPrometheus fetches targets from Prometheus and converts them to topology nodes
func (s *TopologyService) DiscoverFromPrometheus(ctx context.Context) ([]domain.TopologyDevice, error) {
	promCfg, err := s.configRepo.GetActivePrometheus(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active Prometheus configuration found: %w", err)
	}

	baseURL := strings.TrimSuffix(promCfg.ReloadURL, "/-/reload")
	targetsURL := fmt.Sprintf("%s/api/v1/targets", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", targetsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Prometheus targets: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			ActiveTargets []struct {
				Labels    map[string]string `json:"labels"`
				Health    string            `json:"health"`
				ScrapeURL string            `json:"scrapeUrl"`
			} `json:"activeTargets"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	var discovered []domain.TopologyDevice
	for _, t := range payload.Data.ActiveTargets {
		instance := t.Labels["instance"]
		ip := extractIP(instance)
		if ip == "" {
			ip = extractIP(t.ScrapeURL)
		}
		if ip == "" {
			continue
		}

		name := t.Labels["job"]
		if n, ok := t.Labels["nodename"]; ok {
			name = n
		} else if h, ok := t.Labels["hostname"]; ok {
			name = h
		} else {
			name = instance
		}

		status := "offline"
		if t.Health == "up" {
			status = "online"
		}

		labels := make(map[string]interface{})
		for k, v := range t.Labels {
			labels[k] = v
		}

		dev := domain.TopologyDevice{
			ID:         fmt.Sprintf("prom-%s", ip),
			Name:       name,
			IPAddress:  ip,
			DeviceType: inferDeviceType(t.Labels),
			Status:     status,
			Sources:    []string{"PROM"},
			Labels:     labels,
		}
		discovered = append(discovered, dev)
	}

	logger.Info("Topology", fmt.Sprintf("Discovered %d devices from Prometheus", len(discovered)))
	return discovered, nil
}

// ScanSubnet sweeps a CIDR subnet (e.g. 192.168.1.0/24) using concurrent ICMP pings
func (s *TopologyService) ScanSubnet(ctx context.Context, cidr string) ([]domain.TopologyDevice, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %w", err)
	}

	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
		if len(ips) > 512 {
			break // Safety limit
		}
	}

	// Filter network & broadcast
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	var wg sync.WaitGroup
	var discovered []domain.TopologyDevice
	var mu sync.Mutex
	semaphore := make(chan struct{}, 30)

	for _, ipStr := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			reachable, _ := pingHost(ctx, targetIP)
			if reachable {
				mu.Lock()
				discovered = append(discovered, domain.TopologyDevice{
					ID:         fmt.Sprintf("subnet-%s", targetIP),
					Name:       fmt.Sprintf("Device-%s", targetIP),
					IPAddress:  targetIP,
					DeviceType: "unknown",
					Status:     "online",
					Sources:    []string{"SCAN"},
					Labels:     map[string]interface{}{"subnet": cidr},
				})
				mu.Unlock()
			}
		}(ipStr)
	}

	wg.Wait()
	logger.Info("Topology", fmt.Sprintf("Subnet scan on %s found %d reachable hosts", cidr, len(discovered)))
	return discovered, nil
}

func extractIP(s string) string {
	re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func inferDeviceType(labels map[string]string) string {
	job := strings.ToLower(labels["job"])
	if strings.Contains(job, "node") || strings.Contains(job, "linux") || strings.Contains(job, "server") {
		return "server"
	}
	if strings.Contains(job, "switch") || strings.Contains(job, "cisco") {
		return "switch"
	}
	if strings.Contains(job, "router") || strings.Contains(job, "mikrotik") {
		return "router"
	}
	if strings.Contains(job, "firewall") || strings.Contains(job, "pfsense") || strings.Contains(job, "fortigate") {
		return "firewall"
	}
	return "server"
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// SyncFromRemoteServers imports and synchronizes registered remote hosts into topology devices
func (s *TopologyService) SyncFromRemoteServers(ctx context.Context, sheetID *int) ([]domain.TopologyDevice, error) {
	if s.remoteRepo == nil {
		return nil, fmt.Errorf("remote host repository not configured")
	}

	hosts, err := s.remoteRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote hosts: %w", err)
	}

	existingDevices, _ := s.topologyRepo.ListDevices(ctx, sheetID)
	existingMap := make(map[string]domain.TopologyDevice)
	for _, ed := range existingDevices {
		existingMap[ed.ID] = ed
		if ed.IPAddress != "" {
			existingMap[ed.IPAddress] = ed
		}
	}

	var synced []domain.TopologyDevice
	baseX := 220.0
	baseY := 130.0
	idx := len(existingDevices)

	for _, host := range hosts {
		devID := fmt.Sprintf("remote-%s", host.ID)

		var posX, posY *float64
		if ex, exists := existingMap[devID]; exists {
			posX = ex.X
			posY = ex.Y
		} else if ex, exists := existingMap[host.Host]; exists {
			devID = ex.ID
			posX = ex.X
			posY = ex.Y
		} else {
			x := baseX + float64((idx%4)*200)
			y := baseY + float64((idx/4)*160)
			posX = &x
			posY = &y
			idx++
		}

		labels := map[string]interface{}{
			"remoteHostId": host.ID,
			"port":         host.Port,
			"username":     host.Username,
			"groupName":    host.GroupName,
			"tags":         host.Tags,
			"authType":     host.AuthType,
		}

		dev := domain.TopologyDevice{
			ID:         devID,
			Name:       host.Name,
			IPAddress:  host.Host,
			DeviceType: "server",
			Status:     "online",
			Sources:    []string{"REMOTE", "SSH"},
			Labels:     labels,
			SheetID:    sheetID,
			X:          posX,
			Y:          posY,
		}

		if err := s.topologyRepo.SaveDevice(ctx, dev); err != nil {
			logger.Warn("Topology", fmt.Sprintf("Failed to save synced remote server %s: %v", host.Name, err))
			continue
		}

		synced = append(synced, dev)
	}

	logger.Info("Topology", fmt.Sprintf("Synced %d remote server(s) into topology sheet", len(synced)))
	return synced, nil
}

