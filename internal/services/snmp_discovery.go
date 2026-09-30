package services

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"github.com/gosnmp/gosnmp"
)

type SnmpDiscoveryEngine struct {
	snmpService *SnmpService
}

func NewSnmpDiscoveryEngine(s *SnmpService) *SnmpDiscoveryEngine {
	return &SnmpDiscoveryEngine{snmpService: s}
}

// SnmpTableWalker is a helper for walking indexed tables
type SnmpTableWalker struct {
	params *gosnmp.GoSNMP
}

func (w *SnmpTableWalker) WalkIndexed(rootOid string) map[string]string {
	results := make(map[string]string)
	cleanRoot := strings.Trim(rootOid, ".")
	targetOid := "." + cleanRoot

	walkFn := func(pdu gosnmp.SnmpPDU) error {
		pduOid := strings.Trim(strings.TrimPrefix(pdu.Name, "."), ".")
		// Extract index suffix relative to cleanRoot
		if strings.HasPrefix(pduOid, cleanRoot+".") {
			index := strings.TrimPrefix(pduOid, cleanRoot+".")
			val, _ := formatVarbind(pdu)
			results[index] = val
		}
		if len(results) >= 2000 {
			return fmt.Errorf("walk_limit")
		}
		return nil
	}

	var err error
	if w.params.Version == gosnmp.Version2c {
		err = w.params.BulkWalk(targetOid, walkFn)
		if err != nil || len(results) == 0 {
			_ = w.params.Walk(targetOid, walkFn)
		}
	} else {
		_ = w.params.Walk(targetOid, walkFn)
	}

	return results
}

func (w *SnmpTableWalker) GetScalar(oid string) string {
	clean := "." + strings.Trim(oid, ".")
	pkt, err := w.params.Get([]string{clean})
	if err != nil || len(pkt.Variables) == 0 {
		return ""
	}
	v := pkt.Variables[0]
	if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance || v.Type == gosnmp.Null {
		return ""
	}
	val, _ := formatVarbind(v)
	return strings.TrimSpace(val)
}

