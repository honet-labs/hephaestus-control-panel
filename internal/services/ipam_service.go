package services

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type IpamService struct {
	ipamRepo    *repository.IpamRepository
	workerPool  *queue.WorkerPool
	activeScans sync.Map // map[string]bool: subnetID -> isScanning
	stopChan    chan struct{}
}

func NewIpamService(ipamRepo *repository.IpamRepository, workerPool *queue.WorkerPool) *IpamService {
	return &IpamService{
		ipamRepo:   ipamRepo,
		workerPool: workerPool,
		stopChan:   make(chan struct{}),
	}
}

// StartBackgroundEngine starts the automated scheduler for periodic IPAM scans
func (s *IpamService) StartBackgroundEngine() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		logger.Info("IPAM", "Automated background scan scheduler started (checking every 1m).")

		for {
			select {
			case <-s.stopChan:
				logger.Info("IPAM", "Automated scan scheduler stopped.")
				return
			case <-ticker.C:
				s.triggerDueScans()
			}
		}
	}()
}

// StopBackgroundEngine stops the scheduler
func (s *IpamService) StopBackgroundEngine() {
	close(s.stopChan)
}

// triggerDueScans queries due subnets and fires concurrent scans
func (s *IpamService) triggerDueScans() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dueSubnets, err := s.ipamRepo.GetSubnetsDueForScan(ctx)
	if err != nil {
		logger.Warn("IPAM", fmt.Sprintf("Failed to query due subnets for scanning: %v", err))
		return
	}

	for _, sub := range dueSubnets {
		subnetID := sub.ID
		if _, running := s.activeScans.Load(subnetID); running {
			continue // Already scanning
		}

		logger.Info("IPAM", fmt.Sprintf("Triggering scheduled scan for subnet '%s' (%s) [interval: %s]", sub.Name, sub.CIDR, sub.ScanInterval))
		go func(id string) {
			scanCtx, scanCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer scanCancel()
			if _, err := s.ScanSubnet(scanCtx, id); err != nil {
				logger.Error("IPAM", fmt.Sprintf("Scheduled scan failed for subnet ID %s", id), err)
			}
		}(subnetID)
	}
}

// GenerateSubnetHostIPs generates all usable host IP strings for a given IPv4 CIDR
func GenerateSubnetHostIPs(cidrStr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %w", err)
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("only IPv4 subnets are currently supported")
	}

	mask := ipnet.Mask
	if len(mask) != 4 {
		return nil, fmt.Errorf("invalid IPv4 netmask")
	}

	netUint := binary.BigEndian.Uint32(ipv4.Mask(mask))
	maskUint := binary.BigEndian.Uint32(mask)
	bcastUint := netUint | ^maskUint

	totalHosts := bcastUint - netUint + 1
	if totalHosts > 4096 {
		return nil, fmt.Errorf("subnet size (%d hosts) exceeds maximum scan limit of 4096 hosts (/20)", totalHosts)
	}

	var hosts []string
	if totalHosts <= 2 {
		for u := netUint; u <= bcastUint; u++ {
			hostIP := make(net.IP, 4)
			binary.BigEndian.PutUint32(hostIP, u)
			hosts = append(hosts, hostIP.String())
		}
	} else {
		for u := netUint + 1; u < bcastUint; u++ {
			hostIP := make(net.IP, 4)
			binary.BigEndian.PutUint32(hostIP, u)
			hosts = append(hosts, hostIP.String())
		}
	}

	return hosts, nil
}

func calculateNextScanAt(interval string, from time.Time) *time.Time {
	var d time.Duration
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "6h":
		d = 6 * time.Hour
	case "12h":
		d = 12 * time.Hour
	case "1d", "24h":
		d = 24 * time.Hour
	case "3d", "72h":
		d = 72 * time.Hour
	default:
		return nil
	}
	next := from.Add(d)
	return &next
}

// ListSubnets returns all subnets
func (s *IpamService) ListSubnets(ctx context.Context) ([]domain.IpamSubnet, error) {
	return s.ipamRepo.ListSubnets(ctx)
}

