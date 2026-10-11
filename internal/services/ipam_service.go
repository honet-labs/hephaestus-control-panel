package services

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/config"
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
		// Run initial check after 5s startup delay so DB pool is ready
		time.Sleep(5 * time.Second)
		s.triggerDueScans()

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
		go func(id string, interval string) {
			scanCtx, scanCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer scanCancel()
			if _, err := s.ScanSubnet(scanCtx, id); err != nil {
				logger.Error("IPAM", fmt.Sprintf("Scheduled scan failed for subnet ID %s", id), err)
				// Reschedule next attempt in 5 minutes so it doesn't immediately repeat on error
				nextAttempt := time.Now().Add(5 * time.Minute)
				_ = s.ipamRepo.UpdateSubnetNextScan(context.Background(), id, &nextAttempt)
			}
		}(subnetID, sub.ScanInterval)
	}
}

// GenerateSubnetHostIPs generates usable host IP strings for a given CIDR (IPv4 or small IPv6)
func GenerateSubnetHostIPs(cidrStr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %w", err)
	}

	ipv4 := ip.To4()
	if ipv4 != nil {
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

	// IPv6 CIDR
	ones, bits := ipnet.Mask.Size()
	if bits == 128 && ones >= 120 {
		var hosts []string
		baseIP := make(net.IP, 16)
		copy(baseIP, ipnet.IP)
		numHosts := 1 << (128 - ones)
		for i := 1; i < numHosts; i++ {
			currIP := make(net.IP, 16)
			copy(currIP, baseIP)
			currIP[15] += byte(i)
			hosts = append(hosts, currIP.String())
		}
		return hosts, nil
	}

	// For standard IPv6 subnets (/64 etc.), brute-force host generation is impossible.
	// We return an empty slice, and host discovery relies on NDP / neighbor cache.
	return []string{}, nil
}

func calculateNextScanAt(interval string, from time.Time) *time.Time {
	interval = strings.ToLower(strings.TrimSpace(interval))
	if interval == "" || interval == "manual" {
		return nil
	}

	var d time.Duration
	switch interval {
	case "5m":
		d = 5 * time.Minute
	case "10m":
		d = 10 * time.Minute
	case "15m":
		d = 15 * time.Minute
	case "30m":
		d = 30 * time.Minute
	case "1h":
		d = 1 * time.Hour
	case "2h":
		d = 2 * time.Hour
	case "4h":
		d = 4 * time.Hour
	case "6h":
		d = 6 * time.Hour
	case "12h":
		d = 12 * time.Hour
	case "1d", "24h":
		d = 24 * time.Hour
	case "3d", "72h":
		d = 72 * time.Hour
	case "7d", "1w":
		d = 7 * 24 * time.Hour
	default:
		parsed, err := time.ParseDuration(interval)
		if err == nil && parsed > 0 {
			d = parsed
		} else {
			return nil
		}
	}
	next := from.Add(d)
	return &next
}

// =========================================================================
// NETWORK SCANNING & OS / HARDWARE FINGERPRINTING ENGINES
// =========================================================================

// readSystemARPTable extracts IP to MAC address mappings from host /proc/net/arp and ip neigh
func readSystemARPTable() map[string]string {
	arpMap := make(map[string]string)

	// 1. Read /proc/net/arp (Linux kernel standard)
	if data, err := os.ReadFile("/proc/net/arp"); err == nil {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i == 0 {
				continue // skip header: IP address HW type Flags HW address Mask Device
			}
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				ip := fields[0]
				mac := strings.ToLower(fields[3])
				if mac != "00:00:00:00:00:00" && mac != "" && !strings.HasPrefix(mac, "<incomplete>") {
					arpMap[ip] = mac
				}
			}
		}
	}

	// 2. Read 'ip neigh show'
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ip", "neigh", "show")
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				ip := fields[0]
				for idx, f := range fields {
					if f == "lladdr" && idx+1 < len(fields) {
						mac := strings.ToLower(fields[idx+1])
						if mac != "00:00:00:00:00:00" && mac != "" {
							arpMap[ip] = mac
						}
						break
					}
				}
			}
		}
	}

	return arpMap
}