// Discover executes device profile-driven telemetry and sensor discovery
func (e *SnmpDiscoveryEngine) Discover(host string, port uint16, version string, community string, profile string, timeoutSec int, retries int) (*domain.SnmpDiscoveryResult, error) {
	if port == 0 {
		port = 161
	}
	if community == "" {
		community = "public"
	}
	if timeoutSec <= 0 {
		timeoutSec = 6
	}
	if retries <= 0 {
		retries = 2
	}
	if profile == "" {
		profile = "provisioning"
	}

	snmpVersion := gosnmp.Version2c
	if version == "v1" || version == "1" {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:             host,
		Port:               port,
		Transport:          "udp",
		Community:          community,
		Version:            snmpVersion,
		Timeout:            time.Duration(timeoutSec) * time.Second,
		Retries:            retries,
		ExponentialTimeout: true,
		MaxRepetitions:     40,
		MaxOids:            gosnmp.Default.MaxOids,
	}

	startedAt := time.Now()
	if err := params.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed to %s:%d: %w", host, port, err)
	}
	defer params.Conn.Close()

	walker := &SnmpTableWalker{params: params}

	// 1. Resolve Device Identity (MIB-2 System)
	sysDescr := walker.GetScalar("1.3.6.1.2.1.1.1.0")
	sysObjectID := walker.GetScalar("1.3.6.1.2.1.1.2.0")
	sysUptimeRaw := walker.GetScalar("1.3.6.1.2.1.1.3.0")
	sysContact := walker.GetScalar("1.3.6.1.2.1.1.4.0")
	sysName := walker.GetScalar("1.3.6.1.2.1.1.5.0")
	sysLocation := walker.GetScalar("1.3.6.1.2.1.1.6.0")

	// Fallback probe for embedded devices (e.g. Dahua CCTV)
	if sysDescr == "" && sysObjectID == "" {
		dahuaModel := walker.GetScalar("1.3.6.1.4.1.1004849.2.1.2.6.0")
		if dahuaModel != "" {
			sysDescr = "Dahua CCTV Device: " + dahuaModel
			sysObjectID = "1.3.6.1.4.1.1004849"
		} else {
			return nil, fmt.Errorf("SNMP agent at %s:%d is unreachable or rejected community string '%s'", host, port, community)
		}
	}

	if sysName == "" {
		sysName = host
	}

	// Format uptime
	humanUptime := sysUptimeRaw
	if upTicks, err := strconv.ParseInt(sysUptimeRaw, 10, 64); err == nil {
		humanUptime = FormatUptime(upTicks)
	}

	vendor := MatchVendor(sysObjectID, sysDescr)

	device := domain.SnmpDeviceIdentity{
		IPAddress:   host,
		Hostname:    sysName,
		Vendor:      string(vendor),
		SysObjectID: sysObjectID,
		SysDescr:    sysDescr,
		SysUptime:   humanUptime,
		SysContact:  sysContact,
		SysLocation: sysLocation,
		Version:     version,
		Port:        int(port),
		Status:      "online",
	}

	// 2. Determine modules to run based on profile
	modulesToRun := resolveModulesForProfile(profile)

	sensors := make([]domain.SnmpDiscoveredSensor, 0, 100)

	// Context map for interface names: ifIndex -> ifName
	ifNamesMap := make(map[string]string)
	ifAliasMap := make(map[string]string)

	// Pre-load interface names if needed
	if shouldRunModule(modulesToRun, "interfaces", "interface_stats", "optical_dom", "router_switch") {
		ifNamesMap = walker.WalkIndexed("1.3.6.1.2.1.31.1.1.1.1") // ifName
		if len(ifNamesMap) == 0 {
			ifNamesMap = walker.WalkIndexed("1.3.6.1.2.1.2.2.1.2") // fallback ifDescr
		}
		ifAliasMap = walker.WalkIndexed("1.3.6.1.2.1.31.1.1.1.18") // ifAlias
	}

	// 3. Module: Interfaces (IF-MIB)
	if shouldRunModule(modulesToRun, "interfaces") {
		sensors = append(sensors, discoverInterfaces(walker, ifNamesMap, ifAliasMap)...)
	}

	// 4. Module: Interface Stats & Traffic
	if shouldRunModule(modulesToRun, "interface_stats") {
		sensors = append(sensors, discoverInterfaceStats(walker, ifNamesMap)...)
	}

	// 5. Module: Optical DOM Telemetry (Transceivers)
	if shouldRunModule(modulesToRun, "optical_dom", "gpon_optics") {
		sensors = append(sensors, discoverOpticalDom(walker, vendor, ifNamesMap)...)
	}

	// 6. Module: CPU & Memory
	if shouldRunModule(modulesToRun, "cpu", "memory") {
		sensors = append(sensors, discoverCpuAndMemory(walker, vendor)...)
	}

	// 7. Module: Storage & Disks (HOST-RESOURCES-MIB)
	if shouldRunModule(modulesToRun, "storage") {
		sensors = append(sensors, discoverStorage(walker)...)
	}

	// 8. Module: Environmental & Power
	if shouldRunModule(modulesToRun, "environment") {
		sensors = append(sensors, discoverEnvironmental(walker, vendor)...)
	}

	// 9. Module: GPON & Telecom OLT Optics
	if shouldRunModule(modulesToRun, "gpon_optics") && vendor == VendorHuawei {
		sensors = append(sensors, discoverHuaweiGpon(walker, ifNamesMap)...)
	}

	// 10. Module: Rectifier & Power Systems
	if shouldRunModule(modulesToRun, "rectifier") {
		sensors = append(sensors, discoverRectifier(walker)...)
	}

	durationSec := math.Round(time.Since(startedAt).Seconds()*1000) / 1000

	return &domain.SnmpDiscoveryResult{
		Device:      device,
		Vendor:      string(vendor),
		Profile:     profile,
		Sensors:     sensors,
		SensorCount: len(sensors),
		DurationSec: durationSec,
	}, nil
}

func resolveModulesForProfile(profile string) []string {
	for _, p := range AvailableProfiles() {
		if strings.EqualFold(p.ID, profile) {
			return p.Modules
		}
	}
	// Default to provisioning
	return []string{"system", "interfaces", "optical_dom", "environment"}
}

func shouldRunModule(activeModules []string, targetModules ...string) bool {
	for _, a := range activeModules {
		for _, t := range targetModules {
			if strings.EqualFold(a, t) {
				return true
			}
		}
	}
	return false
}

