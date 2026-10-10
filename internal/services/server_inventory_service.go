package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type ServerInventoryService struct {
	repo       *repository.ServerInventoryRepository
	remoteRepo *repository.RemoteHostRepository
	sshService *SSHService
}

func NewServerInventoryService(
	repo *repository.ServerInventoryRepository,
	remoteRepo *repository.RemoteHostRepository,
	sshService *SSHService,
) *ServerInventoryService {
	return &ServerInventoryService{
		repo:       repo,
		remoteRepo: remoteRepo,
		sshService: sshService,
	}
}

func (s *ServerInventoryService) List(ctx context.Context, userID int, userRole string, search string, status string, osType string) ([]domain.ServerInventoryItem, error) {
	return s.repo.List(ctx, userID, userRole, search, status, osType)
}

func (s *ServerInventoryService) GetByID(ctx context.Context, id string) (*domain.ServerInventoryItem, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ServerInventoryService) GetStats(ctx context.Context) (*domain.ServerInventoryStats, error) {
	return s.repo.GetStats(ctx)
}

func (s *ServerInventoryService) Create(ctx context.Context, req domain.CreateServerInventoryRequest, userID int) (*domain.ServerInventoryItem, error) {
	item := domain.ServerInventoryItem{
		RemoteHostID:           cleanOptionalStr(req.RemoteHostID),
		ServerName:             fallbackStr(req.ServerName, "Unnamed Server"),
		IPAddress:              fallbackStr(req.IPAddress, "N/A"),
		OSVersion:              fallbackStr(req.OSVersion, "N/A"),
		OSType:                 fallbackStr(req.OSType, "Linux"),
		ArchitectureType:       fallbackStr(req.ArchitectureType, "N/A"),
		ProcessorModel:         fallbackStr(req.ProcessorModel, "N/A"),
		TotalCore:              fallbackStr(req.TotalCore, "N/A"),
		TotalMemory:            fallbackStr(req.TotalMemory, "N/A"),
		TotalDimmMemory:        fallbackStr(req.TotalDimmMemory, "N/A"),
		TotalStorageSize:       fallbackStr(req.TotalStorageSize, "N/A"),
		TotalDiskCount:         fallbackStr(req.TotalDiskCount, "N/A"),
		TotalNetworkInterfaces: fallbackStr(req.TotalNetworkInterfaces, "N/A"),
		GPUModel:               fallbackStr(req.GPUModel, "N/A"),
		GPUType:                fallbackStr(req.GPUType, "N/A"),
		TotalVRAM:              fallbackStr(req.TotalVRAM, "N/A"),
		Status:                 fallbackStr(req.Status, "active"),
		Notes:                  strings.TrimSpace(req.Notes),
		UserID:                 &userID,
	}

	if err := s.repo.Create(ctx, &item); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, item.ID)
}