// readIPv6Neighbors scans the IPv6 neighbor cache for active addresses in the subnet
func readIPv6Neighbors(cidrStr string) []string {
	_, ipnet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ip", "-6", "neigh", "show")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var found []string
	seen := make(map[string]bool)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			targetIP := net.ParseIP(fields[0])
			if targetIP != nil && ipnet.Contains(targetIP) {
				ipStr := targetIP.String()
				if !seen[ipStr] {
					seen[ipStr] = true
					found = append(found, ipStr)
				}
			}
		}
	}
	return found
}

// queryNetBIOS sends a NetBIOS Node Status query to UDP port 137 to resolve computer name and hardware MAC
func queryNetBIOS(targetIP string, timeout time.Duration) (mac string, computerName string, ok bool) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(targetIP, "137"), timeout)
	if err != nil {
		return "", "", false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// NetBIOS Node Status Query packet (50 bytes)
	req := []byte{
		0x80, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x20, 0x43, 0x4B, 0x41,
		0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41,
		0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41,
		0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41, 0x41,
		0x41, 0x41, 0x41, 0x41, 0x41, 0x00, 0x00, 0x21,
		0x00, 0x01,
	}

	if _, err := conn.Write(req); err != nil {
		return "", "", false
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n < 57 {
		return "", "", false
	}

	numNames := int(buf[56])
	offset := 57
	for i := 0; i < numNames && offset+18 <= n; i++ {
		nameBytes := buf[offset : offset+15]
		nameType := buf[offset+15]
		if (nameType == 0x00 || nameType == 0x20) && computerName == "" {
			nameStr := strings.TrimSpace(string(nameBytes))
			if nameStr != "" && !strings.HasPrefix(nameStr, "IS~") && !strings.HasPrefix(nameStr, "__MSBROWSE__") {
				computerName = nameStr
			}
		}
		offset += 18
	}

	// Unit ID (Hardware MAC address) is right after names
	if offset+6 <= n {
		macBytes := buf[offset : offset+6]
		mac = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
			macBytes[0], macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5])
		if mac == "00:00:00:00:00:00" {
			mac = ""
		}
	}

	return mac, computerName, true
}

// lookupMACVendor matches IEEE OUI prefixes against major hardware manufacturers
func lookupMACVendor(mac string) string {
	clean := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
	if len(clean) < 6 {
		return ""
	}
	prefix := clean[:6]

	ouiMap := map[string]string{
		"525400": "QEMU / KVM Virtual",
		"000c29": "VMware Virtual",
		"005056": "VMware Virtual",
		"000569": "VMware Virtual",
		"080027": "Oracle VirtualBox",
		"00155d": "Microsoft Hyper-V",
		"b827eb": "Raspberry Pi",
		"dca632": "Raspberry Pi",
		"e45f01": "Raspberry Pi",
		"28cdc1": "Raspberry Pi",
		"488f5a": "Apple",
		"a483e7": "Apple",
		"f01898": "Apple",
		"3c22fb": "Apple",
		"acbc32": "Apple",
		"bcd074": "Apple",
		"147dda": "Apple",
		"8c8590": "Apple",
		"f4d488": "Apple",
		"64a2f9": "Samsung",
		"34c059": "Samsung",
		"842519": "Samsung",
		"9c0298": "Samsung",
		"e8508b": "Samsung",
		"b8bc5b": "Samsung",
		"2c4d54": "Samsung",
		"3482c5": "Xiaomi",
		"50642b": "Xiaomi",
		"64cc2e": "Xiaomi",
		"98fae3": "Xiaomi",
		"a444d1": "Xiaomi",
		"c40bcb": "Xiaomi",
		"485702": "Huawei",
		"001e10": "Huawei",
		"00259e": "Huawei",
		"0819a6": "Huawei",
		"286ed4": "Huawei",
		"7072cf": "Huawei",
		"b4fbe4": "Google",
		"001a11": "Google",
		"3c5a37": "Google",
		"546009": "Google",
		"703eac": "Google",
		"f4f5e8": "Google",
		"dc2c6e": "MikroTik",
		"6c3b6b": "MikroTik",
		"b869f4": "MikroTik",
		"cc2de0": "MikroTik",
		"e48d8c": "MikroTik",
		"000c42": "MikroTik",
		"00000c": "Cisco Systems",
		"000142": "Cisco Systems",
		"000143": "Cisco Systems",
		"000163": "Cisco Systems",
		"000164": "Cisco Systems",
		"000196": "Cisco Systems",
		"000197": "Cisco Systems",
		"50c7bf": "TP-Link",
		"14cf92": "TP-Link",
		"30b5c2": "TP-Link",
		"54af97": "TP-Link",
		"704f57": "TP-Link",
		"984827": "TP-Link",
		"c006c3": "TP-Link",
		"f09fc2": "Ubiquiti UniFi",
		"245a4c": "Ubiquiti UniFi",
		"44d9e7": "Ubiquiti UniFi",
		"687251": "Ubiquiti UniFi",
		"7483c2": "Ubiquiti UniFi",
		"788a20": "Ubiquiti UniFi",
		"802aa8": "Ubiquiti UniFi",
		"240ac4": "Espressif (ESP32/ESP8266)",
		"30aea4": "Espressif (ESP32/ESP8266)",
		"840d8e": "Espressif (ESP32/ESP8266)",
		"a4cf12": "Espressif (ESP32/ESP8266)",
		"bcddc2": "Espressif (ESP32/ESP8266)",
		"cc50e3": "Espressif (ESP32/ESP8266)",
		"ecfabc": "Espressif (ESP32/ESP8266)",
		"001b21": "Intel Corporate",
		"001e67": "Intel Corporate",
		"00215a": "Intel Corporate",
		"3417eb": "Intel Corporate",
		"48210b": "Intel Corporate",
		"6805ca": "Intel Corporate",
		"a0369f": "Intel Corporate",
		"00e04c": "Realtek",
		"52544c": "Realtek",
		"0010dc": "Realtek",
		"001422": "Dell",
		"00188b": "Dell",
		"002219": "Dell",
		"1866da": "Dell",
		"842b2b": "Dell",
		"b82a72": "Dell",
		"000802": "HP",
		"000bcd": "HP",
		"000f20": "HP",
		"3cd92b": "HP",
		"9cb654": "HP",
	}

	if strings.Contains(strings.ToLower(mac), "wg") || strings.Contains(strings.ToLower(mac), "wireguard") {
		return "WireGuard VPN Peer"
	}

	if vendor, ok := ouiMap[prefix]; ok {
		return vendor
	}
	return ""
}