// discoverInterfaces scans IF-MIB for port operational states and speeds
func discoverInterfaces(w *SnmpTableWalker, ifNames map[string]string, ifAliases map[string]string) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 32)
	operStatuses := w.WalkIndexed("1.3.6.1.2.1.2.2.1.8") // ifOperStatus
	adminStatuses := w.WalkIndexed("1.3.6.1.2.1.2.2.1.7") // ifAdminStatus
	highSpeeds := w.WalkIndexed("1.3.6.1.2.1.31.1.1.1.15") // ifHighSpeed (Mbps)

	for idx, operVal := range operStatuses {
		ifIdx, _ := strconv.Atoi(idx)
		ifName := ifNames[idx]
		if ifName == "" {
			ifName = "Interface " + idx
		}
		alias := strings.TrimSpace(ifAliases[idx])

		// Interpret status: 1 = up, 2 = down, 3 = testing
		statusLabel := "down"
		statusState := "critical"
		if operVal == "1" || strings.EqualFold(operVal, "up") {
			statusLabel = "up"
			statusState = "ok"
		} else if operVal == "3" || strings.EqualFold(operVal, "testing") {
			statusLabel = "testing"
			statusState = "warning"
		}

		adminLabel := "up"
		if adminStatuses[idx] == "2" {
			adminLabel = "disabled"
		}

		// Oper Status Sensor
		sensors = append(sensors, domain.SnmpDiscoveredSensor{
			SensorClass:    "interface",
			SensorName:     FormatSensorName(ifName, "Operational Status", ""),
			SensorType:     "oper_status",
			InterfaceIndex: &ifIdx,
			InterfaceName:  ifName,
			OID:            "1.3.6.1.2.1.2.2.1.8." + idx,
			RawValue:       operVal,
			Unit:           "",
			Status:         statusState,
			Metadata: map[string]interface{}{
				"alias":        alias,
				"admin_status": adminLabel,
				"oper_status":  statusLabel,
				"display":      strings.ToUpper(statusLabel),
			},
		})

		// Speed Sensor (if present)
		if spdStr, ok := highSpeeds[idx]; ok && spdStr != "" && spdStr != "0" {
			if spdVal, err := strconv.ParseFloat(spdStr, 64); err == nil && spdVal > 0 {
				var speedDisplay string
				if spdVal >= 1000 {
					speedDisplay = fmt.Sprintf("%.1f Gbps", spdVal/1000.0)
				} else {
					speedDisplay = fmt.Sprintf("%.0f Mbps", spdVal)
				}
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "interface",
					SensorName:      FormatSensorName(ifName, "Negotiated Speed", ""),
					SensorType:      "speed",
					InterfaceIndex:  &ifIdx,
					InterfaceName:   ifName,
					OID:             "1.3.6.1.2.1.31.1.1.1.15." + idx,
					RawValue:        spdStr,
					NormalizedValue: &spdVal,
					Unit:            "Mbps",
					Status:          "ok",
					Metadata: map[string]interface{}{
						"display": speedDisplay,
					},
				})
			}
		}
	}

	return sensors
}