func (s *ServerInventoryService) Update(ctx context.Context, id string, req domain.UpdateServerInventoryRequest) (*domain.ServerInventoryItem, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("server inventory item not found: %s", id)
	}

	existing.RemoteHostID = cleanOptionalStr(req.RemoteHostID)
	existing.ServerName = fallbackStr(req.ServerName, existing.ServerName)
	existing.IPAddress = fallbackStr(req.IPAddress, existing.IPAddress)
	existing.OSVersion = fallbackStr(req.OSVersion, existing.OSVersion)
	existing.OSType = fallbackStr(req.OSType, existing.OSType)
	existing.ArchitectureType = fallbackStr(req.ArchitectureType, existing.ArchitectureType)
	existing.ProcessorModel = fallbackStr(req.ProcessorModel, existing.ProcessorModel)
	existing.TotalCore = fallbackStr(req.TotalCore, existing.TotalCore)
	existing.TotalMemory = fallbackStr(req.TotalMemory, existing.TotalMemory)
	existing.TotalDimmMemory = fallbackStr(req.TotalDimmMemory, existing.TotalDimmMemory)
	existing.TotalStorageSize = fallbackStr(req.TotalStorageSize, existing.TotalStorageSize)
	existing.TotalDiskCount = fallbackStr(req.TotalDiskCount, existing.TotalDiskCount)
	existing.TotalNetworkInterfaces = fallbackStr(req.TotalNetworkInterfaces, existing.TotalNetworkInterfaces)
	existing.GPUModel = fallbackStr(req.GPUModel, existing.GPUModel)
	existing.GPUType = fallbackStr(req.GPUType, existing.GPUType)
	existing.TotalVRAM = fallbackStr(req.TotalVRAM, existing.TotalVRAM)
	existing.Status = fallbackStr(req.Status, existing.Status)
	existing.Notes = strings.TrimSpace(req.Notes)

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *ServerInventoryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Linux Probe Script: Returns strict JSON with specifications, defaulting missing values to "N/A"
const linuxDiscoveryBashScript = `
HN=$(hostname 2>/dev/null || cat /etc/hostname 2>/dev/null || echo "N/A")
[ -z "$HN" ] && HN="N/A"

IP=$(ip -4 addr show scope global 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1)
[ -z "$IP" ] && IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$IP" ] && IP="N/A"

OS_VER=$(awk -F'=' '/^PRETTY_NAME/{gsub(/"/, "", $2); print $2}' /etc/os-release 2>/dev/null)
[ -z "$OS_VER" ] && OS_VER=$(uname -r 2>/dev/null || echo "N/A")
OS_TYPE=$(uname -s 2>/dev/null || echo "Linux")
ARCH=$(uname -m 2>/dev/null || echo "N/A")

CPU_MOD=$(awk -F: '/model name/{gsub(/^[ \t]+/, "", $2); print $2; exit}' /proc/cpuinfo 2>/dev/null)
[ -z "$CPU_MOD" ] && CPU_MOD=$(lscpu 2>/dev/null | awk -F: '/Model name:/{gsub(/^[ \t]+/, "", $2); print $2; exit}')
[ -z "$CPU_MOD" ] && CPU_MOD="N/A"

CORES=$(nproc 2>/dev/null || grep -c '^processor' /proc/cpuinfo 2>/dev/null || echo "1")
TOTAL_CORE="$CORES Cores"

TOT_MEM=$(awk '/MemTotal/{printf "%.2f GB", $2/1024/1024}' /proc/meminfo 2>/dev/null)
[ -z "$TOT_MEM" ] && TOT_MEM="N/A"

DIMMS=$(dmidecode -t 17 2>/dev/null | awk '/Size:/{if ($2 !~ /No/ && $2 !~ /Installed/) count++} END {if (count > 0) print count " DIMMs"; else print ""}')
[ -z "$DIMMS" ] && DIMMS="N/A"

TOT_STORAGE=$(lsblk -b -d -n -o SIZE,TYPE 2>/dev/null | awk '$2=="disk"{sum+=$1} END {if (sum>0) {if (sum>=1099511627776) printf "%.2f TB", sum/1099511627776; else printf "%.2f GB", sum/1073741824} else print ""}')
[ -z "$TOT_STORAGE" ] && TOT_STORAGE="N/A"

DISK_LIST=$(lsblk -d -n -o NAME,SIZE,TYPE 2>/dev/null | awk '$3=="disk"{count++; disks=disks (disks?", ":"") $1 " (" $2 ")"} END {if (count>0) print count " Disks [" disks "]"; else print ""}')
[ -z "$DISK_LIST" ] && DISK_LIST="N/A"

NET_LIST=$(ip -br link 2>/dev/null | awk '{count++; iface=iface (iface?", ":"") $1} END {if (count>0) print count " Interfaces [" iface "]"; else print ""}')
[ -z "$NET_LIST" ] && NET_LIST="N/A"

GPU_M=""
GPU_T=""
GPU_V=""
if command -v nvidia-smi >/dev/null 2>&1; then
  GPU_M=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -n1 | tr '\n' ' ' | sed 's/ *$//')
  GPU_V=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader 2>/dev/null | head -n1 | tr '\n' ' ' | sed 's/ *$//')
  [ -n "$GPU_M" ] && GPU_T="Discrete (NVIDIA)"
fi

if [ -z "$GPU_M" ] && command -v lspci >/dev/null 2>&1; then
  PCI_VGA=$(lspci 2>/dev/null | grep -iE 'vga|3d|display' | awk -F: '{print $3}' | sed 's/^[ \t]*//' | head -n1)
  if [ -n "$PCI_VGA" ]; then
    GPU_M="$PCI_VGA"
    GPU_T="Integrated / PCI Display"
  fi
fi

[ -z "$GPU_M" ] && GPU_M="N/A"
[ -z "$GPU_T" ] && GPU_T="N/A"
[ -z "$GPU_V" ] && GPU_V="N/A"

cat <<EOF
{
  "server_name": "$HN",
  "ip_address": "$IP",
  "os_version": "$OS_VER",
  "os_type": "$OS_TYPE",
  "architecture_type": "$ARCH",
  "processor_model": "$CPU_MOD",
  "total_core": "$TOTAL_CORE",
  "total_memory": "$TOT_MEM",
  "total_dimm_memory": "$DIMMS",
  "total_storage_size": "$TOT_STORAGE",
  "total_disk_count": "$DISK_LIST",
  "total_network_interfaces": "$NET_LIST",
  "gpu_model": "$GPU_M",
  "gpu_type": "$GPU_T",
  "total_vram": "$GPU_V"
}
EOF
`