// readWireGuardPeers extracts active peer IPs and their public keys / interfaces from Linux kernel WireGuard
func readWireGuardPeers() map[string]string {
	peers := make(map[string]string)
	if runtime.GOOS == "windows" {
		return peers
	}

	cmd := exec.Command("wg", "show", "all", "dump")
	out, err := cmd.Output()
	if err != nil {
		return peers
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			iface := fields[0]
			pubKey := fields[1]
			allowedIPs := strings.Split(fields[4], ",")
			for _, aip := range allowedIPs {
				ipOnly := strings.Split(strings.TrimSpace(aip), "/")[0]
				if ipOnly != "" {
					shortKey := pubKey
					if len(shortKey) > 10 {
						shortKey = shortKey[:10] + "..."
					}
					peers[ipOnly] = fmt.Sprintf("%s:%s", iface, shortKey)
				}
			}
		}
	}
	return peers
}

// probeTCPPort quickly checks if a port is open and optionally grabs its banner
func probeTCPPort(ip string, port int, timeout time.Duration) (bool, string) {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, ""
	}
	defer conn.Close()

	banner := ""
	if port == 22 {
		_ = conn.SetDeadline(time.Now().Add(350 * time.Millisecond))
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		if n > 0 {
			banner = string(buf[:n])
		}
	} else if port == 80 || port == 443 {
		_ = conn.SetDeadline(time.Now().Add(350 * time.Millisecond))
		_, _ = conn.Write([]byte("HEAD / HTTP/1.0\r\n\r\n"))
		buf := make([]byte, 512)
		n, _ := conn.Read(buf)
		if n > 0 {
			banner = string(buf[:n])
		}
	}

	return true, banner
}