// discoverInterfaceStats collects 64-bit HC In/Out Octets and Errors
func discoverInterfaceStats(w *SnmpTableWalker, ifNames map[string]string) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 32)
	inOctets := w.WalkIndexed("1.3.6.1.2.1.31.1.1.1.6") // ifHCInOctets
	if len(inOctets) == 0 {
		inOctets = w.WalkIndexed("1.3.6.1.2.1.2.2.1.10") // fallback 32-bit
	}
	outOctets := w.WalkIndexed("1.3.6.1.2.1.31.1.1.1.10") // ifHCOutOctets
	if len(outOctets) == 0 {
		outOctets = w.WalkIndexed("1.3.6.1.2.1.2.2.1.16") // fallback 32-bit
	}
	inErrors := w.WalkIndexed("1.3.6.1.2.1.2.2.1.14")
	outErrors := w.WalkIndexed("1.3.6.1.2.1.2.2.1.20")

	for idx, inBytes := range inOctets {
		ifIdx, _ := strconv.Atoi(idx)
		ifName := ifNames[idx]
		if ifName == "" {
			ifName = "Interface " + idx
		}

		if val, ok := ParseSnmpNumeric(inBytes); ok {
			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "interface",
				SensorName:      FormatSensorName(ifName, "Inbound Traffic", "Bytes"),
				SensorType:      "in_octets",
				InterfaceIndex:  &ifIdx,
				InterfaceName:   ifName,
				OID:             "1.3.6.1.2.1.31.1.1.1.6." + idx,
				RawValue:        inBytes,
				NormalizedValue: &val,
				Unit:            "Bytes",
				Status:          "ok",
			})
		}

		if outBytes, ok := outOctets[idx]; ok {
			if val, ok := ParseSnmpNumeric(outBytes); ok {
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "interface",
					SensorName:      FormatSensorName(ifName, "Outbound Traffic", "Bytes"),
					SensorType:      "out_octets",
					InterfaceIndex:  &ifIdx,
					InterfaceName:   ifName,
					OID:             "1.3.6.1.2.1.31.1.1.1.10." + idx,
					RawValue:        outBytes,
					NormalizedValue: &val,
					Unit:            "Bytes",
					Status:          "ok",
				})
			}
		}

		if inErr, ok := inErrors[idx]; ok && inErr != "0" {
			if val, ok := ParseSnmpNumeric(inErr); ok && val > 0 {
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "interface",
					SensorName:      FormatSensorName(ifName, "Inbound Errors", "Pkts"),
					SensorType:      "in_errors",
					InterfaceIndex:  &ifIdx,
					InterfaceName:   ifName,
					OID:             "1.3.6.1.2.1.2.2.1.14." + idx,
					RawValue:        inErr,
					NormalizedValue: &val,
					Unit:            "packets",
					Status:          "warning",
				})
			}
		}

		if outErr, ok := outErrors[idx]; ok && outErr != "0" {
			if val, ok := ParseSnmpNumeric(outErr); ok && val > 0 {
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "interface",
					SensorName:      FormatSensorName(ifName, "Outbound Errors", "Pkts"),
					SensorType:      "out_errors",
					InterfaceIndex:  &ifIdx,
					InterfaceName:   ifName,
					OID:             "1.3.6.1.2.1.2.2.1.20." + idx,
					RawValue:        outErr,
					NormalizedValue: &val,
					Unit:            "packets",
					Status:          "warning",
				})
			}
		}
	}

	return sensors
}

