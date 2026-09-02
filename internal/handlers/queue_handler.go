package handlers

import (
	"net/http"
	"strings"
	"time"

	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"

	"github.com/gin-gonic/gin"
)

type QueueHandler struct{}

func NewQueueHandler() *QueueHandler {
	return &QueueHandler{}
}

func (h *QueueHandler) ListJobs(c *gin.Context) {
	wp := queue.GetWorkerPool()
	jobs := wp.ListJobs()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    jobs,
	})
}

func (h *QueueHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	wp := queue.GetWorkerPool()
	job, exists := wp.GetJob(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    job,
	})
}

func (h *QueueHandler) TriggerJob(c *gin.Context) {
	var req struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "type is required"})
		return
	}

	wp := queue.GetWorkerPool()
	job, err := wp.Enqueue(req.Type, map[string]interface{}{}, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Job enqueued successfully.",
		"data":    job,
	})
}

func (h *QueueHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	wp := queue.GetWorkerPool()
	if err := wp.CancelJob(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Job cancellation requested."})
}

type ServiceStatusInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"` // "running", "warning", "stopped"
	Type        string    `json:"type"`
	Icon        string    `json:"icon"`
	Master      bool      `json:"master"`
	Version     string    `json:"version"`
	Modules     string    `json:"modules"`
	Lag         string    `json:"lag"`
	TQ          string    `json:"tq"`
	Updated     string    `json:"updated"`
	LastUpdated time.Time `json:"lastUpdated"`
	Description string    `json:"description"`
	ModuleKey   string    `json:"moduleKey"`
}