// detectOSAndDevice combines NetBIOS, Reverse DNS, TCP probing, MAC OUI, and TTL metrics
func detectOSAndDevice(ctx context.Context, ip string, ttl int, mac string, existingHostname, subnetName string) (osFamily, deviceType, detectedHostname, detectedMAC string) {
	detectedHostname = existingHostname
	detectedMAC = mac
	devType := "Server"

	// 1. If MAC is empty, try NetBIOS query
	if detectedMAC == "" {
		if nbMac, nbName, ok := queryNetBIOS(ip, 250*time.Millisecond); ok {
			if detectedMAC == "" {
				detectedMAC = nbMac
			}
			if detectedHostname == "" && nbName != "" {
				detectedHostname = nbName
			}
			return "Windows", "Workstation", detectedHostname, detectedMAC
		}
	}

	// 2. Reverse DNS check if hostname empty
	if detectedHostname == "" {
		rCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		if names, err := net.DefaultResolver.LookupAddr(rCtx, ip); err == nil && len(names) > 0 {
			detectedHostname = strings.TrimSuffix(names[0], ".")
		}
		cancel()
	}

	lowerHost := strings.ToLower(detectedHostname)
	vendor := lookupMACVendor(detectedMAC)
	lowerVendor := strings.ToLower(vendor)

	// 3. Fast TCP signature probing (ports: 22, 135, 139, 445, 3389, 5357, 5555, 80, 443)
	checkPorts := []int{22, 135, 139, 445, 3389, 5357, 5555, 80}
	var wg sync.WaitGroup
	var pResults sync.Map

	for _, p := range checkPorts {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			open, banner := probeTCPPort(ip, port, 400*time.Millisecond)
			if open {
				pResults.Store(port, banner)
			}
		}(p)
	}
	wg.Wait()

	// Analyze TCP probe results:
	// Android ADB (port 5555)
	if _, open := pResults.Load(5555); open {
		return "Android", "Mobile", detectedHostname, detectedMAC
	}

	// Windows MSRPC (135), NetBIOS (139), SMB (445), RDP (3389), WSDAPI (5357)
	_, has135 := pResults.Load(135)
	_, has139 := pResults.Load(139)
	_, has445 := pResults.Load(445)
	_, has3389 := pResults.Load(3389)
	_, has5357 := pResults.Load(5357)
	if has135 || has139 || has445 || has3389 || has5357 {
		devType = "Workstation"
		if has3389 || (has445 && (strings.Contains(lowerHost, "srv") || strings.Contains(lowerHost, "server"))) {
			devType = "Server"
		}
		return "Windows", devType, detectedHostname, detectedMAC
	}

	// SSH Port 22 Banner
	if bVal, open := pResults.Load(22); open {
		banner := strings.ToLower(bVal.(string))
		if strings.Contains(banner, "ubuntu") {
			return "Linux (Ubuntu)", "Server", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "debian") {
			return "Linux (Debian)", "Server", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "centos") || strings.Contains(banner, "redhat") || strings.Contains(banner, "el") {
			return "Linux (RHEL/CentOS)", "Server", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "alpine") {
			return "Linux (Alpine)", "VM", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "openwrt") {
			return "Linux (OpenWrt)", "Router", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "mikrotik") || strings.Contains(banner, "rosmk") {
			return "RouterOS (MikroTik)", "Router", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "cisco") {
			return "Cisco IOS", "Switch", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "for_windows") {
			return "Windows", "Server", detectedHostname, detectedMAC
		}
		return "Linux", "Server", detectedHostname, detectedMAC
	}

	// HTTP/HTTPS Web Server Banner
	if bVal, open := pResults.Load(80); open {
		banner := strings.ToLower(bVal.(string))
		if strings.Contains(banner, "microsoft-iis") {
			return "Windows", "Server", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "mikrotik") {
			return "RouterOS (MikroTik)", "Router", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "openwrt") || strings.Contains(banner, "luci") {
			return "Linux (OpenWrt)", "Router", detectedHostname, detectedMAC
		} else if strings.Contains(banner, "apache") || strings.Contains(banner, "nginx") || strings.Contains(banner, "lighttpd") {
			return "Linux", "Server", detectedHostname, detectedMAC
		}
	}

	// 4. Hostname patterns
	if strings.Contains(lowerHost, "android") || strings.Contains(lowerHost, "galaxy") || strings.Contains(lowerHost, "pixel") || strings.Contains(lowerHost, "redmi") || strings.Contains(lowerHost, "xiaomi") || strings.Contains(lowerHost, "oppo") || strings.Contains(lowerHost, "vivo") {
		return "Android", "Mobile", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerHost, "iphone") || strings.Contains(lowerHost, "ipad") {
		return "Apple iOS", "Mobile", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerHost, "macbook") || strings.Contains(lowerHost, "imac") || strings.Contains(lowerHost, "apple") {
		return "macOS", "Workstation", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerHost, "win-") || strings.Contains(lowerHost, "desktop-") || strings.Contains(lowerHost, "laptop-") || strings.Contains(lowerHost, "windows") {
		return "Windows", "Workstation", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerHost, "ubuntu") || strings.Contains(lowerHost, "debian") || strings.Contains(lowerHost, "centos") || strings.Contains(lowerHost, "linux") || strings.Contains(lowerHost, "raspberry") {
		return "Linux", "Server", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerHost, "router") || strings.Contains(lowerHost, "gateway") || strings.Contains(lowerHost, "switch") || strings.Contains(lowerHost, "mikrotik") {
		return "Network Device", "Router", detectedHostname, detectedMAC
	}

	// 5. MAC Vendor patterns
	if strings.Contains(lowerVendor, "apple") {
		return "Apple iOS/macOS", "Mobile", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerVendor, "samsung") || strings.Contains(lowerVendor, "xiaomi") || strings.Contains(lowerVendor, "huawei") || strings.Contains(lowerVendor, "google") {
		if ttl > 0 && ttl <= 64 {
			return "Android", "Mobile", detectedHostname, detectedMAC
		}
	}
	if strings.Contains(lowerVendor, "mikrotik") || strings.Contains(lowerVendor, "cisco") || strings.Contains(lowerVendor, "tp-link") || strings.Contains(lowerVendor, "ubiquiti") {
		return "Network OS", "Router", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerVendor, "espressif") {
		return "Embedded / FreeRTOS", "IoT Device", detectedHostname, detectedMAC
	}
	if strings.Contains(lowerVendor, "vmware") || strings.Contains(lowerVendor, "qemu") || strings.Contains(lowerVendor, "virtualbox") {
		devType = "VM"
	}

	// 6. TTL Fingerprinting fallback
	if ttl > 0 {
		if ttl <= 64 {
			if devType == "VM" {
				return "Linux (VM)", "VM", detectedHostname, detectedMAC
			}
			if strings.Contains(strings.ToLower(subnetName), "wireguard") || strings.Contains(strings.ToLower(subnetName), "vpn") {
				return "Linux / Android", "Mobile / Host", detectedHostname, detectedMAC
			}
			return "Linux", "Server", detectedHostname, detectedMAC
		} else if ttl > 64 && ttl <= 128 {
			return "Windows", "Workstation", detectedHostname, detectedMAC
		} else if ttl > 128 {
			return "Network Device", "Router", detectedHostname, detectedMAC
		}
	}

	// 7. Active Reachable fallback when TTL was not captured or filtered
	if detectedMAC == "" && (strings.Contains(strings.ToLower(subnetName), "wireguard") || strings.Contains(strings.ToLower(subnetName), "vpn")) {
		detectedMAC = "Virtual (WireGuard)"
	}

	if strings.Contains(strings.ToLower(subnetName), "wireguard") || strings.Contains(strings.ToLower(subnetName), "vpn") {
		return "Linux / Android", "Mobile / Host", detectedHostname, detectedMAC
	}

	return "Linux / Unix", devType, detectedHostname, detectedMAC
}