// discoverOpticalDom scans SFP transceiver optical diagnostic metrics
func discoverOpticalDom(w *SnmpTableWalker, vendor SnmpVendor, ifNames map[string]string) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 32)

	// 1. Huawei Optical Transceivers (HUAWEI-ENTITY-EXTENT-MIB)
	if vendor == VendorHuawei {
		// RX Power dBm (.33) or uWatts (.9)
		rxDbm := w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.3.1.33")
		if len(rxDbm) == 0 {
			rxDbm = w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.6") // Legacy optical Rx
		}
		txDbm := w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.3.1.32")
		if len(txDbm) == 0 {
			txDbm = w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.7") // Legacy optical Tx
		}
		opticalTemp := w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.3.1.5")
		opticalVolt := w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.3.1.6")
		opticalBias := w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.3.1.7")

		for idx, rawRx := range rxDbm {
			val, ok := ParseSnmpNumeric(rawRx)
			if !ok || IsInvalidSnmpValue(val) {
				continue
			}
			normVal, unit, status := NormalizeOpticalDbm(val, false, 2)
			ifName := resolveTransceiverName(idx, ifNames)

			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "optical_dom",
				SensorName:      FormatSensorName(ifName, "RX Optical Power", unit),
				SensorType:      "rx_power",
				InterfaceName:   ifName,
				OID:             "1.3.6.1.4.1.2011.5.25.31.1.1.3.1.33." + idx,
				RawValue:        rawRx,
				NormalizedValue: &normVal,
				Unit:            unit,
				Status:          status,
			})

			// Matching TX Power
			if rawTx, hasTx := txDbm[idx]; hasTx {
				if txVal, ok := ParseSnmpNumeric(rawTx); ok && !IsInvalidSnmpValue(txVal) {
					normTx, unitTx, statusTx := NormalizeOpticalDbm(txVal, false, 2)
					sensors = append(sensors, domain.SnmpDiscoveredSensor{
						SensorClass:     "optical_dom",
						SensorName:      FormatSensorName(ifName, "TX Optical Power", unitTx),
						SensorType:      "tx_power",
						InterfaceName:   ifName,
						OID:             "1.3.6.1.4.1.2011.5.25.31.1.1.3.1.32." + idx,
						RawValue:        rawTx,
						NormalizedValue: &normTx,
						Unit:            unitTx,
						Status:          statusTx,
					})
				}
			}

			// Transceiver Temperature
			if rawT, hasT := opticalTemp[idx]; hasT {
				if tVal, ok := ParseSnmpNumeric(rawT); ok && !IsInvalidSnmpValue(tVal) {
					normT, unitT, statusT := NormalizeTemperature(tVal)
					sensors = append(sensors, domain.SnmpDiscoveredSensor{
						SensorClass:     "temperature",
						SensorName:      FormatSensorName(ifName, "Transceiver Temperature", unitT),
						SensorType:      "temperature",
						InterfaceName:   ifName,
						OID:             "1.3.6.1.4.1.2011.5.25.31.1.1.3.1.5." + idx,
						RawValue:        rawT,
						NormalizedValue: &normT,
						Unit:            unitT,
						Status:          statusT,
					})
				}
			}

			// Transceiver Voltage
			if rawV, hasV := opticalVolt[idx]; hasV {
				if vVal, ok := ParseSnmpNumeric(rawV); ok && !IsInvalidSnmpValue(vVal) {
					normV, unitV, statusV := NormalizeVoltage(vVal, true)
					sensors = append(sensors, domain.SnmpDiscoveredSensor{
						SensorClass:     "voltage",
						SensorName:      FormatSensorName(ifName, "Transceiver Voltage", unitV),
						SensorType:      "voltage",
						InterfaceName:   ifName,
						OID:             "1.3.6.1.4.1.2011.5.25.31.1.1.3.1.6." + idx,
						RawValue:        rawV,
						NormalizedValue: &normV,
						Unit:            unitV,
						Status:          statusV,
					})
				}
			}

			// Transceiver TX Bias Current
			if rawB, hasB := opticalBias[idx]; hasB {
				if bVal, ok := ParseSnmpNumeric(rawB); ok && !IsInvalidSnmpValue(bVal) {
					rounded := math.Round(bVal*10) / 10
					sensors = append(sensors, domain.SnmpDiscoveredSensor{
						SensorClass:     "optical_dom",
						SensorName:      FormatSensorName(ifName, "TX Bias Current", "mA"),
						SensorType:      "tx_bias",
						InterfaceName:   ifName,
						OID:             "1.3.6.1.4.1.2011.5.25.31.1.1.3.1.7." + idx,
						RawValue:        rawB,
						NormalizedValue: &rounded,
						Unit:            "mA",
						Status:          "ok",
					})
				}
			}
		}
	}

	// 2. Standard ENTITY-SENSOR-MIB (Cisco, Arista, Juniper, Generic)
	entitySensorValues := w.WalkIndexed("1.3.6.1.2.1.99.1.1.1.4") // entSensorValue
	if len(entitySensorValues) > 0 {
		entitySensorTypes := w.WalkIndexed("1.3.6.1.2.1.99.1.1.1.1") // entSensorType
		entNames := w.WalkIndexed("1.3.6.1.2.1.47.1.1.1.1.7")        // entPhysicalName

		for idx, rawVal := range entitySensorValues {
			numVal, ok := ParseSnmpNumeric(rawVal)
			if !ok || IsInvalidSnmpValue(numVal) {
				continue
			}
			sType := entitySensorTypes[idx]
			entName := entNames[idx]
			if entName == "" {
				entName = "Sensor " + idx
			}

			// Type 14 = dBm (optical power)
			if sType == "14" || strings.Contains(strings.ToLower(entName), "rx") || strings.Contains(strings.ToLower(entName), "tx") {
				normVal, unit, status := NormalizeOpticalDbm(numVal, false, 2)
				metric := "Optical Power"
				if strings.Contains(strings.ToLower(entName), "rx") {
					metric = "RX Optical Power"
				} else if strings.Contains(strings.ToLower(entName), "tx") {
					metric = "TX Optical Power"
				}
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "optical_dom",
					SensorName:      FormatSensorName(entName, metric, unit),
					SensorType:      "optical_power",
					OID:             "1.3.6.1.2.1.99.1.1.1.4." + idx,
					RawValue:        rawVal,
					NormalizedValue: &normVal,
					Unit:            unit,
					Status:          status,
				})
			} else if sType == "8" || strings.Contains(strings.ToLower(entName), "temp") { // Type 8 = Celsius
				normVal, unit, status := NormalizeTemperature(numVal)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "temperature",
					SensorName:      FormatSensorName(entName, "Temperature", unit),
					SensorType:      "temperature",
					OID:             "1.3.6.1.2.1.99.1.1.1.4." + idx,
					RawValue:        rawVal,
					NormalizedValue: &normVal,
					Unit:            unit,
					Status:          status,
				})
			} else if sType == "6" || strings.Contains(strings.ToLower(entName), "volt") { // Type 6 = Volts
				normVal, unit, status := NormalizeVoltage(numVal, false)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "voltage",
					SensorName:      FormatSensorName(entName, "Voltage", unit),
					SensorType:      "voltage",
					OID:             "1.3.6.1.2.1.99.1.1.1.4." + idx,
					RawValue:        rawVal,
					NormalizedValue: &normVal,
					Unit:            unit,
					Status:          status,
				})
			}
		}
	}

	return sensors
}