type rawProbePayload struct {
	ServerName             string `json:"server_name"`
	IPAddress              string `json:"ip_address"`
	OSVersion              string `json:"os_version"`
	OSType                 string `json:"os_type"`
	ArchitectureType       string `json:"architecture_type"`
	ProcessorModel         string `json:"processor_model"`
	TotalCore              string `json:"total_core"`
	TotalMemory            string `json:"total_memory"`
	TotalDimmMemory        string `json:"total_dimm_memory"`
	TotalStorageSize       string `json:"total_storage_size"`
	TotalDiskCount         string `json:"total_disk_count"`
	TotalNetworkInterfaces string `json:"total_network_interfaces"`
	GPUModel               string `json:"gpu_model"`
	GPUType                string `json:"gpu_type"`
	TotalVRAM              string `json:"total_vram"`
}

func (s *ServerInventoryService) SyncFromRemoteHost(ctx context.Context, hostID string, userID int) (*domain.ServerInventoryItem, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("remote host configuration not found: %w", err)
	}

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(cfg, linuxDiscoveryBashScript)

	// Extract JSON payload from stdout
	cleanOut := strings.TrimSpace(stdout)
	startIdx := strings.Index(cleanOut, "{")
	endIdx := strings.LastIndex(cleanOut, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, fmt.Errorf("failed executing hardware probe over SSH (exit %d): %v (stderr: %s, stdout: %s)", exitCode, err, stderr, cleanOut)
	}
	jsonStr := cleanOut[startIdx : endIdx+1]

	var raw rawProbePayload
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse probe JSON output: %w (raw: %s)", err, jsonStr)
	}

	now := time.Now()
	srvName := strings.TrimSpace(raw.ServerName)
	if srvName == "" || srvName == "N/A" {
		srvName = cfg.Name
	}
	srvIP := strings.TrimSpace(raw.IPAddress)
	if srvIP == "" || srvIP == "N/A" {
		srvIP = cfg.Host
	}

	item := domain.ServerInventoryItem{
		RemoteHostID:           &hostID,
		ServerName:             srvName,
		IPAddress:              srvIP,
		OSVersion:              fallbackStr(raw.OSVersion, "N/A"),
		OSType:                 fallbackStr(raw.OSType, "Linux"),
		ArchitectureType:       fallbackStr(raw.ArchitectureType, "N/A"),
		ProcessorModel:         fallbackStr(raw.ProcessorModel, "N/A"),
		TotalCore:              fallbackStr(raw.TotalCore, "N/A"),
		TotalMemory:            fallbackStr(raw.TotalMemory, "N/A"),
		TotalDimmMemory:        fallbackStr(raw.TotalDimmMemory, "N/A"),
		TotalStorageSize:       fallbackStr(raw.TotalStorageSize, "N/A"),
		TotalDiskCount:         fallbackStr(raw.TotalDiskCount, "N/A"),
		TotalNetworkInterfaces: fallbackStr(raw.TotalNetworkInterfaces, "N/A"),
		GPUModel:               fallbackStr(raw.GPUModel, "N/A"),
		GPUType:                fallbackStr(raw.GPUType, "N/A"),
		TotalVRAM:              fallbackStr(raw.TotalVRAM, "N/A"),
		Status:                 "active",
		Notes:                  fmt.Sprintf("Auto-discovered via Remote Host %s (%s)", cfg.Name, cfg.Host),
		UserID:                 &userID,
		LastSyncedAt:           &now,
	}

	res, _, err := s.repo.UpsertFromProbe(ctx, &item)
	if err != nil {
		return nil, fmt.Errorf("failed saving discovered inventory: %w", err)
	}

	return res, nil
}

