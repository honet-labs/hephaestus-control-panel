package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
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

func (s *VpsService) GetNetworkInfo(ctx context.Context, hostID string) (map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	cmd := `ip -o addr 2>/dev/null; echo "===LINK==="; ip -o link 2>/dev/null; echo "===PORTS==="; ss -tulnp 2>/dev/null || netstat -tulnp 2>/dev/null; echo "===CONNS==="; ss -tunp 2>/dev/null | head -n 45`
	stdout, _, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return map[string]interface{}{
			"interfaces": []map[string]interface{}{
				{
					"name":  "eth0",
					"ipv4":  cfg.Host,
					"ipv6":  "-",
					"mac":   "-",
					"state": "UP",
					"mtu":   1500,
					"rx":    "-",
					"tx":    "-",
				},
			},
			"listeningPorts": []map[string]interface{}{
				{
					"proto":     "TCP",
					"localAddr": "0.0.0.0",
					"port":      strconv.Itoa(cfg.Port),
					"state":     "LISTEN",
					"process":   "sshd",
					"pid":       "-",
				},
			},
			"connections": []map[string]interface{}{},
		}, nil
	}

	return parseNetworkOutput(stdout, cfg), nil
}

func parseNetworkOutput(output string, cfg *domain.RemoteHostConfig) map[string]interface{} {
	parts := strings.Split(output, "===LINK===")
	addrPart := parts[0]

	var linkPart, portsPart, connsPart string
	if len(parts) > 1 {
		p2 := strings.Split(parts[1], "===PORTS===")
		linkPart = p2[0]
		if len(p2) > 1 {
			p3 := strings.Split(p2[1], "===CONNS===")
			portsPart = p3[0]
			if len(p3) > 1 {
				connsPart = p3[1]
			}
		}
	}

	// 1. Parse Interfaces
	interfacesMap := make(map[string]map[string]interface{})

	for _, line := range strings.Split(strings.TrimSpace(addrPart), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		ifaceName := fields[1]
		if _, exists := interfacesMap[ifaceName]; !exists {
			interfacesMap[ifaceName] = map[string]interface{}{
				"name":  ifaceName,
				"ipv4":  "-",
				"ipv6":  "-",
				"mac":   "-",
				"state": "UNKNOWN",
				"mtu":   1500,
				"rx":    "-",
				"tx":    "-",
			}
		}
		family := fields[2]
		ipAddr := fields[3]
		if family == "inet" {
			interfacesMap[ifaceName]["ipv4"] = ipAddr
		} else if family == "inet6" && interfacesMap[ifaceName]["ipv6"] == "-" {
			interfacesMap[ifaceName]["ipv6"] = ipAddr
		}
	}

	for _, line := range strings.Split(strings.TrimSpace(linkPart), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ifaceName := strings.TrimSuffix(fields[1], ":")
		if _, exists := interfacesMap[ifaceName]; !exists {
			interfacesMap[ifaceName] = map[string]interface{}{
				"name":  ifaceName,
				"ipv4":  "-",
				"ipv6":  "-",
				"mac":   "-",
				"state": "UNKNOWN",
				"mtu":   1500,
				"rx":    "-",
				"tx":    "-",
			}
		}

		if strings.Contains(line, "state UP") || strings.Contains(line, "<UP") || strings.Contains(line, ",UP,") {
			interfacesMap[ifaceName]["state"] = "UP"
		} else if strings.Contains(line, "state DOWN") {
			interfacesMap[ifaceName]["state"] = "DOWN"
		}

		if idx := strings.Index(line, "mtu "); idx != -1 {
			sub := line[idx+4:]
			if f := strings.Fields(sub); len(f) > 0 {
				if mtu, err := strconv.Atoi(f[0]); err == nil {
					interfacesMap[ifaceName]["mtu"] = mtu
				}
			}
		}

		if idx := strings.Index(line, "link/ether "); idx != -1 {
			sub := line[idx+11:]
			if f := strings.Fields(sub); len(f) > 0 {
				interfacesMap[ifaceName]["mac"] = f[0]
			}
		}
	}

	var ifaceList []map[string]interface{}
	for _, iface := range interfacesMap {
		ifaceList = append(ifaceList, iface)
	}
	if len(ifaceList) == 0 {
		ifaceList = append(ifaceList, map[string]interface{}{
			"name":  "eth0",
			"ipv4":  cfg.Host,
			"ipv6":  "-",
			"mac":   "-",
			"state": "UP",
			"mtu":   1500,
			"rx":    "-",
			"tx":    "-",
		})
	}

	// 2. Parse Listening Ports
	var listeningPorts []map[string]interface{}
	rePID := regexp.MustCompile(`(?:pid=|/)(\d+)`)
	reProc := regexp.MustCompile(`"([^"]+)"`)

	for _, line := range strings.Split(strings.TrimSpace(portsPart), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		protoLower := strings.ToLower(fields[0])
		if !strings.HasPrefix(protoLower, "tcp") && !strings.HasPrefix(protoLower, "udp") {
			continue
		}

		proto := strings.ToUpper(fields[0])
		state := "LISTEN"
		localAddr := ""
		portStr := ""
		procName := "-"
		pidStr := "-"

		for _, f := range fields {
			if f == "LISTEN" || f == "UNCONN" {
				state = f
			}
		}

		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ":") && !strings.Contains(fields[i], "users:") {
				addrFull := fields[i]
				lastColon := strings.LastIndex(addrFull, ":")
				if lastColon != -1 {
					localAddr = addrFull[:lastColon]
					portStr = addrFull[lastColon+1:]
					break
				}
			}
		}

		if portStr == "" {
			continue
		}

		if match := rePID.FindStringSubmatch(line); len(match) > 1 {
			pidStr = match[1]
		}
		if match := reProc.FindStringSubmatch(line); len(match) > 1 {
			procName = match[1]
		} else {
			for _, f := range fields {
				if strings.Contains(f, "/") {
					pParts := strings.Split(f, "/")
					if len(pParts) == 2 && pParts[1] != "" {
						pidStr = pParts[0]
						procName = pParts[1]
					}
				}
			}
		}

		listeningPorts = append(listeningPorts, map[string]interface{}{
			"proto":     proto,
			"localAddr": localAddr,
			"port":      portStr,
			"state":     state,
			"process":   procName,
			"pid":       pidStr,
		})
	}

	// 3. Parse Active Connections
	var conns []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(connsPart), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		protoLower := strings.ToLower(fields[0])
		if !strings.HasPrefix(protoLower, "tcp") && !strings.HasPrefix(protoLower, "udp") {
			continue
		}

		proto := strings.ToUpper(fields[0])
		state := fields[1]
		localAddr := ""
		remoteAddr := ""
		procName := "-"
		pidStr := "-"

		if len(fields) >= 5 {
			localAddr = fields[3]
			remoteAddr = fields[4]
		}

		if match := rePID.FindStringSubmatch(line); len(match) > 1 {
			pidStr = match[1]
		}
		if match := reProc.FindStringSubmatch(line); len(match) > 1 {
			procName = match[1]
		}

		if localAddr != "" && remoteAddr != "" {
			conns = append(conns, map[string]interface{}{
				"proto":      proto,
				"localAddr":  localAddr,
				"remoteAddr": remoteAddr,
				"state":      state,
				"process":    procName,
				"pid":        pidStr,
			})
		}
	}

	return map[string]interface{}{
		"interfaces":     ifaceList,
		"listeningPorts": listeningPorts,
		"connections":    conns,
	}
}
