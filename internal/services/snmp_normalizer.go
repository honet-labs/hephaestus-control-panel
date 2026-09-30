package services

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Invalid SNMP sentinel values that indicate unsupported, unplugged, or error state
var invalidSnmpSentinels = []float64{
	2147483647.0,
	-2147483648.0,
	4294967295.0,
	999999.0,
	-65535.0,
	65535.0,
	-4000.0, // Sentinels for -40 dBm (no light received)
	-40.0,
}

// IsInvalidSnmpValue checks if a float represents an invalid or sentinel SNMP reading
func IsInvalidSnmpValue(val float64) bool {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return true
	}
	for _, sentinel := range invalidSnmpSentinels {
		if math.Abs(val-sentinel) < 0.0001 {
			return true
		}
	}
	return false
}

// ParseSnmpNumeric extracts a float64 from an SNMP raw string value
func ParseSnmpNumeric(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	// Remove trailing units if any (e.g. "45 C", "1000 bps", "3.3 V")
	parts := strings.Fields(trimmed)
	if len(parts) > 0 {
		trimmed = parts[0]
	}
	// Strip quotes
	trimmed = strings.Trim(trimmed, "\"'")
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// NormalizeOpticalDbm converts various optical power readings into calibrated dBm
func NormalizeOpticalDbm(raw float64, isMicroWatts bool, precision int) (float64, string, string) {
	var dbm float64

	if isMicroWatts {
		if raw <= 0 {
			return -40.0, "dBm", "critical"
		}
		// P(dBm) = 10 * log10(P(uW) / 1000.0)
		dbm = 10.0 * math.Log10(raw/1000.0)
	} else {
		// Huawei and ZTE often represent -18.50 dBm as -1850 (0.01 dBm) or -18500 (0.001 dBm)
		absRaw := math.Abs(raw)
		if absRaw > 500 && absRaw <= 10000 {
			dbm = raw / 100.0
		} else if absRaw > 10000 && absRaw <= 100000 {
			dbm = raw / 1000.0
		} else {
			dbm = raw
		}
	}

	rounded := math.Round(dbm*100) / 100

	// Status assessment for transceiver optical power
	status := "ok"
	if rounded < -30.0 || rounded > 2.0 {
		status = "critical"
	} else if (rounded >= -30.0 && rounded < -25.0) || (rounded > -5.0 && rounded <= 2.0) {
		status = "warning"
	}

	return rounded, "dBm", status
}

// NormalizeTemperature converts raw temp readings (Celsius, deci-Celsius, milli-Celsius)
func NormalizeTemperature(raw float64) (float64, string, string) {
	var deg float64
	absRaw := math.Abs(raw)

	if absRaw >= 10000 {
		deg = raw / 1000.0 // milli-Celsius
	} else if absRaw >= 200 && absRaw < 2000 {
		deg = raw / 10.0 // deci-Celsius (0.1 C)
	} else {
		deg = raw
	}

	rounded := math.Round(deg*10) / 10
	status := "ok"
	if rounded > 75.0 {
		status = "critical"
	} else if rounded > 58.0 {
		status = "warning"
	}

	return rounded, "°C", status
}

// NormalizeVoltage converts raw voltage to Volts
func NormalizeVoltage(raw float64, isMilli bool) (float64, string, string) {
	var v float64
	absRaw := math.Abs(raw)

	if isMilli || absRaw > 1000 {
		v = raw / 1000.0
	} else {
		v = raw
	}

	rounded := math.Round(v*100) / 100
	return rounded, "V", "ok"
}

// NormalizePercentage clamps and cleans percentage values (0-100)
func NormalizePercentage(raw float64) (float64, string, string) {
	val := raw
	if val < 0 {
		val = 0
	} else if val > 100 {
		val = 100
	}
	rounded := math.Round(val*10) / 10
	status := "ok"
	if rounded >= 90.0 {
		status = "critical"
	} else if rounded >= 75.0 {
		status = "warning"
	}
	return rounded, "%", status
}

// FormatSensorName produces clean, enterprise-grade labels for sensors
func FormatSensorName(component string, metric string, unit string) string {
	component = strings.TrimSpace(component)
	metric = strings.TrimSpace(metric)
	unit = strings.TrimSpace(unit)

	var sb strings.Builder
	if component != "" {
		sb.WriteString(component)
		sb.WriteString(" - ")
	}
	sb.WriteString(metric)
	if unit != "" && !strings.Contains(metric, unit) {
		sb.WriteString(" (")
		sb.WriteString(unit)
		sb.WriteString(")")
	}
	return sb.String()
}

// FormatUptime converts TimeTicks (hundredths of a second) to a human-readable duration
func FormatUptime(hundredths int64) string {
	totalSec := hundredths / 100
	days := totalSec / 86400
	hours := (totalSec % 86400) / 3600
	minutes := (totalSec % 3600) / 60
	seconds := totalSec % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
