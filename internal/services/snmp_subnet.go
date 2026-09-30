package services

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"github.com/gosnmp/gosnmp"
)

type SnmpSubnetScanner struct{}

func NewSnmpSubnetScanner() *SnmpSubnetScanner {
	return &SnmpSubnetScanner{}
}

// ExpandSubnetOrRange parses CIDR, range, or comma/newline-separated IPs
func ExpandSubnetOrRange(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("subnet or IP range input is required")
	}

	const maxHosts = 512

	// 1. Check for CIDR (e.g. 192.168.1.0/24)
	if strings.Contains(trimmed, "/") {
		_, ipNet, err := net.ParseCIDR(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR format '%s': %w", trimmed, err)
		}

		ones, bits := ipNet.Mask.Size()
		if bits != 32 {
			return nil, fmt.Errorf("only IPv4 subnets are currently supported for subnet scan")
		}
		if ones < 20 {
			return nil, fmt.Errorf("subnet mask /%d is too large (minimum allowed is /20 for browser safety)", ones)
		}

		startIP := binary.BigEndian.Uint32(ipNet.IP)
		totalHosts := 1 << (32 - ones)

		ips := make([]string, 0, totalHosts)
		// For standard subnets (/24 to /30), skip network (.0) and broadcast (.255)
		startIdx := uint32(0)
		endIdx := uint32(totalHosts - 1)
		if ones <= 30 {
			startIdx = 1
			endIdx = uint32(totalHosts - 2)
		}

		for i := startIdx; i <= endIdx; i++ {
			if len(ips) >= maxHosts {
				break
			}
			ipBytes := make(net.IP, 4)
			binary.BigEndian.PutUint32(ipBytes, startIP+i)
			ips = append(ips, ipBytes.String())
		}
		return ips, nil
	}

	// 2. Check for Range (e.g. 192.168.1.1-192.168.1.50)
	if strings.Contains(trimmed, "-") {
		parts := strings.SplitN(trimmed, "-", 2)
		start := net.ParseIP(strings.TrimSpace(parts[0])).To4()
		end := net.ParseIP(strings.TrimSpace(parts[1])).To4()
		if start == nil || end == nil {
			return nil, fmt.Errorf("invalid IP range bounds: %s", trimmed)
		}

		startNum := binary.BigEndian.Uint32(start)
		endNum := binary.BigEndian.Uint32(end)
		if startNum > endNum {
			return nil, fmt.Errorf("start IP cannot be greater than end IP in range")
		}

		ips := make([]string, 0, int(endNum-startNum+1))
		for num := startNum; num <= endNum; num++ {
			if len(ips) >= maxHosts {
				break
			}
			ipBytes := make(net.IP, 4)
			binary.BigEndian.PutUint32(ipBytes, num)
			ips = append(ips, ipBytes.String())
		}
		return ips, nil
	}

	// 3. Fallback: comma or whitespace/newline separated IPs
	splitFn := func(c rune) bool {
		return c == ',' || c == '\n' || c == '\r' || c == ' '
	}
	rawTokens := strings.FieldsFunc(trimmed, splitFn)
	ips := make([]string, 0, len(rawTokens))
	for _, tok := range rawTokens {
		tok = strings.TrimSpace(tok)
		if tok != "" {
			if parsed := net.ParseIP(tok); parsed != nil {
				ips = append(ips, parsed.String())
				if len(ips) >= maxHosts {
					break
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no valid IP addresses found in input")
	}

	return ips, nil
}

// Scan probes multiple IPs concurrently using a worker pool
func (s *SnmpSubnetScanner) Scan(req domain.SnmpSubnetScanRequest) (*domain.SnmpSubnetScanResult, error) {
	ips, err := ExpandSubnetOrRange(req.Subnet)
	if err != nil {
		return nil, err
	}

	if req.Port <= 0 {
		req.Port = 161
	}
	if req.Community == "" {
		req.Community = "public"
	}
	if req.Timeout <= 0 {
		req.Timeout = 2 // Fast probe timeout for subnet discovery
	}
	if req.Retries <= 0 {
		req.Retries = 1
	}

	snmpVersion := gosnmp.Version2c
	if req.Version == "v1" || req.Version == "1" {
		snmpVersion = gosnmp.Version1
	}

	type probeTask struct {
		ip string
	}

	taskChan := make(chan probeTask, len(ips))
	for _, ip := range ips {
		taskChan <- probeTask{ip: ip}
	}
	close(taskChan)

	const workerCount = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]domain.SnmpSubnetHost, 0, len(ips))

	startGlobal := time.Now()

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				hostResult := probeHost(task.ip, uint16(req.Port), req.Community, snmpVersion, req.Timeout, req.Retries)
				mu.Lock()
				results = append(results, hostResult)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	activeCount := 0
	for _, r := range results {
		if r.Reachable {
			activeCount++
		}
	}

	durationSec := math.Round(time.Since(startGlobal).Seconds()*100) / 100

	return &domain.SnmpSubnetScanResult{
		Subnet:      req.Subnet,
		TotalIPs:    len(ips),
		ScannedIPs:  len(results),
		ActiveHosts: activeCount,
		DurationSec: durationSec,
		Hosts:       results,
	}, nil
}