// =========================================================================
// SUBNET & SCAN MANAGEMENT METHODS
// =========================================================================

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

	ipVersion := strings.ToLower(strings.TrimSpace(req.IPVersion))
	if ipVersion == "" {
		if strings.Contains(cleanCIDR, ":") {
			ipVersion = "ipv6"
		} else {
			ipVersion = "ipv4"
		}
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
		IPVersion:      ipVersion,
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
			IPVersion:  ipVersion,
			Status:     "reserved",
			Hostname:   "Default Gateway",
			DeviceType: "Gateway",
			Notes:      "Subnet default gateway",
		}
		_ = s.ipamRepo.UpsertAddress(ctx, gatewayAddr)
		sub.TotalUsedIPs = 1
		if totalIPs > 0 {
			sub.TotalUnusedIPs = totalIPs - 1
		}
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
	sub.ScanInterval = req.ScanInterval
	sub.NextScanAt = calculateNextScanAt(req.ScanInterval, time.Now())

	if err := s.ipamRepo.UpdateSubnet(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// DeleteSubnet removes a subnet and cascaded IP allocations
func (s *IpamService) DeleteSubnet(ctx context.Context, id string) error {
	return s.ipamRepo.DeleteSubnet(ctx, id)
}

// ScanSubnet runs a high-performance concurrent scan with OS & MAC detection
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

	// For IPv6 /64 or large subnets, discover targets via neighbor cache & existing addresses
	if len(hostIPs) == 0 {
		targetSet := make(map[string]bool)
		if sub.Gateway != "" {
			targetSet[sub.Gateway] = true
		}
		existingList, _ := s.ipamRepo.ListAddressesBySubnet(ctx, subnetID)
		for _, a := range existingList {
			targetSet[a.IPAddress] = true
		}
		for _, nip := range readIPv6Neighbors(sub.CIDR) {
			targetSet[nip] = true
		}
		for ip := range targetSet {
			hostIPs = append(hostIPs, ip)
		}
	}

	startTime := time.Now()
	totalHosts := len(hostIPs)

	logger.Info("IPAM", fmt.Sprintf("Starting scan for subnet '%s' (%s) with %d hosts...", sub.Name, sub.CIDR, totalHosts))

	type scanResult struct {
		ip        string
		reachable bool
		latency   *float64
		ttl       int
	}

	resultsChan := make(chan scanResult, totalHosts+1)
	maxScanWorkers := config.GetConfig().GetServiceThreads().IPAM
	if maxScanWorkers <= 0 {
		maxScanWorkers = 30
	}
	sem := make(chan struct{}, maxScanWorkers)
	var wg sync.WaitGroup

	for _, ip := range hostIPs {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pingCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
			defer cancel()

			reachable, latency, ttl := pingHostWithDetails(pingCtx, target)
			resultsChan <- scanResult{
				ip:        target,
				reachable: reachable,
				latency:   latency,
				ttl:       ttl,
			}
		}(ip)
	}

	wg.Wait()
	close(resultsChan)

	// Read system ARP/Neighbor table and WireGuard VPN peers after pings populate kernel cache
	arpTable := readSystemARPTable()
	wgPeers := readWireGuardPeers()

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

		ipVersion := "ipv4"
		if strings.Contains(res.ip, ":") {
			ipVersion = "ipv6"
		}

		if res.reachable {
			foundActive++

			mac := arpTable[res.ip]
			if mac == "" {
				if wgMac, ok := wgPeers[res.ip]; ok {
					mac = wgMac
				}
			}
			existingHostname := ""
			if existing, ok := existingMap[res.ip]; ok {
				existingHostname = existing.Hostname
				if mac == "" && existing.MACAddress != "" {
					mac = existing.MACAddress
				}
			}

			osFamily, deviceType, detectedHostname, detectedMAC := detectOSAndDevice(ctx, res.ip, res.ttl, mac, existingHostname, sub.Name)
			if detectedMAC == "" && mac != "" {
				detectedMAC = mac
			}
			vendor := lookupMACVendor(detectedMAC)

			if existing, ok := existingMap[res.ip]; ok {
				// Update existing address
				existing.IsOnline = true
				if existing.Status == "discovered" || existing.Status == "offline" || existing.Status == "" {
					existing.Status = "active"
				}
				existing.ResponseTimeMS = latencyMS
				existing.LastSeenAt = &now
				existing.IPVersion = ipVersion
				if existing.OSFamily == "" || existing.OSFamily == "Unknown" {
					existing.OSFamily = osFamily
				}
				if existing.DeviceType == "" || existing.DeviceType == "Unknown" || existing.DeviceType == "Server" {
					if deviceType != "Unknown" && deviceType != "" {
						existing.DeviceType = deviceType
					}
				}
				if existing.Hostname == "" && detectedHostname != "" {
					existing.Hostname = detectedHostname
				}
				if existing.MACAddress == "" && detectedMAC != "" {
					existing.MACAddress = detectedMAC
				}
				if existing.MACVendor == "" && vendor != "" {
					existing.MACVendor = vendor
				}
				_ = s.ipamRepo.UpsertAddress(ctx, &existing)
			} else {
				// Newly discovered host
				newAddr := &domain.IpamAddress{
					ID:             fmt.Sprintf("ip-%s", uuid.New().String()[:8]),
					SubnetID:       subnetID,
					IPAddress:      res.ip,
					IPVersion:      ipVersion,
					Status:         "active",
					Hostname:       detectedHostname,
					MACAddress:     detectedMAC,
					MACVendor:      vendor,
					OSFamily:       osFamily,
					DeviceType:     deviceType,
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
				if existing.Status != "reserved" {
					existing.Status = "offline"
				}
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

	// Update subnet metrics (using a detached context with timeout to guarantee persistence even if client disconnects)
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer saveCancel()
	_ = s.ipamRepo.UpdateSubnetScanStats(saveCtx, subnetID, totalHosts, totalUsed, totalUnused, endTime, nextScan)

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
	_ = s.ipamRepo.SaveScanLog(saveCtx, scanLog)

	logger.Info("IPAM", fmt.Sprintf("Completed scan for '%s' in %dms: %d/%d active hosts found, %d used, %d unused.",
		sub.Name, durationMS, foundActive, totalHosts, totalUsed, totalUnused))

	return scanLog, nil
}

// ScanAllSubnets scans all configured subnets
func (s *IpamService) ScanAllSubnets(ctx context.Context) (*domain.IpamScanAllResponse, error) {
	subnets, err := s.ipamRepo.ListSubnets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subnets: %w", err)
	}

	res := &domain.IpamScanAllResponse{
		TotalSubnets: len(subnets),
		Results:      make([]domain.IpamScanLog, 0),
		Errors:       make([]string, 0),
	}

	for _, sub := range subnets {
		scanLog, err := s.ScanSubnet(ctx, sub.ID)
		if err != nil {
			errMsg := fmt.Sprintf("Subnet '%s' (%s): %v", sub.Name, sub.CIDR, err)
			res.Errors = append(res.Errors, errMsg)
			logger.Warn("IPAM", errMsg)
		} else {
			res.ScannedCount++
			res.TotalActive += scanLog.FoundActive
			res.Results = append(res.Results, *scanLog)
		}
	}

	return res, nil
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

	ipVersion := "ipv4"
	if strings.Contains(cleanIP, ":") {
		ipVersion = "ipv6"
	}
	cleanMAC := strings.TrimSpace(req.MACAddress)
	vendor := lookupMACVendor(cleanMAC)

	addr := &domain.IpamAddress{
		ID:         fmt.Sprintf("ip-%s", uuid.New().String()[:8]),
		SubnetID:   req.SubnetID,
		IPAddress:  cleanIP,
		IPVersion:  ipVersion,
		Status:     req.Status,
		Hostname:   strings.TrimSpace(req.Hostname),
		MACAddress: cleanMAC,
		MACVendor:  vendor,
		OSFamily:   req.OSFamily,
		DeviceType: req.DeviceType,
		Notes:      strings.TrimSpace(req.Notes),
	}

	if addr.Status == "" {
		addr.Status = "active"
	}
	if addr.DeviceType == "" {
		addr.DeviceType = "Server"
	}
	if addr.OSFamily == "" {
		addr.OSFamily = "Unknown"
	}

	if err := s.ipamRepo.UpsertAddress(ctx, addr); err != nil {
		return nil, err
	}

	// Trigger quick background ping to set liveness
	go func(target, subnetID string) {
		pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		reachable, latency, _ := pingHostWithDetails(pCtx, target)
		if existing, _ := s.ipamRepo.GetAddressByIP(context.Background(), subnetID, target); existing != nil {
			existing.IsOnline = reachable
			if latency != nil {
				existing.ResponseTimeMS = int(*latency)
			}
			now := time.Now()
			if reachable {
				existing.LastSeenAt = &now
				if existing.Status == "discovered" || existing.Status == "offline" {
					existing.Status = "active"
				}
			} else {
				if existing.Status != "reserved" {
					existing.Status = "offline"
				}
			}
			_ = s.ipamRepo.UpsertAddress(context.Background(), existing)
		}
	}(cleanIP, req.SubnetID)

	return addr, nil
}

// UpdateAddress updates metadata of an existing IP
func (s *IpamService) UpdateAddress(ctx context.Context, id string, req *domain.UpdateIpamAddressRequest) (*domain.IpamAddress, error) {
	if req.MACAddress != "" && req.MACVendor == "" {
		req.MACVendor = lookupMACVendor(req.MACAddress)
	}
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

	reachable, latency, _ := pingHostWithDetails(pCtx, cleanIP)
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