func resolveTransceiverName(index string, ifNames map[string]string) string {
	if name, ok := ifNames[index]; ok && name != "" {
		return name
	}
	return "Transceiver " + index
}

// discoverCpuAndMemory scans CPU utilization and Memory pools
func discoverCpuAndMemory(w *SnmpTableWalker, vendor SnmpVendor) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 16)

	// 1. Standard HOST-RESOURCES-MIB Processors (per core)
	procLoads := w.WalkIndexed("1.3.6.1.2.1.25.3.3.1.2") // hrProcessorLoad
	for idx, loadStr := range procLoads {
		if val, ok := ParseSnmpNumeric(loadStr); ok && !IsInvalidSnmpValue(val) {
			norm, unit, status := NormalizePercentage(val)
			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "processor",
				SensorName:      fmt.Sprintf("CPU Core %s Utilization (%%)", idx),
				SensorType:      "cpu_usage",
				OID:             "1.3.6.1.2.1.25.3.3.1.2." + idx,
				RawValue:        loadStr,
				NormalizedValue: &norm,
				Unit:            unit,
				Status:          status,
			})
		}
	}

	// 2. Cisco CPU & Memory
	if vendor == VendorCisco {
		ciscoCpu := w.WalkIndexed("1.3.6.1.4.1.9.9.109.1.1.1.1.8") // cpmCPUTotal5minRev
		for idx, cpuStr := range ciscoCpu {
			if val, ok := ParseSnmpNumeric(cpuStr); ok && !IsInvalidSnmpValue(val) {
				norm, unit, status := NormalizePercentage(val)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "processor",
					SensorName:      fmt.Sprintf("Cisco CPU %s (5-min avg)", idx),
					SensorType:      "cpu_usage",
					OID:             "1.3.6.1.4.1.9.9.109.1.1.1.1.8." + idx,
					RawValue:        cpuStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}
		memUsed := w.WalkIndexed("1.3.6.1.4.1.9.9.48.1.1.1.5")
		memFree := w.WalkIndexed("1.3.6.1.4.1.9.9.48.1.1.1.6")
		memNames := w.WalkIndexed("1.3.6.1.4.1.9.9.48.1.1.1.2")
		for idx, usedStr := range memUsed {
			freeStr := memFree[idx]
			uVal, okU := ParseSnmpNumeric(usedStr)
			fVal, okF := ParseSnmpNumeric(freeStr)
			if okU && okF && (uVal+fVal) > 0 {
				pct := (uVal / (uVal + fVal)) * 100.0
				norm, unit, status := NormalizePercentage(pct)
				name := memNames[idx]
				if name == "" {
					name = "Pool " + idx
				}
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "memory",
					SensorName:      fmt.Sprintf("Cisco Memory %s (%%)", name),
					SensorType:      "memory_usage",
					OID:             "1.3.6.1.4.1.9.9.48.1.1.1.5." + idx,
					RawValue:        usedStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}
	}

	// 3. Huawei CPU & Memory
	if vendor == VendorHuawei {
		hwCpu := w.WalkIndexed("1.3.6.1.4.1.2011.6.3.4.1.2") // hwCpuDevDuty
		if len(hwCpu) == 0 {
			hwCpu = w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5") // hwEntityCpuUsage
		}
		for idx, cpuStr := range hwCpu {
			if val, ok := ParseSnmpNumeric(cpuStr); ok && !IsInvalidSnmpValue(val) {
				norm, unit, status := NormalizePercentage(val)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "processor",
					SensorName:      fmt.Sprintf("Huawei CPU %s (%%)", idx),
					SensorType:      "cpu_usage",
					OID:             "1.3.6.1.4.1.2011.6.3.4.1.2." + idx,
					RawValue:        cpuStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}

		hwMem := w.WalkIndexed("1.3.6.1.4.1.2011.6.3.5.1.1.2") // hwMemoryDevDuty
		if len(hwMem) == 0 {
			hwMem = w.WalkIndexed("1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7") // hwEntityMemUsage
		}
		for idx, memStr := range hwMem {
			if val, ok := ParseSnmpNumeric(memStr); ok && !IsInvalidSnmpValue(val) {
				norm, unit, status := NormalizePercentage(val)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "memory",
					SensorName:      fmt.Sprintf("Huawei Memory %s (%%)", idx),
					SensorType:      "memory_usage",
					OID:             "1.3.6.1.4.1.2011.6.3.5.1.1.2." + idx,
					RawValue:        memStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}
	}

	// 4. Linux / UCD-SNMP
	if vendor == VendorLinux || len(sensors) == 0 {
		idleStr := w.GetScalar("1.3.6.1.4.1.2021.11.11.0") // ssCpuIdle
		if idleVal, ok := ParseSnmpNumeric(idleStr); ok && !IsInvalidSnmpValue(idleVal) {
			usage := 100.0 - idleVal
			norm, unit, status := NormalizePercentage(usage)
			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "processor",
				SensorName:      "System CPU Utilization (%)",
				SensorType:      "cpu_usage",
				OID:             "1.3.6.1.4.1.2021.11.11.0",
				RawValue:        idleStr,
				NormalizedValue: &norm,
				Unit:            unit,
				Status:          status,
			})
		}
	}

	return sensors
}

// discoverStorage scans storage pools and fixed disks
func discoverStorage(w *SnmpTableWalker) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 16)
	storageDescr := w.WalkIndexed("1.3.6.1.2.1.25.2.3.1.3")
	storageUnits := w.WalkIndexed("1.3.6.1.2.1.25.2.3.1.4")
	storageSize := w.WalkIndexed("1.3.6.1.2.1.25.2.3.1.5")
	storageUsed := w.WalkIndexed("1.3.6.1.2.1.25.2.3.1.6")

	for idx, descr := range storageDescr {
		szVal, okSz := ParseSnmpNumeric(storageSize[idx])
		uVal, okU := ParseSnmpNumeric(storageUsed[idx])
		allocUnits, _ := ParseSnmpNumeric(storageUnits[idx])
		if allocUnits <= 0 {
			allocUnits = 1
		}

		if okSz && okU && szVal > 0 {
			pct := (uVal / szVal) * 100.0
			norm, unit, status := NormalizePercentage(pct)
			usedGb := (uVal * allocUnits) / (1024 * 1024 * 1024)
			totalGb := (szVal * allocUnits) / (1024 * 1024 * 1024)

			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "storage",
				SensorName:      FormatSensorName(descr, "Usage", unit),
				SensorType:      "storage_usage",
				OID:             "1.3.6.1.2.1.25.2.3.1.6." + idx,
				RawValue:        storageUsed[idx],
				NormalizedValue: &norm,
				Unit:            unit,
				Status:          status,
				Metadata: map[string]interface{}{
					"used_gb":  math.Round(usedGb*10) / 10,
					"total_gb": math.Round(totalGb*10) / 10,
				},
			})
		}
	}

	return sensors
}