func probeHost(ip string, port uint16, community string, version gosnmp.SnmpVersion, timeoutSec int, retries int) domain.SnmpSubnetHost {
	start := time.Now()

	params := &gosnmp.GoSNMP{
		Target:             ip,
		Port:               port,
		Transport:          "udp",
		Community:          community,
		Version:            version,
		Timeout:            time.Duration(timeoutSec) * time.Second,
		Retries:            retries,
		ExponentialTimeout: false,
	}

	if err := params.Connect(); err != nil {
		return domain.SnmpSubnetHost{
			IPAddress: ip,
			Reachable: false,
			Error:     "Connection error",
		}
	}
	defer params.Conn.Close()

	// Probe sysDescr (.1.3.6.1.2.1.1.1.0) and sysObjectID (.1.3.6.1.2.1.1.2.0) and sysName (.1.3.6.1.2.1.1.5.0)
	pkt, err := params.Get([]string{
		".1.3.6.1.2.1.1.1.0",
		".1.3.6.1.2.1.1.2.0",
		".1.3.6.1.2.1.1.3.0",
		".1.3.6.1.2.1.1.5.0",
	})

	latencyMs := math.Round(time.Since(start).Seconds()*10000) / 10

	if err != nil || len(pkt.Variables) == 0 {
		return domain.SnmpSubnetHost{
			IPAddress: ip,
			Reachable: false,
			LatencyMs: latencyMs,
			Error:     "No SNMP response (timeout / invalid community)",
		}
	}

	var sysDescr, sysObjectID, sysUptime, sysName string
	for _, v := range pkt.Variables {
		name := strings.TrimPrefix(v.Name, ".")
		val, _ := formatVarbind(v)
		if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance || v.Type == gosnmp.Null {
			continue
		}
		if name == "1.3.6.1.2.1.1.1.0" {
			sysDescr = strings.TrimSpace(val)
		} else if name == "1.3.6.1.2.1.1.2.0" {
			sysObjectID = strings.TrimSpace(val)
		} else if name == "1.3.6.1.2.1.1.3.0" {
			if upTicks, err := strconv.ParseInt(val, 10, 64); err == nil {
				sysUptime = FormatUptime(upTicks)
			} else {
				sysUptime = val
			}
		} else if name == "1.3.6.1.2.1.1.5.0" {
			sysName = strings.TrimSpace(val)
		}
	}

	if sysName == "" {
		sysName = ip
	}

	vendor := MatchVendor(sysObjectID, sysDescr)

	return domain.SnmpSubnetHost{
		IPAddress: ip,
		Reachable: true,
		Hostname:  sysName,
		Vendor:    string(vendor),
		SysDescr:  sysDescr,
		SysUptime: sysUptime,
		LatencyMs: latencyMs,
	}
}