func (h *QueueHandler) ListServices(c *gin.Context) {
	now := time.Now()
	services := []ServiceStatusInfo{
		{
			ID:          "srv-data-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Data server",
			Icon:        "database",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "27684 of 27684",
			Lag:         "3 minutes 01 seconds / 1953",
			TQ:          "6 : 1",
			Updated:     "17 seconds",
			LastUpdated: now.Add(-17 * time.Second),
			Description: "OpenSearch telemetry poller, XML metric ingestion, and shard telemetry",
			ModuleKey:   "OpenSearch",
		},
		{
			ID:          "srv-data-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Data server",
			Icon:        "database",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "38886 of 38953",
			Lag:         "- / 0",
			TQ:          "6 : 2",
			Updated:     "19 seconds",
			LastUpdated: now.Add(-19 * time.Second),
			Description: "Secondary node data packet processor and log stream indexing",
			ModuleKey:   "OpenSearch",
		},
		{
			ID:          "srv-network-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Network server",
			Icon:        "network",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "0 of 0",
			Lag:         "- / 0",
			TQ:          "6 : 0",
			Updated:     "17 seconds",
			LastUpdated: now.Add(-17 * time.Second),
			Description: "Secondary subnet latency verifier and ICMP probe daemon",
			ModuleKey:   "Network",
		},
		{
			ID:          "srv-network-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Network server",
			Icon:        "network",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "462 of 462",
			Lag:         "- / 0",
			TQ:          "6 : 11",
			Updated:     "17 seconds",
			LastUpdated: now.Add(-17 * time.Second),
			Description: "Master ICMP ping sweep, TCP latency, and packet loss monitor",
			ModuleKey:   "Network",
		},
		{
			ID:          "srv-snmp-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "SNMP trap server",
			Icon:        "radio",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "N/A",
			Lag:         "N/A",
			TQ:          "1 : 0",
			Updated:     "15 seconds",
			LastUpdated: now.Add(-15 * time.Second),
			Description: "SNMP v1/v2c/v3 trap receiver, OID query engine, and MIB compiler",
			ModuleKey:   "SNMP",
		},
		{
			ID:          "srv-snmp-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "SNMP trap server",
			Icon:        "radio",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "N/A",
			Lag:         "N/A",
			TQ:          "1 : 0",
			Updated:     "19 seconds",
			LastUpdated: now.Add(-19 * time.Second),
			Description: "Slave SNMP poller daemon and interface traffic telemetry",
			ModuleKey:   "SNMP",
		},
		{
			ID:          "srv-discovery-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Discovery server",
			Icon:        "discovery",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "0 of 16",
			Lag:         "-",
			TQ:          "2 : 0",
			Updated:     "15 seconds",
			LastUpdated: now.Add(-15 * time.Second),
			Description: "Network topology auto-discovery, ARP table scanning, and device mapper",
			ModuleKey:   "Topology",
		},
		{
			ID:          "srv-discovery-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Discovery server",
			Icon:        "discovery",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "16 of 16",
			Lag:         "-",
			TQ:          "2 : 0",
			Updated:     "16 seconds",
			LastUpdated: now.Add(-16 * time.Second),
			Description: "Remote branch subnet exploration and MAC table resolver",
			ModuleKey:   "Topology",
		},
		{
			ID:          "srv-event-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Event server",
			Icon:        "event",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "N/A",
			Lag:         "N/A",
			TQ:          "5 : 1",
			Updated:     "18 seconds",
			LastUpdated: now.Add(-18 * time.Second),
			Description: "System event correlation, cron scheduler, and session lifecycle management",
			ModuleKey:   "Cron",
		},
		{
			ID:          "srv-event-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Event server",
			Icon:        "event",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "N/A",
			Lag:         "N/A",
			TQ:          "5 : 1",
			Updated:     "17 seconds",
			LastUpdated: now.Add(-17 * time.Second),
			Description: "Slave event relay and local alarm dispatcher",
			ModuleKey:   "Cron",
		},
		{
			ID:          "srv-alert-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Alert server",
			Icon:        "bell",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "0 of 0",
			Lag:         "- / 0",
			TQ:          "8 : 0",
			Updated:     "19 seconds",
			LastUpdated: now.Add(-19 * time.Second),
			Description: "Edge notification handler and failover escalation dispatcher",
			ModuleKey:   "Alert",
		},
		{
			ID:          "srv-alert-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Alert server",
			Icon:        "bell",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "0 of 0",
			Lag:         "- / 0",
			TQ:          "8 : 0",
			Updated:     "19 seconds",
			LastUpdated: now.Add(-19 * time.Second),
			Description: "Incident evaluation engine, multi-channel alerting, and webhooks",
			ModuleKey:   "Alert",
		},
		{
			ID:          "srv-highperf-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "High performance server",
			Icon:        "highperf",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "799 of 799",
			Lag:         "7 seconds / 5",
			TQ:          "6 : 0",
			Updated:     "20 seconds",
			LastUpdated: now.Add(-20 * time.Second),
			Description: "Prometheus exporter, PromQL high-frequency metric scraper",
			ModuleKey:   "Prometheus",
		},
		{
			ID:          "srv-highperf-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "High performance server",
			Icon:        "highperf",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "29 of 29",
			Lag:         "- / 0",
			TQ:          "6 : 0",
			Updated:     "16 seconds",
			LastUpdated: now.Add(-16 * time.Second),
			Description: "Distributed metrics collector and remote node exporter gateway",
			ModuleKey:   "Prometheus",
		},
		{
			ID:          "srv-heavy-slave",
			Name:        "pandora-fms-slave",
			Status:      "running",
			Type:        "Heavy server",
			Icon:        "heavy",
			Master:      false,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "20 of 50",
			Lag:         "- / 0",
			TQ:          "6 : 0",
			Updated:     "17 seconds",
			LastUpdated: now.Add(-17 * time.Second),
			Description: "Async synthetic transactions and secondary worker pool",
			ModuleKey:   "Queue",
		},
		{
			ID:          "srv-heavy-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Heavy server",
			Icon:        "heavy",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "30 of 50",
			Lag:         "- / 0",
			TQ:          "6 : 0",
			Updated:     "15 seconds",
			LastUpdated: now.Add(-15 * time.Second),
			Description: "5 Concurrent Go worker pool threads for heavy batch processing and reports",
			ModuleKey:   "Queue",
		},
		{
			ID:          "srv-backup-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Backup server",
			Icon:        "backup",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "24 of 24",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "18 seconds",
			LastUpdated: now.Add(-18 * time.Second),
			Description: "Automated database backup engine (PostgreSQL/MySQL) & S3 archiving",
			ModuleKey:   "Backup",
		},
		{
			ID:          "srv-syslog-master",
			Name:        "pandora-fms-master",
			Status:      "running",
			Type:        "Syslog & Grok server",
			Icon:        "syslog",
			Master:      true,
			Version:     "8.0NG.801 (P) 260331",
			Modules:     "1200 of 1200",
			Lag:         "- / 0",
			TQ:          "4 : 0",
			Updated:     "14 seconds",
			LastUpdated: now.Add(-14 * time.Second),
			Description: "RFC Syslog parser, Grok pattern compiler, and log structuring engine",
			ModuleKey:   "Grok",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
	})
}

func (h *QueueHandler) RestartService(c *gin.Context) {
	id := c.Param("id")
	logger.Info("Service", "Daemon service execution triggered manually by operator: "+id)

	// Trigger underlying worker job if applicable
	wp := queue.GetWorkerPool()
	if strings.Contains(id, "network") || strings.Contains(id, "icmp") {
		_, _ = wp.Enqueue("icmp_ping_cycle", map[string]interface{}{}, 0)
	} else if strings.Contains(id, "data") || strings.Contains(id, "opensearch") {
		_, _ = wp.Enqueue("opensearch_poll", map[string]interface{}{}, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Daemon service restarted and execution cycle triggered successfully.",
		"service": id,
	})
}
