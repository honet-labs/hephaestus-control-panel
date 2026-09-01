package services

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type IcmpPingService struct {
	topologyRepo *repository.TopologyRepository
}

type PingResult struct {
	DeviceID  string
	IP        string
	Reachable bool
	LatencyMS *float64
	CheckedAt time.Time
}

func NewIcmpPingService(topologyRepo *repository.TopologyRepository) *IcmpPingService {
	s := &IcmpPingService{
		topologyRepo: topologyRepo,
	}

	wp := queue.GetWorkerPool()
	wp.RegisterHandler("icmp_ping_cycle", s.HandlePingCycleJob)

	return s
}

func (s *IcmpPingService) HandlePingCycleJob(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error {
	devices, err := s.topologyRepo.ListDevices(ctx, nil)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		updateProgress(100, "No topology devices to ping")
		return nil
	}

	updateProgress(10, fmt.Sprintf("Pinging %d devices concurrently...", len(devices)))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // max 20 concurrent pings
	results := make([]domain.DevicePingResult, len(devices))

	for i, dev := range devices {
		wg.Add(1)
		go func(idx int, d domain.TopologyDevice) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			reachable, latency := pingHost(ctx, d.IPAddress)
			results[idx] = domain.DevicePingResult{
				DeviceID:  d.ID,
				IP:        d.IPAddress,
				Reachable: reachable,
				LatencyMS: latency,
				CheckedAt: time.Now(),
			}
		}(i, dev)
	}

	wg.Wait()

	updateProgress(80, "Saving ping results to database...")
	onlineCount := 0
	for _, res := range results {
		_ = s.topologyRepo.SavePingResult(ctx, res)
		if res.Reachable {
			onlineCount++
		}
	}

	updateProgress(100, fmt.Sprintf("Ping cycle completed: %d/%d online", onlineCount, len(devices)))
	logger.Info("ICMP", fmt.Sprintf("Ping cycle completed: %d online, %d offline", onlineCount, len(devices)-onlineCount))
	return nil
}

func pingHost(ctx context.Context, ip string) (bool, *float64) {
	if ip == "" {
		return false, nil
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "2000", ip)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", ip)
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false, nil
	}

	output := out.String()
	// Parse latency (e.g. "time=12.4 ms" or "time<1ms" or "time=12ms")
	re := regexp.MustCompile(`time[=<](\d+\.?\d*)\s*ms`)
	match := re.FindStringSubmatch(output)
	if len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			return true, &val
		}
		zero := float64(0)
		return true, &zero
	}

	if stringsContainsIgnoreCase(output, "ttl=") {
		zero := float64(0)
		return true, &zero
	}

	return false, nil
}

func stringsContainsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && regexp.MustCompile("(?i)"+regexp.QuoteMeta(substr)).MatchString(s)
}
