package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

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
	var userPtr *int
	if userID > 0 {
		userPtr = &userID
	}

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
		UserID:                 userPtr,
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

clean_val() {
  printf '%s' "$1" | tr -d '"\\\r\n' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

HN=$(clean_val "$HN")
IP=$(clean_val "$IP")
OS_VER=$(clean_val "$OS_VER")
OS_TYPE=$(clean_val "$OS_TYPE")
ARCH=$(clean_val "$ARCH")
CPU_MOD=$(clean_val "$CPU_MOD")
TOTAL_CORE=$(clean_val "$TOTAL_CORE")
TOT_MEM=$(clean_val "$TOT_MEM")
DIMMS=$(clean_val "$DIMMS")
TOT_STORAGE=$(clean_val "$TOT_STORAGE")
DISK_LIST=$(clean_val "$DISK_LIST")
NET_LIST=$(clean_val "$NET_LIST")
GPU_M=$(clean_val "$GPU_M")
GPU_T=$(clean_val "$GPU_T")
GPU_V=$(clean_val "$GPU_V")

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

// Windows Discovery PowerShell Script: Gathers CPU, cores, memory DIMMs, storage drives, network interfaces, and GPU specifications
const windowsDiscoveryPowerShellScript = `
$ErrorActionPreference = 'SilentlyContinue'
$os = Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue
if (-not $os) { $os = Get-WmiObject Win32_OperatingSystem -ErrorAction SilentlyContinue }
$cs = Get-CimInstance Win32_ComputerSystem -ErrorAction SilentlyContinue
if (-not $cs) { $cs = Get-WmiObject Win32_ComputerSystem -ErrorAction SilentlyContinue }
$cpu = Get-CimInstance Win32_Processor -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $cpu) { $cpu = Get-WmiObject Win32_Processor -ErrorAction SilentlyContinue | Select-Object -First 1 }

$coresSum = (Get-CimInstance Win32_Processor -ErrorAction SilentlyContinue | Measure-Object -Property NumberOfCores -Sum).Sum
$cores = if ($coresSum) { $coresSum } elseif ($cpu.NumberOfCores) { $cpu.NumberOfCores } else { 1 }

$totMemBytes = if ($cs.TotalPhysicalMemory) { [int64]$cs.TotalPhysicalMemory } elseif ($os.TotalVisibleMemorySize) { [int64]$os.TotalVisibleMemorySize * 1024 } else { 0 }
$totMemGB = if ($totMemBytes -gt 0) { [math]::Round($totMemBytes / 1GB, 2) } else { 0 }

$dimmCount = (Get-CimInstance Win32_PhysicalMemory -ErrorAction SilentlyContinue | Measure-Object).Count
$dimmStr = if ($dimmCount -gt 0) { "$dimmCount DIMMs" } else { "N/A" }

$disks = @(Get-CimInstance Win32_DiskDrive -ErrorAction SilentlyContinue)
if ($disks.Count -eq 0) { $disks = @(Get-WmiObject Win32_DiskDrive -ErrorAction SilentlyContinue) }
$diskCount = $disks.Count
$totDiskBytes = ($disks | Measure-Object -Property Size -Sum).Sum
$totDiskStr = if ($totDiskBytes -gt 1TB) { "$([math]::Round($totDiskBytes / 1TB, 2)) TB" } elseif ($totDiskBytes -gt 0) { "$([math]::Round($totDiskBytes / 1GB, 2)) GB" } else { "N/A" }

$diskParts = @()
foreach ($d in $disks) {
    $sz = if ($d.Size -gt 1TB) { "$([math]::Round($d.Size / 1TB, 1))T" } elseif ($d.Size -gt 0) { "$([math]::Round($d.Size / 1GB, 1))G" } else { "?" }
    $nm = if ($d.DeviceID) { $d.DeviceID -replace '\\\\\.\\','' } else { "Disk" }
    $diskParts += "$nm ($sz)"
}
$diskListStr = if ($diskCount -gt 0) { "$diskCount Disks [" + ($diskParts -join ", ") + "]" } else { "N/A" }

$nics = @(Get-CimInstance Win32_NetworkAdapter -Filter "NetConnectionStatus=2" -ErrorAction SilentlyContinue)
if ($nics.Count -eq 0) {
    $nics = @(Get-CimInstance Win32_NetworkAdapter -Filter "PhysicalAdapter=True" -ErrorAction SilentlyContinue)
}
$nicNames = @($nics | ForEach-Object { $_.NetConnectionID } | Where-Object { $_ })
$nicListStr = if ($nicNames.Count -gt 0) { "$($nicNames.Count) Interfaces [" + ($nicNames -join ", ") + "]" } else { "N/A" }

$gpus = @(Get-CimInstance Win32_VideoController -ErrorAction SilentlyContinue)
$gpu = if ($gpus.Count -gt 0) { $gpus[0] } else { $null }
$gpuName = if ($gpu -and $gpu.Name) { $gpu.Name.Trim() } else { "N/A" }
$gpuType = if ($gpuName -ne "N/A") {
    if ($gpuName -match "NVIDIA|GeForce|RTX|Quadro|Tesla") { "Discrete (NVIDIA)" }
    elseif ($gpuName -match "AMD|Radeon") { "Discrete (AMD)" }
    elseif ($gpuName -match "Intel") { "Integrated (Intel)" }
    else { "Standard / Virtual Display" }
} else { "N/A" }
$gpuVram = if ($gpu -and $gpu.AdapterRAM -and $gpu.AdapterRAM -gt 0) { "$([math]::Round([int64]$gpu.AdapterRAM / 1MB, 0)) MB" } else { "N/A" }

$hn = if ($os.CSName) { $os.CSName } elseif ($env:COMPUTERNAME) { $env:COMPUTERNAME } else { "N/A" }
$ipObj = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin Dhcp,Manual -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } | Select-Object -First 1
$ip = if ($ipObj) { $ipObj.IPAddress } else { "N/A" }

$arch = if ($os.OSArchitecture -match "64") { "x86_64" } elseif ($os.OSArchitecture -match "ARM64") { "arm64" } elseif ($os.OSArchitecture -match "32") { "x86_32" } elseif ([IntPtr]::Size -eq 8) { "x86_64" } else { "x86_32" }

$obj = [ordered]@{
    server_name = $hn
    ip_address = $ip
    os_version = if ($os.Caption) { $os.Caption.Trim() } else { "Windows" }
    os_type = "Windows"
    architecture_type = $arch
    processor_model = if ($cpu.Name) { $cpu.Name.Trim() } else { "N/A" }
    total_core = "$cores Cores"
    total_memory = if ($totMemGB -gt 0) { "$totMemGB GB" } else { "N/A" }
    total_dimm_memory = $dimmStr
    total_storage_size = $totDiskStr
    total_disk_count = $diskListStr
    total_network_interfaces = $nicListStr
    gpu_model = $gpuName
    gpu_type = $gpuType
    total_vram = $gpuVram
}
$obj | ConvertTo-Json -Compress
`

func encodePowerShellCommand(psCode string) string {
	runes := utf16.Encode([]rune(psCode))
	b := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(b[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func isWindowsRemoteHost(cfg *domain.RemoteHostConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.Port == 3389 {
		return true
	}
	for _, tag := range cfg.Tags {
		lower := strings.ToLower(tag)
		if lower == "rdp" || lower == "windows" || lower == "win" {
			return true
		}
	}
	lowerName := strings.ToLower(cfg.Name)
	if strings.Contains(lowerName, "windows") || strings.Contains(lowerName, "win10") ||
		strings.Contains(lowerName, "win11") || strings.Contains(lowerName, "win20") ||
		strings.Contains(lowerName, "winserver") || strings.Contains(lowerName, "hyper-v") ||
		strings.Contains(lowerName, "hyperv") {
		return true
	}
	if strings.ToLower(cfg.Username) == "administrator" {
		return true
	}
	return false
}

func testTcpPort(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func probeRdpHostInfo(host string, timeout time.Duration) (string, string, bool) {
	if !testTcpPort(host, 3389, timeout) {
		return "", "", false
	}

	computerName := ""
	osVersion := "Windows Server (RDP)"

	// 1. Try querying NetBIOS for hostname
	_, nbName, nbOk := queryNetBIOS(host, 400*time.Millisecond)
	if nbOk && nbName != "" {
		computerName = nbName
	}

	// 2. Try TLS handshake on 3389 to read certificate Subject CN
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}
	tlsDialer := &net.Dialer{
		Timeout: timeout,
	}
	if tlsConn, err := tls.DialWithDialer(tlsDialer, "tcp", net.JoinHostPort(host, "3389"), tlsConf); err == nil {
		defer tlsConn.Close()
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			if computerName == "" && cert.Subject.CommonName != "" {
				computerName = cert.Subject.CommonName
			}
		}
	}

	return computerName, osVersion, true
}

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

	now := time.Now()
	var userPtr *int
	if userID > 0 {
		userPtr = &userID
	}

	isWindows := isWindowsRemoteHost(cfg)
	var raw rawProbePayload
	hasValidJSON := false
	var lastErr error

	if isWindows {
		// Attempt A: OpenSSH on Windows (if port is 22 or if port 22 is open when cfg.Port == 3389)
		sshCfg := cfg
		canTrySSH := false
		if cfg.Port != 3389 {
			canTrySSH = true
		} else if testTcpPort(cfg.Host, 22, 1200*time.Millisecond) {
			cloned := *cfg
			cloned.Port = 22
			sshCfg = &cloned
			canTrySSH = true
		}

		if canTrySSH {
			psCmd := fmt.Sprintf("powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand %s", encodePowerShellCommand(windowsDiscoveryPowerShellScript))
			stdout, stderr, exitCode, sshErr := s.sshService.ExecuteCommand(sshCfg, psCmd)
			cleanOut := strings.TrimSpace(stdout)
			startIdx := strings.Index(cleanOut, "{")
			endIdx := strings.LastIndex(cleanOut, "}")
			if sshErr == nil && startIdx != -1 && endIdx != -1 && endIdx > startIdx {
				jsonStr := cleanOut[startIdx : endIdx+1]
				if parseErr := json.Unmarshal([]byte(jsonStr), &raw); parseErr == nil && strings.TrimSpace(raw.ServerName) != "" {
					hasValidJSON = true
				}
			} else {
				if sshErr != nil {
					lastErr = sshErr
				} else if strings.TrimSpace(stderr) != "" {
					lastErr = errors.New(strings.TrimSpace(stderr))
				} else if exitCode != 0 {
					lastErr = fmt.Errorf("exit code %d", exitCode)
				}
			}
		}

		// Attempt B: If SSH was not available or didn't return valid JSON, check RDP reachability (port 3389)
		if !hasValidJSON {
			rdpPort := cfg.Port
			if rdpPort != 3389 && !testTcpPort(cfg.Host, rdpPort, 2*time.Second) {
				rdpPort = 3389
			}
			if testTcpPort(cfg.Host, rdpPort, 3*time.Second) {
				compName, osVer, _ := probeRdpHostInfo(cfg.Host, 2*time.Second)
				srvName := cfg.Name
				if compName != "" {
					srvName = compName
				}
				item := domain.ServerInventoryItem{
					RemoteHostID:           &hostID,
					ServerName:             srvName,
					IPAddress:              cfg.Host,
					OSVersion:              osVer,
					OSType:                 "Windows",
					ArchitectureType:       "x86_64",
					ProcessorModel:         "Intel / AMD Processor",
					TotalCore:              "N/A",
					TotalMemory:            "N/A",
					TotalDimmMemory:        "N/A",
					TotalStorageSize:       "N/A",
					TotalDiskCount:         "N/A",
					TotalNetworkInterfaces: "N/A",
					GPUModel:               "N/A",
					GPUType:                "N/A",
					TotalVRAM:              "N/A",
					Status:                 "active",
					Notes:                  fmt.Sprintf("Auto-discovered via Remote Desktop (RDP %d). Active & reachable.", rdpPort),
					UserID:                 userPtr,
					LastSyncedAt:           &now,
				}

				res, _, saveErr := s.repo.UpsertFromProbe(ctx, &item)
				if saveErr != nil {
					return nil, fmt.Errorf("failed saving remote host to inventory: %w", saveErr)
				}
				return res, nil
			}
		}
	} else {
		// Non-Windows (assumed Linux): Try bash discovery script first
		stdout, stderr, exitCode, sshErr := s.sshService.ExecuteCommand(cfg, linuxDiscoveryBashScript)
		cleanOut := strings.TrimSpace(stdout)
		startIdx := strings.Index(cleanOut, "{")
		endIdx := strings.LastIndex(cleanOut, "}")
		if sshErr == nil && startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			jsonStr := cleanOut[startIdx : endIdx+1]
			if parseErr := json.Unmarshal([]byte(jsonStr), &raw); parseErr == nil && strings.TrimSpace(raw.ServerName) != "" {
				hasValidJSON = true
			}
		} else {
			if sshErr != nil {
				lastErr = sshErr
			} else if strings.TrimSpace(stderr) != "" {
				lastErr = errors.New(strings.TrimSpace(stderr))
			} else if exitCode != 0 {
				lastErr = fmt.Errorf("exit code %d", exitCode)
			}
		}

		// What if the host was actually a Windows server running SSH with CMD/PowerShell?
		if !hasValidJSON && sshErr == nil {
			psCmd := fmt.Sprintf("powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand %s", encodePowerShellCommand(windowsDiscoveryPowerShellScript))
			stdoutPs, _, _, psErr := s.sshService.ExecuteCommand(cfg, psCmd)
			cleanPs := strings.TrimSpace(stdoutPs)
			startPs := strings.Index(cleanPs, "{")
			endPs := strings.LastIndex(cleanPs, "}")
			if psErr == nil && startPs != -1 && endPs != -1 && endPs > startPs {
				jsonStr := cleanPs[startPs : endPs+1]
				if parseErr := json.Unmarshal([]byte(jsonStr), &raw); parseErr == nil && strings.TrimSpace(raw.ServerName) != "" {
					hasValidJSON = true
					isWindows = true
				}
			}
		}

		// What if SSH failed, but port 3389 is open (Windows RDP host registered without tags)?
		if !hasValidJSON && testTcpPort(cfg.Host, 3389, 2*time.Second) {
			compName, osVer, _ := probeRdpHostInfo(cfg.Host, 2*time.Second)
			srvName := cfg.Name
			if compName != "" {
				srvName = compName
			}
			item := domain.ServerInventoryItem{
				RemoteHostID:           &hostID,
				ServerName:             srvName,
				IPAddress:              cfg.Host,
				OSVersion:              osVer,
				OSType:                 "Windows",
				ArchitectureType:       "x86_64",
				ProcessorModel:         "Intel / AMD Processor",
				TotalCore:              "N/A",
				TotalMemory:            "N/A",
				TotalDimmMemory:        "N/A",
				TotalStorageSize:       "N/A",
				TotalDiskCount:         "N/A",
				TotalNetworkInterfaces: "N/A",
				GPUModel:               "N/A",
				GPUType:                "N/A",
				TotalVRAM:              "N/A",
				Status:                 "active",
				Notes:                  "Auto-discovered via Remote Desktop (RDP 3389). Active & reachable.",
				UserID:                 userPtr,
				LastSyncedAt:           &now,
			}

			res, _, saveErr := s.repo.UpsertFromProbe(ctx, &item)
			if saveErr != nil {
				return nil, fmt.Errorf("failed saving remote host to inventory: %w", saveErr)
			}
			return res, nil
		}
	}

	if !hasValidJSON {
		errReason := "Connection or probe command failed"
		if lastErr != nil {
			errReason = lastErr.Error()
		}
		fallbackOSType := "Linux"
		if isWindows {
			fallbackOSType = "Windows"
		}

		item := domain.ServerInventoryItem{
			RemoteHostID:           &hostID,
			ServerName:             cfg.Name,
			IPAddress:              cfg.Host,
			OSVersion:              "N/A",
			OSType:                 fallbackOSType,
			ArchitectureType:       "N/A",
			ProcessorModel:         "N/A",
			TotalCore:              "N/A",
			TotalMemory:            "N/A",
			TotalDimmMemory:        "N/A",
			TotalStorageSize:       "N/A",
			TotalDiskCount:         "N/A",
			TotalNetworkInterfaces: "N/A",
			GPUModel:               "N/A",
			GPUType:                "N/A",
			TotalVRAM:              "N/A",
			Status:                 "offline",
			Notes:                  fmt.Sprintf("Probe unreachable: %s", errReason),
			UserID:                 userPtr,
			LastSyncedAt:           &now,
		}

		res, _, saveErr := s.repo.UpsertFromProbe(ctx, &item)
		if saveErr != nil {
			return nil, fmt.Errorf("failed saving remote host to inventory: %w", saveErr)
		}
		return res, fmt.Errorf("Probe unreachable: %s (saved as offline)", errReason)
	}

	// Successful probe (Linux or Windows via PowerShell)
	srvName := strings.TrimSpace(raw.ServerName)
	if srvName == "" || srvName == "N/A" {
		srvName = cfg.Name
	}
	srvIP := strings.TrimSpace(raw.IPAddress)
	if srvIP == "" || srvIP == "N/A" {
		srvIP = cfg.Host
	}

	targetOSType := strings.TrimSpace(raw.OSType)
	if targetOSType == "" || targetOSType == "N/A" {
		if isWindows {
			targetOSType = "Windows"
		} else {
			targetOSType = "Linux"
		}
	}

	item := domain.ServerInventoryItem{
		RemoteHostID:           &hostID,
		ServerName:             srvName,
		IPAddress:              srvIP,
		OSVersion:              fallbackStr(raw.OSVersion, "N/A"),
		OSType:                 targetOSType,
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
		UserID:                 userPtr,
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
	sem := make(chan struct{}, 8) // max 8 concurrent SSH probes
	var wg sync.WaitGroup

	for _, h := range hosts {
		wg.Add(1)
		go func(host domain.RemoteHostConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			probeCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			defer cancel()

			item, err := s.SyncFromRemoteHost(probeCtx, host.ID, userID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if item != nil {
					// Host was successfully registered to inventory with offline status
					result.SyncedSuccess++
					result.Errors = append(result.Errors, fmt.Sprintf("%s (%s): %v", host.Name, host.Host, err))
				} else {
					result.SyncedFailed++
					result.Errors = append(result.Errors, fmt.Sprintf("%s (%s): %v", host.Name, host.Host, err))
				}
				logger.Warn("ServerInventory", fmt.Sprintf("Notice syncing host %s: %v", host.Name, err))
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

	sample3 := []string{
		"win-app-srv-01",
		"192.168.1.30",
		"Microsoft Windows Server 2022 Datacenter",
		"Windows",
		"x86_64",
		"Intel Xeon E5-2680 v4 @ 2.40GHz",
		"16 Cores",
		"32.00 GB",
		"4 DIMMs",
		"500.00 GB",
		"1 Disks [PHYSICALDRIVE0 (500G)]",
		"2 Interfaces [Ethernet0, vEthernet]",
		"N/A",
		"N/A",
		"N/A",
		"active",
		"Windows Active Directory / Application Host",
	}

	_ = w.Write(sample1)
	_ = w.Write(sample2)
	_ = w.Write(sample3)
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

	var userPtr *int
	if userID > 0 {
		userPtr = &userID
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
			UserID:                 userPtr,
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