// GetSubnet returns a subnet and its addresses
func (s *IpamService) GetSubnet(ctx context.Context, id string) (*domain.IpamSubnet, []domain.IpamAddress, error) {
	sub, err := s.ipamRepo.GetSubnetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	addresses, err := s.ipamRepo.ListAddressesBySubnet(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	return sub, addresses, nil
}

// CreateSubnet creates and initializes a new subnet
func (s *IpamService) CreateSubnet(ctx context.Context, req *domain.CreateIpamSubnetRequest, userID *int) (*domain.IpamSubnet, error) {
	cleanCIDR := strings.TrimSpace(req.CIDR)
	hostIPs, err := GenerateSubnetHostIPs(cleanCIDR)
	if err != nil {
		return nil, err
	}

	// Check for duplicate CIDR
	if existing, _ := s.ipamRepo.GetSubnetByCIDR(ctx, cleanCIDR); existing != nil {
		return nil, fmt.Errorf("subnet with CIDR '%s' already exists", cleanCIDR)
	}

	totalIPs := len(hostIPs)
	totalUsed := 0
	totalUnused := totalIPs

	now := time.Now()
	nextScan := calculateNextScanAt(req.ScanInterval, now)

	sub := &domain.IpamSubnet{
		ID:             fmt.Sprintf("sub-%s", uuid.New().String()[:8]),
		Name:           strings.TrimSpace(req.Name),
		CIDR:           cleanCIDR,
		Gateway:        strings.TrimSpace(req.Gateway),
		VlanID:         req.VlanID,
		VRF:            req.VRF,
		Description:    strings.TrimSpace(req.Description),
		ScanInterval:   req.ScanInterval,
		LastScannedAt:  nil,
		NextScanAt:     nextScan,
		TotalIPs:       totalIPs,
		TotalUsedIPs:   totalUsed,
		TotalUnusedIPs: totalUnused,
		UserID:         userID,
	}

	if sub.VRF == "" {
		sub.VRF = "Default"
	}
	if sub.ScanInterval == "" {
		sub.ScanInterval = "6h"
	}

	if err := s.ipamRepo.CreateSubnet(ctx, sub); err != nil {
		return nil, err
	}

	// If gateway is supplied, auto-register as reserved
	if sub.Gateway != "" {
		gatewayAddr := &domain.IpamAddress{
			ID:         fmt.Sprintf("ip-%s", uuid.New().String()[:8]),
			SubnetID:   sub.ID,
			IPAddress:  sub.Gateway,
			Status:     "reserved",
			Hostname:   "Default Gateway",
			DeviceType: "Gateway",
			Notes:      "Subnet default gateway",
		}
		_ = s.ipamRepo.UpsertAddress(ctx, gatewayAddr)
		sub.TotalUsedIPs = 1
		sub.TotalUnusedIPs = totalIPs - 1
		_ = s.ipamRepo.UpdateSubnetScanStats(ctx, sub.ID, totalIPs, 1, sub.TotalUnusedIPs, now, nextScan)
	}

	return sub, nil
}

// UpdateSubnet updates subnet details and adjusts next scan time
func (s *IpamService) UpdateSubnet(ctx context.Context, id string, req *domain.UpdateIpamSubnetRequest) (*domain.IpamSubnet, error) {
	sub, err := s.ipamRepo.GetSubnetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sub.Name = strings.TrimSpace(req.Name)
	sub.Gateway = strings.TrimSpace(req.Gateway)
	sub.VlanID = req.VlanID
	sub.VRF = req.VRF
	sub.Description = strings.TrimSpace(req.Description)

	if req.ScanInterval != "" && req.ScanInterval != sub.ScanInterval {
		sub.ScanInterval = req.ScanInterval
		sub.NextScanAt = calculateNextScanAt(req.ScanInterval, time.Now())
	}

	if err := s.ipamRepo.UpdateSubnet(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// DeleteSubnet removes a subnet
func (s *IpamService) DeleteSubnet(ctx context.Context, id string) error {
	return s.ipamRepo.DeleteSubnet(ctx, id)
}

// ScanSubnet runs a high-performance concurrent ping scan on all hosts in the subnet
func (s *IpamService) ScanSubnet(ctx context.Context, subnetID string) (*domain.IpamScanLog, error) {
	if _, loaded := s.activeScans.LoadOrStore(subnetID, true); loaded {
		return nil, fmt.Errorf("a scan is already in progress for this subnet")
	}
	defer s.activeScans.Delete(subnetID)

	sub, err := s.ipamRepo.GetSubnetByID(ctx, subnetID)
	if err != nil {
		return nil, fmt.Errorf("subnet not found: %w", err)
	}

	hostIPs, err := GenerateSubnetHostIPs(sub.CIDR)
	if err != nil {
		return nil, fmt.Errorf("cannot parse subnet CIDR: %w", err)
	}

	startTime := time.Now()
	totalHosts := len(hostIPs)

	logger.Info("IPAM", fmt.Sprintf("Starting scan for subnet '%s' (%s) with %d hosts...", sub.Name, sub.CIDR, totalHosts))

	type scanResult struct {
		ip        string
		reachable bool
		latency   *float64
	}

	resultsChan := make(chan scanResult, totalHosts)
	sem := make(chan struct{}, 30) // 30 concurrent ping workers
	var wg sync.WaitGroup

	for _, ip := range hostIPs {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pingCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			defer cancel()

			reachable, latency := pingHost(pingCtx, target)
			resultsChan <- scanResult{
				ip:        target,
				reachable: reachable,
				latency:   latency,
			}
		}(ip)
	}

	wg.Wait()
	close(resultsChan)

	// Fetch existing addresses map
	existingList, err := s.ipamRepo.ListAddressesBySubnet(ctx, subnetID)
	if err != nil {
		logger.Warn("IPAM", fmt.Sprintf("Error fetching existing addresses for subnet %s: %v", subnetID, err))
	}
	existingMap := make(map[string]domain.IpamAddress)
	for _, a := range existingList {
		existingMap[a.IPAddress] = a
	}

	foundActive := 0
	now := time.Now()

	for res := range resultsChan {
		var latencyMS int
		if res.latency != nil {
			latencyMS = int(*res.latency)
		}

		if res.reachable {
			foundActive++
			if existing, ok := existingMap[res.ip]; ok {
				// Update existing address
				existing.IsOnline = true
				existing.ResponseTimeMS = latencyMS
				existing.LastSeenAt = &now
				_ = s.ipamRepo.UpsertAddress(ctx, &existing)
			} else {
				// Newly discovered host!
				newAddr := &domain.IpamAddress{
					ID:             fmt.Sprintf("ip-%s", uuid.New().String()[:8]),
					SubnetID:       subnetID,
					IPAddress:      res.ip,
					Status:         "discovered",
					Hostname:       "",
					DeviceType:     "Unknown",
					IsOnline:       true,
					ResponseTimeMS: latencyMS,
					LastSeenAt:     &now,
					Notes:          "Auto-discovered by IPAM network scan engine",
				}
				_ = s.ipamRepo.UpsertAddress(ctx, newAddr)
			}
		} else {
			if existing, ok := existingMap[res.ip]; ok {
				existing.IsOnline = false
				_ = s.ipamRepo.UpsertAddress(ctx, &existing)
			}
		}
	}

	// Re-query addresses to compute accurate used and unused numbers
	refreshedList, _ := s.ipamRepo.ListAddressesBySubnet(ctx, subnetID)
	totalUsed := 0
	for _, a := range refreshedList {
		if a.Status == "active" || a.Status == "reserved" || a.IsOnline {
			totalUsed++
		}
	}
	totalUnused := totalHosts - totalUsed
	if totalUnused < 0 {
		totalUnused = 0
	}

	endTime := time.Now()
	durationMS := int(endTime.Sub(startTime).Milliseconds())
	nextScan := calculateNextScanAt(sub.ScanInterval, endTime)

	// Update subnet metrics
	_ = s.ipamRepo.UpdateSubnetScanStats(ctx, subnetID, totalHosts, totalUsed, totalUnused, endTime, nextScan)

	// Save scan log
	scanLog := &domain.IpamScanLog{
		ID:           fmt.Sprintf("slog-%s", uuid.New().String()[:8]),
		SubnetID:     subnetID,
		StartedAt:    startTime,
		FinishedAt:   endTime,
		DurationMS:   durationMS,
		ScannedIPs:   totalHosts,
		FoundActive:  foundActive,
		Status:       "success",
		ErrorMessage: "",
	}
	_ = s.ipamRepo.SaveScanLog(ctx, scanLog)

	logger.Info("IPAM", fmt.Sprintf("Completed scan for '%s' in %dms: %d/%d active hosts found, %d used, %d unused.",
		sub.Name, durationMS, foundActive, totalHosts, totalUsed, totalUnused))

	return scanLog, nil
}

// GetNextAvailableIP finds the first unused host IP in the subnet
func (s *IpamService) GetNextAvailableIP(ctx context.Context, subnetID string) (string, error) {
	sub, err := s.ipamRepo.GetSubnetByID(ctx, subnetID)
	if err != nil {
		return "", err
	}

	hostIPs, err := GenerateSubnetHostIPs(sub.CIDR)
	if err != nil {
		return "", err
	}

	addresses, err := s.ipamRepo.ListAddressesBySubnet(ctx, subnetID)
	if err != nil {
		return "", err
	}

	usedSet := make(map[string]bool)
	for _, a := range addresses {
		// Considered unavailable if allocated (active, reserved, or online)
		if a.Status == "active" || a.Status == "reserved" || a.IsOnline {
			usedSet[a.IPAddress] = true
		}
	}

	for _, ip := range hostIPs {
		if !usedSet[ip] {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IPs found in subnet '%s' (%s)", sub.Name, sub.CIDR)
}

// SaveAddress creates or updates an address allocation manually
func (s *IpamService) SaveAddress(ctx context.Context, req *domain.SaveIpamAddressRequest) (*domain.IpamAddress, error) {
	cleanIP := strings.TrimSpace(req.IPAddress)
	if net.ParseIP(cleanIP) == nil {
		return nil, fmt.Errorf("invalid IP address format: %s", cleanIP)
	}

	addr := &domain.IpamAddress{
		ID:         fmt.Sprintf("ip-%s", uuid.New().String()[:8]),
		SubnetID:   req.SubnetID,
		IPAddress:  cleanIP,
		Status:     req.Status,
		Hostname:   strings.TrimSpace(req.Hostname),
		MACAddress: strings.TrimSpace(req.MACAddress),
		DeviceType: req.DeviceType,
		Notes:      strings.TrimSpace(req.Notes),
	}

	if addr.Status == "" {
		addr.Status = "active"
	}
	if addr.DeviceType == "" {
		addr.DeviceType = "Server"
	}

	if err := s.ipamRepo.UpsertAddress(ctx, addr); err != nil {
		return nil, err
	}

	// Trigger quick background ping to set liveness
	go func(target, subnetID string) {
		pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		reachable, latency := pingHost(pCtx, target)
		if existing, _ := s.ipamRepo.GetAddressByIP(context.Background(), subnetID, target); existing != nil {
			existing.IsOnline = reachable
			if latency != nil {
				existing.ResponseTimeMS = int(*latency)
			}
			now := time.Now()
			if reachable {
				existing.LastSeenAt = &now
			}
			_ = s.ipamRepo.UpsertAddress(context.Background(), existing)
		}
	}(cleanIP, req.SubnetID)

	return addr, nil
}

// UpdateAddress updates metadata of an existing IP
func (s *IpamService) UpdateAddress(ctx context.Context, id string, req *domain.UpdateIpamAddressRequest) (*domain.IpamAddress, error) {
	if err := s.ipamRepo.UpdateAddress(ctx, id, req); err != nil {
		return nil, err
	}
	return s.ipamRepo.GetAddress(ctx, id)
}

// DeleteAddress releases an IP allocation
func (s *IpamService) DeleteAddress(ctx context.Context, id string) error {
	return s.ipamRepo.DeleteAddress(ctx, id)
}

// PingSingleIP checks connectivity to an individual IP
func (s *IpamService) PingSingleIP(ctx context.Context, ip string) (bool, *float64, error) {
	cleanIP := strings.TrimSpace(ip)
	if net.ParseIP(cleanIP) == nil {
		return false, nil, fmt.Errorf("invalid IP address format: %s", cleanIP)
	}

	pCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	reachable, latency := pingHost(pCtx, cleanIP)
	return reachable, latency, nil
}

// ListScanLogs returns recent scan logs
func (s *IpamService) ListScanLogs(ctx context.Context, subnetID string, limit int) ([]domain.IpamScanLog, error) {
	return s.ipamRepo.ListScanLogs(ctx, subnetID, limit)
}

// GetSummaryStats returns global IPAM metrics
func (s *IpamService) GetSummaryStats(ctx context.Context) (*domain.IpamSummaryStats, error) {
	return s.ipamRepo.GetSummaryStats(ctx)
}
