package services

import (
	"context"
	"fmt"
	"math"
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

	cmd := `nproc 2>/dev/null || echo 1; echo "===CPU==="; (top -bn1 2>/dev/null | grep -i "%Cpu" | head -1) || echo ""; echo "===SYS==="; (cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d= -f2- | tr -d '"') || uname -s; uname -r; uname -m; hostname; (uptime -p 2>/dev/null || uptime); echo "===LOAD==="; uptime 2>/dev/null; echo "===MEM==="; free -m 2>/dev/null; echo "===DF==="; df -hP -x tmpfs -x devtmpfs -x squashfs 2>/dev/null || df -hP 2>/dev/null`
	stdout, _, _, err := s.sshService.ExecuteCommand(cfg, cmd)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return map[string]interface{}{
			"hostname":     cfg.Name,
			"ip":           cfg.Host,
			"osName":       "Linux",
			"kernel":       "-",
			"arch":         "-",
			"uptime":       "-",
			"cpuUsage":     0.0,
			"cpuCores":     1,
			"memPercent":   0.0,
			"memUsed":      "0 B",
			"memTotal":     "0 B",
			"memFree":      "0 B",
			"memAvailable": "0 B",
			"swapUsed":     "0 B",
			"swapTotal":    "0 B",
			"swapPercent":  0.0,
			"loadAverage":  "0.00 / 0.00 / 0.00",
			"disksCount":   0,
			"disks":        []map[string]interface{}{},
		}, nil
	}

	cores := 1
	cpuUsage := 0.0
	osName := "Linux"
	kernel := "-"
	arch := "-"
	uptimeStr := "-"
	loadAvg := "0.00 / 0.00 / 0.00"
	memPercent := 0.0
	memUsedStr := "0 MB"
	memTotalStr := "0 MB"
	memFreeStr := "0 MB"
	memAvailStr := "0 MB"
	swapUsedStr := "0 MB"
	swapTotalStr := "0 MB"
	swapPercent := 0.0
	var disks []map[string]interface{}

	reIdle := regexp.MustCompile(`([0-9.]+)\s*(?:%?\s*id|id)`)

	sections := strings.Split(stdout, "===CPU===")
	if len(sections) > 0 {
		coresLine := strings.TrimSpace(sections[0])
		if c, err := strconv.Atoi(coresLine); err == nil && c > 0 {
			cores = c
		}
	}

	if len(sections) > 1 {
		pSys := strings.Split(sections[1], "===SYS===")
		cpuLine := strings.TrimSpace(pSys[0])
		if match := reIdle.FindStringSubmatch(cpuLine); len(match) > 1 {
			if idleVal, err := strconv.ParseFloat(match[1], 64); err == nil {
				cpuUsage = 100.0 - idleVal
				if cpuUsage < 0 {
					cpuUsage = 0
				}
				if cpuUsage > 100 {
					cpuUsage = 100
				}
			}
		}

		if len(pSys) > 1 {
			pLoad := strings.Split(pSys[1], "===LOAD===")
			sysLines := strings.Split(strings.TrimSpace(pLoad[0]), "\n")
			if len(sysLines) > 0 && strings.TrimSpace(sysLines[0]) != "" {
				osName = strings.TrimSpace(sysLines[0])
			}
			if len(sysLines) > 1 && strings.TrimSpace(sysLines[1]) != "" {
				kernel = strings.TrimSpace(sysLines[1])
			}
			if len(sysLines) > 2 && strings.TrimSpace(sysLines[2]) != "" {
				arch = strings.TrimSpace(sysLines[2])
			}
			if len(sysLines) > 4 && strings.TrimSpace(sysLines[4]) != "" {
				uptimeStr = strings.TrimSpace(sysLines[4])
			}

			if len(pLoad) > 1 {
				pMem := strings.Split(pLoad[1], "===MEM===")
				loadLine := strings.TrimSpace(pMem[0])
				if idx := strings.Index(loadLine, "load average:"); idx != -1 {
					loadAvg = strings.TrimSpace(loadLine[idx+len("load average:"):])
				}

				if len(pMem) > 1 {
					pDf := strings.Split(pMem[1], "===DF===")
					memLines := strings.Split(strings.TrimSpace(pDf[0]), "\n")
					for _, line := range memLines {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "Mem:") {
							fields := strings.Fields(line)
							if len(fields) >= 4 {
								total, _ := strconv.ParseFloat(fields[1], 64)
								used, _ := strconv.ParseFloat(fields[2], 64)
								free, _ := strconv.ParseFloat(fields[3], 64)
								var avail float64
								if len(fields) >= 7 {
									avail, _ = strconv.ParseFloat(fields[6], 64)
								} else {
									avail = free
								}
								if total > 0 {
									memPercent = (used / total) * 100
									if total >= 1024 {
										memTotalStr = fmt.Sprintf("%.1f GB", total/1024)
										memUsedStr = fmt.Sprintf("%.1f GB", used/1024)
										memFreeStr = fmt.Sprintf("%.1f GB", free/1024)
										memAvailStr = fmt.Sprintf("%.1f GB", avail/1024)
									} else {
										memTotalStr = fmt.Sprintf("%.0f MB", total)
										memUsedStr = fmt.Sprintf("%.0f MB", used)
										memFreeStr = fmt.Sprintf("%.0f MB", free)
										memAvailStr = fmt.Sprintf("%.0f MB", avail)
									}
								}
							}
						} else if strings.HasPrefix(line, "Swap:") {
							fields := strings.Fields(line)
							if len(fields) >= 4 {
								total, _ := strconv.ParseFloat(fields[1], 64)
								used, _ := strconv.ParseFloat(fields[2], 64)
								if total > 0 {
									swapPercent = (used / total) * 100
									if total >= 1024 {
										swapTotalStr = fmt.Sprintf("%.1f GB", total/1024)
										swapUsedStr = fmt.Sprintf("%.1f GB", used/1024)
									} else {
										swapTotalStr = fmt.Sprintf("%.0f MB", total)
										swapUsedStr = fmt.Sprintf("%.0f MB", used)
									}
								}
							}
						}
					}

					if len(pDf) > 1 {
						dfLines := strings.Split(strings.TrimSpace(pDf[1]), "\n")
						for _, line := range dfLines {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, "Filesystem") || line == "" {
								continue
							}
							fields := strings.Fields(line)
							if len(fields) >= 6 {
								pctStr := strings.TrimSuffix(fields[4], "%")
								pct, _ := strconv.Atoi(pctStr)
								disks = append(disks, map[string]interface{}{
									"filesystem": fields[0],
									"total":      fields[1],
									"used":       fields[2],
									"avail":      fields[3],
									"percent":    pct,
									"mount":      fields[5],
								})
							}
						}
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"hostname":     cfg.Name,
		"ip":           cfg.Host,
		"osName":       osName,
		"kernel":       kernel,
		"arch":         arch,
		"uptime":       uptimeStr,
		"cpuUsage":     math.Round(cpuUsage*10) / 10,
		"cpuCores":     cores,
		"memPercent":   math.Round(memPercent*10) / 10,
		"memUsed":      memUsedStr,
		"memTotal":     memTotalStr,
		"memFree":      memFreeStr,
		"memAvailable": memAvailStr,
		"swapUsed":     swapUsedStr,
		"swapTotal":    swapTotalStr,
		"swapPercent":  math.Round(swapPercent*10) / 10,
		"loadAverage":  loadAvg,
		"disksCount":   len(disks),
		"disks":        disks,
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

var knownServicesByPort = map[string]string{
	"21":    "ftp",
	"22":    "sshd",
	"23":    "telnet",
	"25":    "smtp",
	"53":    "systemd-resolved",
	"67":    "dhcp-server",
	"68":    "systemd-networkd",
	"80":    "http",
	"110":   "pop3",
	"123":   "chrony/ntp",
	"143":   "imap",
	"443":   "https",
	"465":   "smtps",
	"993":   "imaps",
	"995":   "pop3s",
	"1194":  "openvpn",
	"1433":  "mssql",
	"2049":  "nfs",
	"2375":  "docker",
	"2376":  "docker-tls",
	"2918":  "poseidon-agent",
	"3000":  "grafana",
	"3306":  "mysql/mariadb",
	"5000":  "docker-registry",
	"5432":  "postgresql",
	"5601":  "kibana",
	"6379":  "redis-server",
	"8000":  "http-alt",
	"8080":  "http-proxy",
	"8443":  "https-alt",
	"8888":  "jupyter/hcp-panel",
	"8889":  "hcp-proxy",
	"9000":  "php-fpm/minio",
	"9090":  "prometheus",
	"9092":  "kafka",
	"9100":  "node_exporter",
	"9200":  "opensearch/es",
	"9300":  "opensearch-cluster",
	"9411":  "zipkin",
	"10000": "webmin",
	"24998": "wireguard/vpn",
}

func (s *VpsService) GetNetworkInfo(ctx context.Context, hostID string) (map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	var sudoPrefix string
	if cfg.Password != nil && *cfg.Password != "" {
		escaped := strings.ReplaceAll(*cfg.Password, "'", "'\\''")
		sudoPrefix = fmt.Sprintf("(echo '%s' | sudo -S -p '' 2>/dev/null || sudo -n ) ", escaped)
	} else {
		sudoPrefix = "sudo -n "
	}

	cmd := fmt.Sprintf(`ip -o addr 2>/dev/null; echo "===LINK==="; ip -o link 2>/dev/null; echo "===PORTS==="; (%[1]sss -tulnp 2>/dev/null || ss -tulnp 2>/dev/null || %[1]snetstat -tulnp 2>/dev/null || netstat -tulnp 2>/dev/null); echo "===LSOF==="; (%[1]slsof -iTCP -iUDP -sTCP:LISTEN -P -n 2>/dev/null || lsof -iTCP -iUDP -sTCP:LISTEN -P -n 2>/dev/null | head -n 50); echo "===CONNS==="; (%[1]sss -tunp 2>/dev/null || ss -tunp 2>/dev/null | head -n 45)`, sudoPrefix)

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

	var linkPart, portsPart, lsofPart, connsPart string
	if len(parts) > 1 {
		p2 := strings.Split(parts[1], "===PORTS===")
		linkPart = p2[0]
		if len(p2) > 1 {
			p3 := strings.Split(p2[1], "===LSOF===")
			portsPart = p3[0]
			if len(p3) > 1 {
				p4 := strings.Split(p3[1], "===CONNS===")
				lsofPart = p4[0]
				if len(p4) > 1 {
					connsPart = p4[1]
				}
			} else {
				p4 := strings.Split(p2[1], "===CONNS===")
				portsPart = p4[0]
				if len(p4) > 1 {
					connsPart = p4[1]
				}
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

	// 1b. Parse lsof output for port -> process and pid correlation
	lsofProcMap := make(map[string]string)
	lsofPidMap := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(lsofPart), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMAND") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		cmdName := fields[0]
		pidVal := fields[1]
		for _, f := range fields[2:] {
			if strings.Contains(f, ":") {
				lastColon := strings.LastIndex(f, ":")
				if lastColon != -1 {
					p := f[lastColon+1:]
					p = strings.TrimSuffix(p, ")")
					p = strings.TrimPrefix(p, "->")
					if _, err := strconv.Atoi(p); err == nil {
						lsofProcMap[p] = cmdName
						lsofPidMap[p] = pidVal
					}
				}
			}
		}
	}

	// 2. Parse Listening Ports
	var listeningPorts []map[string]interface{}
	rePID := regexp.MustCompile(`(?:pid=|/)(\d+)`)
	reProc := regexp.MustCompile(`users:\(\("?([^",\)]+)"?`)
	reProcFallback := regexp.MustCompile(`"([^"]+)"`)

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
		} else if match := reProcFallback.FindStringSubmatch(line); len(match) > 1 {
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

		// Correlate with lsof if process or pid is unknown
		if proc, exists := lsofProcMap[portStr]; exists && (procName == "-" || procName == "") {
			procName = proc
		}
		if pid, exists := lsofPidMap[portStr]; exists && (pidStr == "-" || pidStr == "") {
			pidStr = pid
		}

		// Fallback to known services map
		if procName == "-" || procName == "" {
			if known, exists := knownServicesByPort[portStr]; exists {
				procName = known
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
		} else if match := reProcFallback.FindStringSubmatch(line); len(match) > 1 {
			procName = match[1]
		}

		if proc, exists := lsofProcMap[portStr]; exists && (procName == "-" || procName == "") {
			procName = proc
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