type SyncAllResult struct {
	TotalHosts    int      `json:"totalHosts"`
	SyncedSuccess int      `json:"syncedSuccess"`
	SyncedFailed  int      `json:"syncedFailed"`
	Errors        []string `json:"errors"`
}

func (s *ServerInventoryService) SyncAllRemoteHosts(ctx context.Context, userID int, userRole string) (*SyncAllResult, error) {
	hosts, err := s.remoteRepo.List(ctx, userID, userRole)
	if err != nil {
		return nil, err
	}

	result := &SyncAllResult{
		TotalHosts: len(hosts),
		Errors:     make([]string, 0),
	}

	if len(hosts) == 0 {
		return result, nil
	}

	var mu sync.Mutex
	sem := make(chan struct{}, 4) // max 4 concurrent SSH probes
	var wg sync.WaitGroup

	for _, h := range hosts {
		wg.Add(1)
		go func(host domain.RemoteHostConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			probeCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()

			_, err := s.SyncFromRemoteHost(probeCtx, host.ID, userID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.SyncedFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s (%s): %v", host.Name, host.Host, err))
				logger.Warn("ServerInventory", fmt.Sprintf("Failed syncing host %s: %v", host.Name, err))
			} else {
				result.SyncedSuccess++
			}
		}(h)
	}

	wg.Wait()
	return result, nil
}

// GenerateCSVTemplate returns a ready-to-use CSV template
func (s *ServerInventoryService) GenerateCSVTemplate() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{
		"server_name",
		"ip_address",
		"os_version",
		"os_type",
		"architecture_type",
		"processor_model",
		"total_core",
		"total_memory",
		"total_dimm_memory",
		"total_storage_size",
		"total_disk_count",
		"total_network_interfaces",
		"gpu_model",
		"gpu_type",
		"total_vram",
		"status",
		"notes",
	}
	_ = w.Write(headers)

	sample1 := []string{
		"prod-db-01",
		"192.168.1.10",
		"Ubuntu 22.04.4 LTS",
		"Linux",
		"x86_64",
		"Intel Xeon Gold 6330 @ 2.00GHz",
		"32 Cores",
		"64.00 GB",
		"4 DIMMs",
		"2.00 TB",
		"2 Disks [sda (1T), sdb (1T)]",
		"4 Interfaces [eth0, eth1, docker0, lo]",
		"N/A",
		"N/A",
		"N/A",
		"active",
		"Primary Production Database",
	}

	sample2 := []string{
		"ai-worker-01",
		"192.168.1.25",
		"Rocky Linux 9.3 (Blue Onyx)",
		"Linux",
		"x86_64",
		"AMD EPYC 7763 64-Core Processor",
		"64 Cores",
		"256.00 GB",
		"8 DIMMs",
		"4.00 TB",
		"4 Disks [nvme0n1 (2T), nvme1n1 (2T)]",
		"2 Interfaces [enp1s0f0, lo]",
		"NVIDIA RTX 4090",
		"Discrete (NVIDIA)",
		"24576 MiB",
		"active",
		"Deep Learning Inference Node",
	}

	_ = w.Write(sample1)
	_ = w.Write(sample2)
	w.Flush()
	return buf.Bytes()
}

// ExportCSV streams all server inventory records as CSV
func (s *ServerInventoryService) ExportCSV(ctx context.Context, userID int, userRole string) ([]byte, error) {
	items, err := s.repo.List(ctx, userID, userRole, "", "", "")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{
		"server_name",
		"ip_address",
		"os_version",
		"os_type",
		"architecture_type",
		"processor_model",
		"total_core",
		"total_memory",
		"total_dimm_memory",
		"total_storage_size",
		"total_disk_count",
		"total_network_interfaces",
		"gpu_model",
		"gpu_type",
		"total_vram",
		"status",
		"notes",
	}
	_ = w.Write(headers)

	for _, it := range items {
		row := []string{
			it.ServerName,
			it.IPAddress,
			it.OSVersion,
			it.OSType,
			it.ArchitectureType,
			it.ProcessorModel,
			it.TotalCore,
			it.TotalMemory,
			it.TotalDimmMemory,
			it.TotalStorageSize,
			it.TotalDiskCount,
			it.TotalNetworkInterfaces,
			it.GPUModel,
			it.GPUType,
			it.TotalVRAM,
			it.Status,
			it.Notes,
		}
		_ = w.Write(row)
	}

	w.Flush()
	return buf.Bytes(), nil
}