// discoverEnvironmental scans chassis temperatures, fans, and power supplies
func discoverEnvironmental(w *SnmpTableWalker, vendor SnmpVendor) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 16)

	if vendor == VendorCisco {
		// Cisco ENVMON Temperature
		ciscoTemp := w.WalkIndexed("1.3.6.1.4.1.9.9.13.1.3.1.3")
		for idx, tStr := range ciscoTemp {
			if val, ok := ParseSnmpNumeric(tStr); ok && !IsInvalidSnmpValue(val) {
				norm, unit, status := NormalizeTemperature(val)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "temperature",
					SensorName:      fmt.Sprintf("Cisco Chassis Sensor %s Temperature (%s)", idx, unit),
					SensorType:      "temperature",
					OID:             "1.3.6.1.4.1.9.9.13.1.3.1.3." + idx,
					RawValue:        tStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}
		// Cisco ENVMON Voltage
		ciscoVolt := w.WalkIndexed("1.3.6.1.4.1.9.9.13.1.2.1.3")
		for idx, vStr := range ciscoVolt {
			if val, ok := ParseSnmpNumeric(vStr); ok && !IsInvalidSnmpValue(val) {
				norm, unit, status := NormalizeVoltage(val, true)
				sensors = append(sensors, domain.SnmpDiscoveredSensor{
					SensorClass:     "voltage",
					SensorName:      fmt.Sprintf("Cisco Power Supply %s Voltage (V)", idx),
					SensorType:      "voltage",
					OID:             "1.3.6.1.4.1.9.9.13.1.2.1.3." + idx,
					RawValue:        vStr,
					NormalizedValue: &norm,
					Unit:            unit,
					Status:          status,
				})
			}
		}
	}

	return sensors
}

