package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go-hephaestus/internal/repository"
)

type VpsService struct {
	remoteRepo *repository.RemoteHostRepository
	sshService *SSHService
}

func NewVpsService(remoteRepo *repository.RemoteHostRepository, sshService *SSHService) *VpsService {
	return &VpsService{
		remoteRepo: remoteRepo,
		sshService: sshService,
	}
}

func (s *VpsService) GetMetrics(ctx context.Context, hostID string) (map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	cmd := `nproc 2>/dev/null || echo 1; uptime 2>/dev/null; free -m 2>/dev/null; df -hP / /boot /boot/efi 2>/dev/null`
	stdout, _, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return map[string]interface{}{
			"hostname":    cfg.Name,
			"ip":          cfg.Host,
			"cpuUsage":    0.0,
			"cpuCores":    1,
			"memPercent":  0.0,
			"memUsed":     "0 B",
			"memTotal":    "0 B",
			"loadAverage": "0.00 / 0.00 / 0.00",
			"disksCount":  0,
			"disks":       []map[string]interface{}{},
		}, nil
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	cores := 1
	if len(lines) > 0 {
		if c, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil && c > 0 {
			cores = c
		}
	}

	loadAvg := "0.00 / 0.00 / 0.00"
	memPercent := 0.0
	memUsedStr := "0 MB"
	memTotalStr := "0 MB"
	var disks []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "load average:") {
			parts := strings.Split(line, "load average:")
			if len(parts) > 1 {
				loadAvg = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total, _ := strconv.ParseFloat(fields[1], 64)
				used, _ := strconv.ParseFloat(fields[2], 64)
				if total > 0 {
					memPercent = (used / total) * 100
					memUsedStr = fmt.Sprintf("%.1f GB", used/1024)
					memTotalStr = fmt.Sprintf("%.1f GB", total/1024)
				}
			}
		} else if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "udev") || strings.HasPrefix(line, "tmpfs") {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				pctStr := strings.TrimSuffix(fields[4], "%")
				pct, _ := strconv.Atoi(pctStr)
				disks = append(disks, map[string]interface{}{
					"mount":   fields[5],
					"total":   fields[1],
					"used":    fields[2],
					"avail":   fields[3],
					"percent": pct,
				})
			}
		}
	}

	return map[string]interface{}{
		"hostname":    cfg.Name,
		"ip":          cfg.Host,
		"cpuUsage":    0.0,
		"cpuCores":    cores,
		"memPercent":  memPercent,
		"memUsed":     memUsedStr,
		"memTotal":    memTotalStr,
		"loadAverage": loadAvg,
		"disksCount":  len(disks),
		"disks":       disks,
	}, nil
}

func (s *VpsService) GetProcesses(ctx context.Context, hostID string) ([]map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	cmd := `ps -eo pid,user,%cpu,%mem,rss,args --sort=-%cpu | head -n 35`
	stdout, _, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return []map[string]interface{}{}, nil
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) <= 1 {
		return []map[string]interface{}{}, nil
	}

	var procs []map[string]interface{}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		cpuVal, _ := strconv.ParseFloat(fields[2], 64)
		memVal, _ := strconv.ParseFloat(fields[3], 64)
		rssKB, _ := strconv.Atoi(fields[4])

		var rssStr string
		if rssKB > 1024*1024 {
			rssStr = fmt.Sprintf("%.1f GB", float64(rssKB)/(1024*1024))
		} else if rssKB > 1024 {
			rssStr = fmt.Sprintf("%.1f MB", float64(rssKB)/1024)
		} else {
			rssStr = fmt.Sprintf("%d KB", rssKB)
		}

		command := strings.Join(fields[5:], " ")
		procs = append(procs, map[string]interface{}{
			"pid":     pid,
			"user":    fields[1],
			"cpu":     cpuVal,
			"mem":     memVal,
			"rss":     rssStr,
			"command": command,
		})
	}

	return procs, nil
}

func (s *VpsService) KillProcess(ctx context.Context, hostID string, pid int) error {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("kill -9 %d", pid)
	_, _, _, err = s.sshService.ExecuteCommand(cfg, cmd)
	return err
}

func (s *VpsService) GetServices(ctx context.Context, hostID string) ([]map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	cmd := `systemctl list-unit-files --type=service --no-pager --no-legend 2>/dev/null | head -n 40`
	stdout, _, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return []map[string]interface{}{}, nil
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var svcs []map[string]interface{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ".service")
		status := "INACTIVE"
		if fields[1] == "enabled" || fields[1] == "running" || fields[1] == "active" {
			status = "ACTIVE"
		} else if fields[1] == "failed" {
			status = "FAILED"
		}

		svcs = append(svcs, map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("System service %s", name),
			"status":      status,
		})
	}

	return svcs, nil
}

func (s *VpsService) ControlService(ctx context.Context, hostID, serviceName, action string) (string, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return "", err
	}

	var cmd string
	switch action {
	case "status":
		cmd = fmt.Sprintf("systemctl status '%s' --no-pager", serviceName)
	case "start", "stop", "restart", "reload":
		cmd = fmt.Sprintf("systemctl %s '%s'", action, serviceName)
	default:
		return "", fmt.Errorf("invalid service action: %s", action)
	}

	stdout, stderr, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil && stdout == "" {
		return stderr, err
	}
	return stdout + stderr, nil
}
