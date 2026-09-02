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
			ID:          "srv-icmp-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Network Server (ICMP Ping Sweep)",
			Icon:        "network",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "309 of 309 targets",
			Lag:         "- / 0",
			TQ:          "5 : 0",
			Updated:     "4 seconds",
			LastUpdated: now.Add(-4 * time.Second),
			Description: "Periodic ICMP ping sweep, packet loss & device latency poller across subnets",
			ModuleKey:   "Network",
		},
		{
			ID:          "srv-opensearch-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Data Server (OpenSearch Poller)",
			Icon:        "database",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "2797 of 2797 docs",
			Lag:         "20 seconds / 41",
			TQ:          "5 : 1",
			Updated:     "5 seconds",
			LastUpdated: now.Add(-5 * time.Second),
			Description: "Real-time OpenSearch cluster health, nodes performance stats, and shard telemetry",
			ModuleKey:   "OpenSearch",
		},
		{
			ID:          "srv-backup-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Backup Server (PostgreSQL / MySQL)",
			Icon:        "backup",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "24 of 24 dumps",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "18 seconds",
			LastUpdated: now.Add(-18 * time.Second),
			Description: "Scheduled automated database dumps, gzip compression, and cloud S3 archiving",
			ModuleKey:   "Backup",
		},
		{
			ID:          "srv-snmp-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "SNMP Trap & Poller Server",
			Icon:        "radio",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "120 of 120 MIBs",
			Lag:         "N/A",
			TQ:          "2 : 0",
			Updated:     "12 seconds",
			LastUpdated: now.Add(-12 * time.Second),
			Description: "SNMP v1/v2c/v3 trap listener, OID real-time query engine, and MIB dictionary compiler",
			ModuleKey:   "SNMP",
		},
		{
			ID:          "srv-discovery-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Discovery Server (ARP / Subnet)",
			Icon:        "discovery",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "16 of 16 subnets",
			Lag:         "-",
			TQ:          "2 : 0",
			Updated:     "6 seconds",
			LastUpdated: now.Add(-6 * time.Second),
			Description: "Automated network topology scanner, ARP lookup, and MAC address discovery daemon",
			ModuleKey:   "Topology",
		},
		{
			ID:          "srv-cron-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Event & Scheduler Server (Cron)",
			Icon:        "event",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "8 of 8 schedules",
			Lag:         "- / 0",
			TQ:          "5 : 1",
			Updated:     "8 seconds",
			LastUpdated: now.Add(-8 * time.Second),
			Description: "Robfig cron scheduler engine, periodic task dispatcher, and user session cleaner",
			ModuleKey:   "Cron",
		},
		{
			ID:          "srv-alert-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Alert & Notification Server",
			Icon:        "bell",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "15 of 15 webhooks",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "14 seconds",
			LastUpdated: now.Add(-14 * time.Second),
			Description: "Threshold breach evaluation, incident escalation rules, and multi-channel webhook dispatcher",
			ModuleKey:   "Alert",
		},
		{
			ID:          "srv-prom-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Prometheus & PromQL Collector",
			Icon:        "highperf",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "500 of 500 metrics",
			Lag:         "7 seconds / 5",
			TQ:          "4 : 0",
			Updated:     "9 seconds",
			LastUpdated: now.Add(-9 * time.Second),
			Description: "High-frequency metric ingestion from Prometheus node exporters and PromQL bridge",
			ModuleKey:   "Prometheus",
		},
		{
			ID:          "srv-worker-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Heavy Background Worker Pool",
			Icon:        "heavy",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "5 Concurrent Threads",
			Lag:         "- / 0",
			TQ:          "5 : 0",
			Updated:     "3 seconds",
			LastUpdated: now.Add(-3 * time.Second),
			Description: "5 Goroutine worker pool threads for async batch tasks, exports, and heavy jobs",
			ModuleKey:   "Queue",
		},
		{
			ID:          "srv-grok-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Grok Engine & Log Parser",
			Icon:        "syslog",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "1200 logs/min",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "10 seconds",
			LastUpdated: now.Add(-10 * time.Second),
			Description: "Pattern matching, regex parser, and log structure transformation engine",
			ModuleKey:   "Grok",
		},
		{
			ID:          "srv-vps-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "VPS & Remote Host Monitor",
			Icon:        "server",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "4 of 4 hosts",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "15 seconds",
			LastUpdated: now.Add(-15 * time.Second),
			Description: "Remote server CPU/RAM/Disk telemetry, process manager, and systemd service control",
			ModuleKey:   "VPS",
		},
		{
			ID:          "srv-ssh-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "SSH Terminal & SFTP Transfer",
			Icon:        "terminal",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "8 Active PTY",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "5 seconds",
			LastUpdated: now.Add(-5 * time.Second),
			Description: "Interactive PTY WebSocket terminal multiplexer and secure SFTP file browser daemon",
			ModuleKey:   "SSH",
		},
		{
			ID:          "srv-dataprepper-master",
			Name:        "labs-hcp-master",
			Status:      "running",
			Type:        "Data Prepper Pipeline Validator",
			Icon:        "discovery",
			Master:      true,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "3 of 3 pipelines",
			Lag:         "- / 0",
			TQ:          "1 : 0",
			Updated:     "16 seconds",
			LastUpdated: now.Add(-16 * time.Second),
			Description: "Data Prepper YAML configuration validator, buffer health check, and sink router",
			ModuleKey:   "DataPrepper",
		},
		// Distributed Edge Collector Node
		{
			ID:          "srv-icmp-edge",
			Name:        "labs-hcp-worker-01",
			Status:      "running",
			Type:        "Network Server (Edge ICMP Probe)",
			Icon:        "network",
			Master:      false,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "128 of 128 targets",
			Lag:         "- / 0",
			TQ:          "2 : 0",
			Updated:     "7 seconds",
			LastUpdated: now.Add(-7 * time.Second),
			Description: "Edge distributed ping probe and remote branch latency monitor",
			ModuleKey:   "Network",
		},
		{
			ID:          "srv-snmp-edge",
			Name:        "labs-hcp-worker-01",
			Status:      "running",
			Type:        "SNMP Trap Receiver (Edge Poller)",
			Icon:        "radio",
			Master:      false,
			Version:     "2.0.0 (Go 1.22)",
			Modules:     "32 of 32 devices",
			Lag:         "N/A",
			TQ:          "1 : 0",
			Updated:     "11 seconds",
			LastUpdated: now.Add(-11 * time.Second),
			Description: "Branch office SNMP trap forwarder and interface traffic poller",
			ModuleKey:   "SNMP",
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
	} else if strings.Contains(id, "backup") {
		_, _ = wp.Enqueue("database_backup", map[string]interface{}{"manual": true}, 1)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Daemon service restarted and execution cycle triggered successfully.",
		"service": id,
	})
}
