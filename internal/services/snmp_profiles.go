package services

import (
	"regexp"
	"strings"

	"go-hephaestus/internal/core/domain"
)

type SnmpVendor string

const (
	VendorCisco    SnmpVendor = "Cisco"
	VendorHuawei   SnmpVendor = "Huawei"
	VendorZTE      SnmpVendor = "ZTE"
	VendorAlcatel  SnmpVendor = "Alcatel-Lucent"
	VendorRaisecom SnmpVendor = "Raisecom"
	VendorF5       SnmpVendor = "F5 Networks"
	VendorDahua    SnmpVendor = "Dahua"
	VendorMikroTik SnmpVendor = "MikroTik"
	VendorLinux    SnmpVendor = "Linux / Unix"
	VendorGeneric  SnmpVendor = "Generic MIB-2"
)

type VendorMatcher struct {
	EnterpriseOid string
	SysDescrRegex *regexp.Regexp
	Vendor        SnmpVendor
}

var vendorMatchers = []VendorMatcher{
	{
		EnterpriseOid: "1.3.6.1.4.1.9",
		SysDescrRegex: regexp.MustCompile(`(?i)\bCisco\b|IOS[- ]XE|NX-OS|Catalyst|Nexus`),
		Vendor:        VendorCisco,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.2011",
		SysDescrRegex: regexp.MustCompile(`(?i)\bHuawei\b|\bVRP\b|Quidway|CloudEngine|NetEngine|SmartAX|EchoLife|OptiXstar|\b(CE|S|NE|AR|USG|ME|MA|EA)\d{2,}`),
		Vendor:        VendorHuawei,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.3902",
		SysDescrRegex: regexp.MustCompile(`(?i)\bZTE\b|\bZXROS\b|\bZXR10\b|\bZXA10\b`),
		Vendor:        VendorZTE,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.6486", // Alcatel OmniSwitch
		SysDescrRegex: regexp.MustCompile(`(?i)OmniSwitch|TiMOS|Alcatel|Nokia`),
		Vendor:        VendorAlcatel,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.6527", // Alcatel-Lucent 7750 SR / TiMOS
		SysDescrRegex: regexp.MustCompile(`(?i)OmniSwitch|TiMOS|Alcatel|Nokia`),
		Vendor:        VendorAlcatel,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.8886",
		SysDescrRegex: regexp.MustCompile(`(?i)Raisecom|ISCOM`),
		Vendor:        VendorRaisecom,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.3375",
		SysDescrRegex: regexp.MustCompile(`(?i)\bBIG-IP\b|\bF5\b`),
		Vendor:        VendorF5,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.1004849",
		SysDescrRegex: regexp.MustCompile(`(?i)Dahua|IPC|NVR|DVR`),
		Vendor:        VendorDahua,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.14988",
		SysDescrRegex: regexp.MustCompile(`(?i)RouterOS|MikroTik`),
		Vendor:        VendorMikroTik,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.8072", // Net-SNMP
		SysDescrRegex: regexp.MustCompile(`(?i)Linux|Unix|FreeBSD|Ubuntu|Debian|CentOS|Red Hat|Windows`),
		Vendor:        VendorLinux,
	},
	{
		EnterpriseOid: "1.3.6.1.4.1.2021", // UCD-SNMP
		SysDescrRegex: regexp.MustCompile(`(?i)Linux|Unix|FreeBSD`),
		Vendor:        VendorLinux,
	},
}

// MatchVendor determines device vendor from sysObjectID and sysDescr
func MatchVendor(sysObjectID, sysDescr string) SnmpVendor {
	cleanOID := strings.Trim(strings.TrimSpace(sysObjectID), ".")

	// 1. Try exact enterprise OID prefix match
	if cleanOID != "" {
		for _, m := range vendorMatchers {
			if strings.HasPrefix(cleanOID, m.EnterpriseOid) || strings.HasPrefix(cleanOID, "."+m.EnterpriseOid) {
				return m.Vendor
			}
		}
	}

	// 2. Try sysDescr regex match
	if sysDescr != "" {
		for _, m := range vendorMatchers {
			if m.SysDescrRegex != nil && m.SysDescrRegex.MatchString(sysDescr) {
				return m.Vendor
			}
		}
	}

	return VendorGeneric
}

// AvailableProfiles returns the list of discovery profiles and their descriptions
func AvailableProfiles() []domain.SnmpDiscoveryProfileInfo {
	return []domain.SnmpDiscoveryProfileInfo{
		{
			ID:          "provisioning",
			Name:        "Provisioning (Fast)",
			Description: "Fast production scan for device identity, basic interface speeds, and optics.",
			Modules:     []string{"system", "interfaces", "optical_dom", "environment"},
		},
		{
			ID:          "network",
			Name:        "Network & Interfaces",
			Description: "Full network scan: interface states, 64-bit HC traffic counters, drops, and optical DOM.",
			Modules:     []string{"system", "interfaces", "interface_stats", "optical_dom"},
		},
		{
			ID:          "optical_dom",
			Name:        "Optical DOM / Transceivers",
			Description: "Targeted SFP/XFP/QSFP transceiver diagnostics: RX/TX Power, Temperature, Voltage, and Bias Current.",
			Modules:     []string{"optical_dom", "gpon_optics"},
		},
		{
			ID:          "router_switch",
			Name:        "Router & Switch",
			Description: "Comprehensive switch/router telemetry: Interfaces, Bandwidth usage, Optical DOM, CPU, Memory, and Fans.",
			Modules:     []string{"system", "interfaces", "interface_stats", "optical_dom", "cpu", "memory", "environment"},
		},
		{
			ID:          "system",
			Name:        "System & Resources",
			Description: "Server and OS metrics: CPU cores, RAM, Swap, Storage/Disks, Processes, and Uptime.",
			Modules:     []string{"system", "cpu", "memory", "storage"},
		},
		{
			ID:          "olt",
			Name:        "OLT & Telecom GPON",
			Description: "GPON OLT and ONT telemetry: PON ports, ONT optical receive power, and OLT transceivers.",
			Modules:     []string{"system", "optical_dom", "gpon_optics"},
		},
		{
			ID:          "rectifier",
			Name:        "Rectifier & Power Systems",
			Description: "DC power system scan: AC Phase Voltages, Battery Voltage/Current, System Power, and Load Current.",
			Modules:     []string{"system", "rectifier", "environment"},
		},
		{
			ID:          "printer",
			Name:        "Printer & Consumables",
			Description: "Targeted printer scan: Supplies, Toner/Ink levels, Paper trays, and Page counters.",
			Modules:     []string{"system", "printer"},
		},
		{
			ID:          "full",
			Name:        "Full Comprehensive Scan",
			Description: "Runs all discovery modules across hardware, optics, interfaces, and environmental sensors.",
			Modules:     []string{"system", "interfaces", "interface_stats", "optical_dom", "cpu", "memory", "storage", "environment", "gpon_optics", "rectifier"},
		},
	}
}