// ImportCSV parses uploaded CSV and creates/updates inventory records
func (s *ServerInventoryService) ImportCSV(ctx context.Context, r io.Reader, userID int) (*domain.BulkImportServerResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed reading CSV file: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows (expected header + at least 1 row)")
	}

	headerMap := make(map[string]int)
	for i, h := range rows[0] {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	getCol := func(row []string, names ...string) string {
		for _, name := range names {
			if idx, ok := headerMap[name]; ok && idx < len(row) {
				val := strings.TrimSpace(row[idx])
				if val != "" {
					return val
				}
			}
		}
		return ""
	}

	result := &domain.BulkImportServerResult{
		Errors: make([]string, 0),
	}

	for lineIdx, row := range rows[1:] {
		result.TotalProcessed++
		serverName := getCol(row, "server_name", "servername", "hostname", "name")
		ipAddress := getCol(row, "ip_address", "ipaddress", "ip", "host")

		if serverName == "" && ipAddress == "" {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: Missing both server_name and ip_address", lineIdx+2))
			continue
		}

		if serverName == "" {
			serverName = "Server-" + ipAddress
		}
		if ipAddress == "" {
			ipAddress = "N/A"
		}

		item := domain.ServerInventoryItem{
			ServerName:             serverName,
			IPAddress:              ipAddress,
			OSVersion:              fallbackStr(getCol(row, "os_version", "osversion", "os"), "N/A"),
			OSType:                 fallbackStr(getCol(row, "os_type", "ostype"), "Linux"),
			ArchitectureType:       fallbackStr(getCol(row, "architecture_type", "arch", "architecture"), "N/A"),
			ProcessorModel:         fallbackStr(getCol(row, "processor_model", "processormodel", "cpu", "processor"), "N/A"),
			TotalCore:              fallbackStr(getCol(row, "total_core", "totalcore", "cores", "core"), "N/A"),
			TotalMemory:            fallbackStr(getCol(row, "total_memory", "totalmemory", "memory", "ram"), "N/A"),
			TotalDimmMemory:        fallbackStr(getCol(row, "total_dimm_memory", "totaldimmmemory", "dimm", "dimms"), "N/A"),
			TotalStorageSize:       fallbackStr(getCol(row, "total_storage_size", "totalstoragesize", "storage", "disk_size"), "N/A"),
			TotalDiskCount:         fallbackStr(getCol(row, "total_disk_count", "totaldiskcount", "disks", "disk_count"), "N/A"),
			TotalNetworkInterfaces: fallbackStr(getCol(row, "total_network_interfaces", "totalnetworkinterfaces", "interfaces", "network"), "N/A"),
			GPUModel:               fallbackStr(getCol(row, "gpu_model", "gpumodel", "gpu"), "N/A"),
			GPUType:                fallbackStr(getCol(row, "gpu_type", "gputype"), "N/A"),
			TotalVRAM:              fallbackStr(getCol(row, "total_vram", "totalvram", "vram"), "N/A"),
			Status:                 fallbackStr(getCol(row, "status"), "active"),
			Notes:                  getCol(row, "notes", "note", "description"),
			UserID:                 &userID,
		}

		_, created, err := s.repo.UpsertFromProbe(ctx, &item)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d (%s): %v", lineIdx+2, serverName, err))
		} else if created {
			result.CreatedCount++
		} else {
			result.UpdatedCount++
		}
	}

	return result, nil
}

func fallbackStr(val, def string) string {
	t := strings.TrimSpace(val)
	if t == "" {
		return def
	}
	return t
}

func cleanOptionalStr(val *string) *string {
	if val == nil {
		return nil
	}
	t := strings.TrimSpace(*val)
	if t == "" {
		return nil
	}
	return &t
}