// discoverHuaweiGpon discovers GPON OLT and ONT optics
func discoverHuaweiGpon(w *SnmpTableWalker, ifNames map[string]string) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 32)

	// OLT PON Optical RX Power
	oltRx := w.WalkIndexed("1.3.6.1.4.1.2011.6.128.1.1.2.23.1.5")
	for idx, valStr := range oltRx {
		if val, ok := ParseSnmpNumeric(valStr); ok && !IsInvalidSnmpValue(val) {
			norm, unit, status := NormalizeOpticalDbm(val, false, 2)
			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "gpon",
				SensorName:      fmt.Sprintf("GPON OLT PON %s - RX Optical Power (%s)", idx, unit),
				SensorType:      "rx_power",
				OID:             "1.3.6.1.4.1.2011.6.128.1.1.2.23.1.5." + idx,
				RawValue:        valStr,
				NormalizedValue: &norm,
				Unit:            unit,
				Status:          status,
			})
		}
	}

	// ONT Optical RX Power
	ontRx := w.WalkIndexed("1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4")
	for idx, valStr := range ontRx {
		if val, ok := ParseSnmpNumeric(valStr); ok && !IsInvalidSnmpValue(val) {
			norm, unit, status := NormalizeOpticalDbm(val, false, 2)
			sensors = append(sensors, domain.SnmpDiscoveredSensor{
				SensorClass:     "gpon",
				SensorName:      fmt.Sprintf("GPON ONT %s - RX Optical Power (%s)", idx, unit),
				SensorType:      "rx_power",
				OID:             "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4." + idx,
				RawValue:        valStr,
				NormalizedValue: &norm,
				Unit:            unit,
				Status:          status,
			})
		}
	}

	return sensors
}

// discoverRectifier discovers rectifier DC power readings
func discoverRectifier(w *SnmpTableWalker) []domain.SnmpDiscoveredSensor {
	sensors := make([]domain.SnmpDiscoveredSensor, 0, 8)
	// Common UPS / Rectifier OIDs (RFC 1628 UPS-MIB)
	battVoltStr := w.GetScalar("1.3.6.1.2.1.33.1.2.5.0") // upsBatteryVoltage
	if val, ok := ParseSnmpNumeric(battVoltStr); ok && !IsInvalidSnmpValue(val) {
		norm, unit, status := NormalizeVoltage(val, false)
		sensors = append(sensors, domain.SnmpDiscoveredSensor{
			SensorClass:     "rectifier",
			SensorName:      "Battery Bank DC Voltage (V)",
			SensorType:      "voltage",
			OID:             "1.3.6.1.2.1.33.1.2.5.0",
			RawValue:        battVoltStr,
			NormalizedValue: &norm,
			Unit:            unit,
			Status:          status,
		})
	}

	battTempStr := w.GetScalar("1.3.6.1.2.1.33.1.2.7.0") // upsBatteryTemperature
	if val, ok := ParseSnmpNumeric(battTempStr); ok && !IsInvalidSnmpValue(val) {
		norm, unit, status := NormalizeTemperature(val)
		sensors = append(sensors, domain.SnmpDiscoveredSensor{
			SensorClass:     "rectifier",
			SensorName:      "Battery Bank Temperature (°C)",
			SensorType:      "temperature",
			OID:             "1.3.6.1.2.1.33.1.2.7.0",
			RawValue:        battTempStr,
			NormalizedValue: &norm,
			Unit:            unit,
			Status:          status,
		})
	}

	return sensors
}
