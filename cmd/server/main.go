package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go-hephaestus/internal/cli"
	"go-hephaestus/internal/config"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/handlers"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/middleware"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/scheduler"
	"go-hephaestus/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Support direct CLI subcommands (e.g. reset-password, list-users, create-user)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset-password", "reset-admin", "passwd", "list-users", "create-user", "add-user", "cli":
			cli.RunCLI(os.Args[1:])
			return
		}
	}

	// 1. Load Configurations
	cfg := config.LoadConfig()

	// 2. Initialize Structured Logger with file rotation
	logger.InitLogger(cfg.LogsDir)
	logger.Info("Server", fmt.Sprintf("Starting Hephaestus Control Panel (HCP) v2.0.0 on port %d...", cfg.Port))
	if config.IsUsingDefaultSecretKey() {
		logger.Warn("Security", "CRITICAL WARNING: Running with default APP_ENCRYPTION_KEY! Please set a unique APP_ENCRYPTION_KEY in your .env file to ensure credential encryption security.")
	}

	// 3. Initialize PostgreSQL Connection Pool & Run Migrations with Retry
	var dbErr error
	for attempt := 1; attempt <= 15; attempt++ {
		dbCtx, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		dbErr = database.InitDatabase(dbCtx, cfg)
		cancelDB()
		if dbErr == nil {
			logger.Info("Database", "PostgreSQL database connected and schema initialized successfully.")
			break
		}
		logger.Warn("Database", fmt.Sprintf("Waiting for PostgreSQL connection (attempt %d/15): %v", attempt, dbErr))
		time.Sleep(2 * time.Second)
	}
	if dbErr != nil {
		logger.Warn("Database", fmt.Sprintf("PostgreSQL connection incomplete (%v). Server will start in Setup/Recovery mode.", dbErr))
	}

	// 4. Initialize Repositories
	userRepo := repository.NewUserRepository()
	configRepo := repository.NewConfigRepository()
	snmpRepo := repository.NewSnmpRepository()
	remoteRepo := repository.NewRemoteHostRepository()
	topologyRepo := repository.NewTopologyRepository()
	backupRepo := repository.NewBackupRepository()
	otelRepo := repository.NewOTelRepository()
	vaultwardenRepo := repository.NewVaultwardenRepository()
	dockerRepo := repository.NewDockerRepository()
	reportRepo := repository.NewReportRepository()
	statusPageRepo := repository.NewStatusPageRepository()
	monitoringInstanceRepo := repository.NewMonitoringInstanceRepository()
	ipamRepo := repository.NewIpamRepository()
	serverInventoryRepo := repository.NewServerInventoryRepository()

	// 5. Initialize Background Worker Pool & Scheduler
	workerPool := queue.InitWorkerPool(10)
	cronSched := scheduler.InitCronScheduler(backupRepo, userRepo)

	// 6. Initialize Services
	authService := services.NewAuthService(userRepo, configRepo)
	sshService := services.NewSSHService(remoteRepo)
	wsService := services.NewWsTerminalService(sshService)
	backupService := services.NewBackupService(backupRepo, sshService)
	snmpService := services.NewSnmpService(snmpRepo, cfg.DataDir)
	icmpService := services.NewIcmpPingService(topologyRepo) // Registers ICMP worker
	topologyService := services.NewTopologyService(topologyRepo, configRepo, remoteRepo)
	openSearchService := services.NewOpenSearchService()
	openSearchService.RegisterWorker(workerPool)
	promService := services.NewPrometheusService(configRepo, sshService)
	vpsService := services.NewVpsService(remoteRepo, sshService)
	firewallService := services.NewFirewallService(remoteRepo, sshService)
	grokService := services.NewGrokService()
	dpService := services.NewDataPrepperService(sshService)
	otelService := services.NewOTelService(otelRepo, sshService)
	vaultwardenService := services.NewVaultwardenService(vaultwardenRepo)
	vaultwardenService.StartBackgroundSync(nil)
	dockerService := services.NewDockerService(dockerRepo, remoteRepo, sshService)
	systemService := services.NewSystemService()
	reportService := services.NewReportService(reportRepo, configRepo, openSearchService, promService)
	statusPageService := services.NewStatusPageService(statusPageRepo, topologyRepo, remoteRepo, configRepo, openSearchService, promService, sshService)
	monitoringInstanceService := services.NewMonitoringInstanceService(monitoringInstanceRepo, remoteRepo, promService, workerPool, vpsService)
	monitoringInstanceService.StartBackgroundEngine()
	ipamService := services.NewIpamService(ipamRepo, workerPool)
	ipamService.StartBackgroundEngine()
	serverInventoryService := services.NewServerInventoryService(serverInventoryRepo, remoteRepo, sshService)
	guacService := services.NewGuacamoleService()

	// 7. Initialize HTTP Handlers
	authHandler := handlers.NewAuthHandler(authService)
	setupHandler := handlers.NewSetupHandler(authService)
	remoteHostHandler := handlers.NewRemoteHostHandler(remoteRepo, sshService, wsService, authService, vpsService, firewallService, guacService)
	topologyHandler := handlers.NewTopologyHandler(topologyRepo, topologyService, icmpService)
	backupHandler := handlers.NewBackupHandler(backupRepo, backupService, cronSched)
	snmpHandler := handlers.NewSnmpHandler(snmpRepo, snmpService)
	openSearchHandler := handlers.NewOpenSearchHandler(openSearchService)
	promHandler := handlers.NewPrometheusHandler(promService)
	grokHandler := handlers.NewGrokHandler(grokService)
	dpHandler := handlers.NewDataPrepperHandler(dpService)
	otelHandler := handlers.NewOTelHandler(otelService, otelRepo)
	vaultwardenHandler := handlers.NewVaultwardenHandler(vaultwardenService, vaultwardenRepo)
	dockerHandler := handlers.NewDockerHandler(dockerService, dockerRepo)
	settingsHandler := handlers.NewSettingsHandler(configRepo, userRepo, systemService)
	logsHandler := handlers.NewLogsHandler(authService)
	queueHandler := handlers.NewQueueHandler()
	reportHandler := handlers.NewReportHandler(reportService)
	statusPageHandler := handlers.NewStatusPageHandler(statusPageService, authService)
	monitoringInstanceHandler := handlers.NewMonitoringInstanceHandler(monitoringInstanceService)
	ipamHandler := handlers.NewIpamHandler(ipamService)
	serverInventoryHandler := handlers.NewServerInventoryHandler(serverInventoryService)

	// 8. Gin Router Setup
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLoggerMiddleware())
	r.Use(middleware.GzipMiddleware())

	// Set Trusted Proxies to prevent X-Forwarded-For IP spoofing (C-01)
	// Trust Docker internal bridge, localhost, and RFC1918 private subnets
	_ = r.SetTrustedProxies([]string{
		"127.0.0.1",
		"::1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})

	// CORS Setup - Restrict origins to prevent credentialed cross-origin reflection (H-01, N-02)
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOriginFunc = func(origin string) bool {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			return true
		}

		// Check explicitly configured allowed origins from config / env
		for _, allowed := range cfg.AllowedOrigins {
			if strings.EqualFold(origin, strings.TrimSpace(allowed)) {
				return true
			}
		}

		// In development mode only, permit local frontend development dev servers
		if cfg.Env != "production" && gin.Mode() != gin.ReleaseMode {
			if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" ||
				origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
				return true
			}
		}

		// In production, reject arbitrary loopback & external origins from credential reflection (N-02)
		return false
	}
	corsConfig.AllowCredentials = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-Requested-With"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	// Healthcheck & Metrics (HCP-SEC-010: Minimal UP status to prevent information disclosure)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})

	// Public Setup Wizard routes
	setupGroup := r.Group("/api/v1/setup")
	{
		setupGroup.GET("/status", setupHandler.GetSetupStatus)
		setupGroup.POST("/complete", setupHandler.CompleteSetup)
	}

	// Public Auth routes (with brute-force protection)
	r.POST("/api/v1/auth/login", middleware.AuthRateLimitMiddleware(), authHandler.Login)
	r.POST("/api/v1/auth/logout", authHandler.Logout)

	// Public Status Page Endpoint (accessible publicly or checks auth if private)
	r.GET("/api/v1/status-pages/public/:slug", statusPageHandler.GetPublicReport)

	// WebSocket Endpoints (WebSocket handles auth via handshake message)
	r.GET("/ws/remote-host", remoteHostHandler.HandleWebSocketTerminal)
	r.GET("/ws/remote-desktop", remoteHostHandler.HandleWebSocketDesktop)
	r.GET("/ws/logs", logsHandler.StreamLogsWebSocket)

	// Protected API Routes (with general rate limiting)
	api := r.Group("/api/v1")
	api.Use(middleware.GeneralRateLimitMiddleware())
	api.Use(middleware.AuthMiddleware(authService))
	{
		// User & Profile
		api.GET("/auth/me", authHandler.GetCurrentUser)
		api.POST("/auth/change-password", authHandler.ChangePassword)

		// Remote Hosts & SFTP (Feature: remote_servers)
		api.GET("/remote-host", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.List)
		api.GET("/remote-host/:id", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetByID)
		api.POST("/remote-host", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.Save)
		api.POST("/remote-host/batch-group", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.BatchUpdateGroup)
		api.DELETE("/remote-host/:id", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.Delete)
		api.POST("/remote-host/test", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.TestConnection)
		api.GET("/remote-host/:id/rdp", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.DownloadRdpFile)
		api.GET("/remote-host/:id/sftp/list", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.SftpList)
		api.POST("/remote-host/:id/sftp/upload", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.SftpUpload)
		api.GET("/remote-host/:id/sftp/download", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.SftpDownload)
		api.POST("/remote-host/sftp/transfer-remote", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.SftpTransferRemote)
		api.GET("/remote-host/:id/metrics", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetMetrics)
		api.GET("/remote-host/:id/processes", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetProcesses)
		api.DELETE("/remote-host/:id/processes/:pid", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.KillProcess)
		api.GET("/remote-host/:id/services", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetServices)
		api.POST("/remote-host/:id/services/control", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.ControlService)
		api.GET("/remote-host/:id/network", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetNetworkInfo)
		api.GET("/remote-host/:id/firewall", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetFirewallStatus)
		api.POST("/remote-host/:id/firewall/rules", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.AddFirewallRule)
		api.DELETE("/remote-host/:id/firewall/rules/:ruleId", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.DeleteFirewallRule)
		api.POST("/remote-host/:id/firewall/toggle", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.ToggleFirewall)
		api.GET("/remote-host/users", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.ListAvailableUsers)
		api.GET("/remote-host/:id/shares", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.ListShares)
		api.POST("/remote-host/:id/shares", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.AddShare)
		api.DELETE("/remote-host/:id/shares/:userId", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.DeleteShare)

		// Topology (Feature: network_topology)
		api.GET("/topology", middleware.RequirePermission("network_topology", "read"), topologyHandler.GetGraph)
		api.GET("/topology/users", middleware.RequirePermission("network_topology", "read"), topologyHandler.ListAvailableUsers)
		api.GET("/topology/sheets", middleware.RequirePermission("network_topology", "read"), topologyHandler.ListSheets)
		api.POST("/topology/sheets", middleware.RequirePermission("network_topology", "manage"), topologyHandler.CreateSheet)
		api.PUT("/topology/sheets/:id", middleware.RequirePermission("network_topology", "manage"), topologyHandler.UpdateSheet)
		api.PUT("/topology/sheets/:id/visibility", middleware.RequirePermission("network_topology", "manage"), topologyHandler.UpdateSheetVisibility)
		api.DELETE("/topology/sheets/:id", middleware.RequirePermission("network_topology", "manage"), topologyHandler.DeleteSheet)
		api.GET("/topology/sheets/:id/shares", middleware.RequirePermission("network_topology", "read"), topologyHandler.ListSheetShares)
		api.POST("/topology/sheets/:id/shares", middleware.RequirePermission("network_topology", "manage"), topologyHandler.AddSheetShare)
		api.DELETE("/topology/sheets/:id/shares/:userId", middleware.RequirePermission("network_topology", "manage"), topologyHandler.DeleteSheetShare)
		api.POST("/topology/devices", middleware.RequirePermission("network_topology", "manage"), topologyHandler.SaveDevice)
		api.PUT("/topology/devices/:id/position", middleware.RequirePermission("network_topology", "manage"), topologyHandler.UpdatePosition)
		api.DELETE("/topology/devices/:id", middleware.RequirePermission("network_topology", "manage"), topologyHandler.DeleteDevice)
		api.POST("/topology/devices/:id/remove-from-canvas", middleware.RequirePermission("network_topology", "manage"), topologyHandler.RemoveDeviceFromCanvas)
		api.POST("/topology/edges", middleware.RequirePermission("network_topology", "manage"), topologyHandler.SaveEdge)
		api.DELETE("/topology/edges/:id", middleware.RequirePermission("network_topology", "manage"), topologyHandler.DeleteEdge)
		api.GET("/topology/discover/prometheus", middleware.RequirePermission("network_topology", "read"), topologyHandler.DiscoverPrometheus)
		api.GET("/topology/discover/subnet", middleware.RequirePermission("network_topology", "read"), topologyHandler.ScanSubnet)
		api.POST("/topology/sync/remote-server", middleware.RequirePermission("network_topology", "manage"), topologyHandler.SyncRemoteServers)
		api.GET("/topology/sync/remote-server", middleware.RequirePermission("network_topology", "manage"), topologyHandler.SyncRemoteServers)
		api.GET("/topology/ping", middleware.RequirePermission("network_topology", "read"), topologyHandler.PingDevice)

		// IP Address Management (Feature: ipam)
		api.GET("/ipam/stats", middleware.RequirePermission("ipam", "read"), ipamHandler.GetSummaryStats)
		api.GET("/ipam/subnets", middleware.RequirePermission("ipam", "read"), ipamHandler.ListSubnets)
		api.POST("/ipam/subnets", middleware.RequirePermission("ipam", "manage"), ipamHandler.CreateSubnet)
		api.POST("/ipam/subnets/scan-all", middleware.RequirePermission("ipam", "manage"), ipamHandler.ScanAllSubnets)
		api.GET("/ipam/subnets/:id", middleware.RequirePermission("ipam", "read"), ipamHandler.GetSubnet)
		api.PUT("/ipam/subnets/:id", middleware.RequirePermission("ipam", "manage"), ipamHandler.UpdateSubnet)
		api.DELETE("/ipam/subnets/:id", middleware.RequirePermission("ipam", "manage"), ipamHandler.DeleteSubnet)
		api.POST("/ipam/subnets/:id/scan", middleware.RequirePermission("ipam", "manage"), ipamHandler.TriggerScan)
		api.GET("/ipam/subnets/:id/next-available", middleware.RequirePermission("ipam", "read"), ipamHandler.GetNextAvailableIP)
		api.GET("/ipam/subnets/:id/logs", middleware.RequirePermission("ipam", "read"), ipamHandler.ListScanLogs)
		api.POST("/ipam/addresses", middleware.RequirePermission("ipam", "manage"), ipamHandler.SaveAddress)
		api.PUT("/ipam/addresses/:id", middleware.RequirePermission("ipam", "manage"), ipamHandler.UpdateAddress)
		api.DELETE("/ipam/addresses/:id", middleware.RequirePermission("ipam", "manage"), ipamHandler.DeleteAddress)
		api.POST("/ipam/addresses/ping", middleware.RequirePermission("ipam", "read"), ipamHandler.PingIP)

		// Backups (Feature: backup)
		api.GET("/backup/databases", middleware.RequirePermission("backup", "read"), backupHandler.ListDBConfigs)
		api.POST("/backup/databases", middleware.RequirePermission("backup", "manage"), backupHandler.SaveDBConfig)
		api.POST("/backup/databases/test", middleware.RequirePermission("backup", "manage"), backupHandler.TestDBConfig)
		api.DELETE("/backup/databases/:id", middleware.RequirePermission("backup", "manage"), backupHandler.DeleteDBConfig)
		api.GET("/backup/destinations", middleware.RequirePermission("backup", "read"), backupHandler.ListDestinations)
		api.POST("/backup/destinations", middleware.RequirePermission("backup", "manage"), backupHandler.SaveDestination)
		api.POST("/backup/destinations/test", middleware.RequirePermission("backup", "manage"), backupHandler.TestDestination)
		api.DELETE("/backup/destinations/:id", middleware.RequirePermission("backup", "manage"), backupHandler.DeleteDestination)
		api.GET("/backup/schedules", middleware.RequirePermission("backup", "read"), backupHandler.ListSchedules)
		api.POST("/backup/schedules", middleware.RequirePermission("backup", "manage"), backupHandler.SaveSchedule)
		api.DELETE("/backup/schedules/:id", middleware.RequirePermission("backup", "manage"), backupHandler.DeleteSchedule)
		api.POST("/backup/run", middleware.RequirePermission("backup", "manage"), backupHandler.RunBackup)
		api.GET("/backup/history", middleware.RequirePermission("backup", "read"), backupHandler.ListHistory)
		api.GET("/backup/history/:id/download", middleware.RequirePermission("backup", "read"), backupHandler.DownloadBackup)
		api.DELETE("/backup/history/:id", middleware.RequirePermission("backup", "manage"), backupHandler.DeleteHistory)

		// SNMP (Feature: snmp)
		api.POST("/snmp/discover", middleware.RequirePermission("snmp", "read"), snmpHandler.Discover)
		api.GET("/snmp/profiles", middleware.RequirePermission("snmp", "read"), snmpHandler.GetProfiles)
		api.POST("/snmp/subnet-scan", middleware.RequirePermission("snmp", "read"), snmpHandler.ScanSubnet)
		api.POST("/snmp/query", middleware.RequirePermission("snmp", "read"), snmpHandler.Query)
		api.GET("/snmp/mibs", middleware.RequirePermission("snmp", "read"), snmpHandler.ListMibs)
		api.POST("/snmp/mibs", middleware.RequirePermission("snmp", "manage"), snmpHandler.ImportMib)
		api.DELETE("/snmp/mibs/:name", middleware.RequirePermission("snmp", "manage"), snmpHandler.DeleteMib)
		api.GET("/snmp/translate", middleware.RequirePermission("snmp", "read"), snmpHandler.TranslateOID)

		// OpenSearch Cluster Management (Feature: opensearch)
		api.GET("/opensearch/health", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetHealth)
		api.GET("/opensearch/nodes", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetNodesStats)
		api.GET("/opensearch/nodes/info", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetNodesInfo)
		api.GET("/opensearch/indices", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetIndices)
		api.GET("/opensearch/shards", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetShards)
		api.GET("/opensearch/recovery", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetRecovery)
		api.GET("/opensearch/config", middleware.RequirePermission("opensearch", "read"), openSearchHandler.GetConfig)
		api.GET("/opensearch/configs", middleware.RequirePermission("opensearch", "read"), openSearchHandler.ListConfigs)
		api.POST("/opensearch/config", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.SaveConfig)
		api.DELETE("/opensearch/config", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.DeleteConfig)
		api.DELETE("/opensearch/config/:id", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.DeleteConfig)
		api.POST("/opensearch/test", middleware.RequirePermission("opensearch", "read"), openSearchHandler.TestConnection)
		api.GET("/opensearch/configs/:id/shares", middleware.RequirePermission("opensearch", "read"), openSearchHandler.ListShares)
		api.POST("/opensearch/configs/:id/shares", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.AddShare)
		api.DELETE("/opensearch/configs/:id/shares/:userId", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.DeleteShare)
		api.POST("/opensearch/configs/:id/visibility", middleware.RequirePermission("opensearch", "manage"), openSearchHandler.UpdateVisibility)

		// Prometheus & PromQL (Feature: prometheus_config)
		api.GET("/prometheus/query", middleware.RequirePermission("prometheus_config", "read"), promHandler.Query)
		api.POST("/prometheus/reload", middleware.RequirePermission("prometheus_config", "manage"), promHandler.Reload)
		api.GET("/prometheus/config", middleware.RequirePermission("prometheus_config", "read"), promHandler.GetConfig)
		api.POST("/prometheus/config", middleware.RequirePermission("prometheus_config", "manage"), promHandler.SaveConfig)
		api.POST("/prometheus/validate", middleware.RequirePermission("prometheus_config", "read"), promHandler.ValidateConfig)

		// Monitoring Instances (Feature: monitoring_instances)
		api.GET("/monitoring/instances", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.ListInstances)
		api.GET("/monitoring/instances/groups", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.GetGroups)
		api.GET("/monitoring/containers", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.ListContainers)
		api.GET("/monitoring/containers/history", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.GetContainerHistory)
		api.GET("/monitoring/instances/:id", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.GetInstance)
		api.POST("/monitoring/instances", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.CreateInstance)
		api.PUT("/monitoring/instances/:id", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.UpdateInstance)
		api.DELETE("/monitoring/instances/:id", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.DeleteInstance)
		api.PUT("/monitoring/instances/:id/alert", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.ToggleAlert)
		api.POST("/monitoring/instances/sync-remote-hosts", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.SyncFromRemoteHosts)
		api.GET("/monitoring/instances/:id/history", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.GetInstanceHistory)
		api.GET("/monitoring/instances/:id/shares", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.ListShares)
		api.POST("/monitoring/instances/:id/shares", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.AddShare)
		api.DELETE("/monitoring/instances/:id/shares/:userId", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.DeleteShare)
		api.GET("/monitoring/instances/engine/status", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.GetEngineStatus)
		api.POST("/monitoring/instances/engine/interval", middleware.RequirePermission("monitoring_instances", "manage"), monitoringInstanceHandler.SetEngineInterval)
		api.POST("/monitoring/instances/poll-now", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.PollNow)
		api.POST("/monitoring/instances/:id/fetch-ssh", middleware.RequirePermission("monitoring_instances", "read"), monitoringInstanceHandler.FetchSSHMetrics)

		// Server Inventory & Hardware Specs (Feature: server_inventory, connections, or infrastructure)
		api.GET("/inventory/servers", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "read"), serverInventoryHandler.List)
		api.GET("/inventory/servers/stats", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "read"), serverInventoryHandler.GetStats)
		api.GET("/inventory/servers/template", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "read"), serverInventoryHandler.DownloadTemplate)
		api.GET("/inventory/servers/export", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "read"), serverInventoryHandler.ExportCSV)
		api.POST("/inventory/servers/import", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.ImportCSV)
		api.POST("/inventory/servers/sync-all", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.SyncAllRemoteHosts)
		api.POST("/inventory/servers/sync-remote/:remoteHostId", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.SyncFromRemoteHost)
		api.GET("/inventory/servers/:id", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "read"), serverInventoryHandler.GetByID)
		api.POST("/inventory/servers", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.Create)
		api.PUT("/inventory/servers/:id", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.Update)
		api.DELETE("/inventory/servers/:id", middleware.RequireAnyPermission([]string{"server_inventory", "connections", "infrastructure"}, "manage"), serverInventoryHandler.Delete)

		// VPS Telemetry, Processes, Services, and Network (Feature: remote_servers) - Compatibility aliases
		api.GET("/vps/:id/metrics", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetMetrics)
		api.GET("/vps/:id/processes", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetProcesses)
		api.POST("/vps/:id/processes/:pid/kill", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.KillProcess)
		api.DELETE("/vps/:id/processes/:pid", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.KillProcess)
		api.GET("/vps/:id/services", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetServices)
		api.POST("/vps/:id/services/control", middleware.RequirePermission("remote_servers", "manage"), remoteHostHandler.ControlService)
		api.GET("/vps/:id/network", middleware.RequirePermission("remote_servers", "read"), remoteHostHandler.GetNetworkInfo)

		// Grok Debugger (Feature: grok_debugger)
		api.POST("/grok/test", middleware.RequirePermission("grok_debugger", "read"), grokHandler.Test)
		api.GET("/grok/patterns", middleware.RequirePermission("grok_debugger", "read"), grokHandler.GetPatterns)

		// Data Prepper (Feature: dataprepper_config)
		api.GET("/dataprepper/pipelines", middleware.RequirePermission("dataprepper_config", "read"), dpHandler.ListPipelines)
		api.GET("/dataprepper/pipeline", middleware.RequirePermission("dataprepper_config", "read"), dpHandler.GetPipelineFile)
		api.POST("/dataprepper/pipeline", middleware.RequirePermission("dataprepper_config", "manage"), dpHandler.SavePipelineFile)
		api.DELETE("/dataprepper/pipeline", middleware.RequirePermission("dataprepper_config", "manage"), dpHandler.DeletePipelineFile)
		api.POST("/dataprepper/validate", middleware.RequirePermission("dataprepper_config", "read"), dpHandler.ValidateYAML)

		// OpenTelemetry Remote Config (Feature: opentelemetry_config)
		api.GET("/otel/hosts", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.ListHosts)
		api.GET("/otel/hosts/:id", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.GetHost)
		api.POST("/otel/hosts", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.SaveHost)
		api.PUT("/otel/hosts/:id", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.SaveHost)
		api.DELETE("/otel/hosts/:id", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.DeleteHost)
		api.POST("/otel/hosts/test", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.TestHost)
		api.GET("/otel/hosts/:id/status", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.GetHostStatus)
		api.GET("/otel/hosts/:id/config", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.GetConfigFile)
		api.POST("/otel/hosts/:id/config", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.SaveConfigFile)
		api.POST("/otel/hosts/:id/validate", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.ValidateConfig)
		api.POST("/otel/hosts/:id/restart", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.RestartService)
		api.GET("/otel/presets", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.GetPresets)
		api.GET("/otel/hosts/:id/history", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.ListHistory)
		api.GET("/otel/hosts/:id/shares", middleware.RequirePermission("opentelemetry_config", "read"), otelHandler.ListShares)
		api.POST("/otel/hosts/:id/shares", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.AddShare)
		api.DELETE("/otel/hosts/:id/shares/:userId", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.DeleteShare)
		api.POST("/otel/hosts/:id/visibility", middleware.RequirePermission("opentelemetry_config", "manage"), otelHandler.UpdateVisibility)

		// Live Logs, Services & Queue (Feature: settings)
		api.GET("/logs", middleware.RequirePermission("settings", "read"), logsHandler.GetRecentLogs)
		api.GET("/logs/recent", middleware.RequirePermission("settings", "read"), logsHandler.GetRecentLogs)
		api.GET("/services", middleware.RequirePermission("settings", "read"), queueHandler.ListServices)
		api.POST("/services/:id/restart", middleware.RequirePermission("settings", "manage"), queueHandler.RestartService)
		api.GET("/queue/jobs", middleware.RequirePermission("settings", "read"), queueHandler.ListJobs)
		api.GET("/queue/jobs/:id", middleware.RequirePermission("settings", "read"), queueHandler.GetJob)
		api.POST("/queue/jobs/trigger", middleware.RequirePermission("settings", "manage"), queueHandler.TriggerJob)
		api.POST("/queue/jobs/:id/cancel", middleware.RequirePermission("settings", "manage"), queueHandler.CancelJob)

		// Settings & System Stats
		api.GET("/settings/system", middleware.RequirePermission("settings", "read"), settingsHandler.GetSystemStats)
		api.GET("/settings/activity-logs", middleware.RequirePermission("settings", "read"), settingsHandler.ListActivityLogs)

		// User & Role Management (Exclusive to ADMIN role)
		api.GET("/settings/users", middleware.RequireRole("ADMIN"), settingsHandler.ListUsers)
		api.POST("/settings/users", middleware.RequireRole("ADMIN"), settingsHandler.CreateUser)
		api.PUT("/settings/users/:id", middleware.RequireRole("ADMIN"), settingsHandler.UpdateUser)
		api.PUT("/settings/users/:id/role", middleware.RequireRole("ADMIN"), settingsHandler.UpdateUserRole)
		api.DELETE("/settings/users/:id", middleware.RequireRole("ADMIN"), settingsHandler.DeleteUser)
		api.GET("/settings/roles", middleware.RequireRole("ADMIN"), settingsHandler.ListRoles)
		api.POST("/settings/roles", middleware.RequireRole("ADMIN"), settingsHandler.SaveRole)
		api.DELETE("/settings/roles/:id", middleware.RequireRole("ADMIN"), settingsHandler.DeleteRole)

		// External Monitoring Connections (Feature: connections)
		api.GET("/settings/grafana", middleware.RequirePermission("connections", "read"), settingsHandler.ListGrafana)
		api.POST("/settings/grafana", middleware.RequirePermission("connections", "manage"), settingsHandler.SaveGrafana)
		api.POST("/settings/grafana/test", middleware.RequirePermission("connections", "read"), settingsHandler.TestGrafana)
		api.GET("/settings/grafana/datasources", middleware.RequirePermission("connections", "read"), settingsHandler.GetGrafanaDatasources)
		api.GET("/reports/grafana/datasources", middleware.RequirePermission("reports", "read"), settingsHandler.GetGrafanaDatasources)
		api.POST("/settings/grafana/:id/active", middleware.RequirePermission("connections", "manage"), settingsHandler.SetActiveGrafana)
		api.DELETE("/settings/grafana/:id", middleware.RequirePermission("connections", "manage"), settingsHandler.DeleteGrafana)
		api.GET("/settings/grafana/:id/shares", middleware.RequirePermission("connections", "read"), settingsHandler.ListGrafanaShares)
		api.POST("/settings/grafana/:id/shares", middleware.RequirePermission("connections", "manage"), settingsHandler.AddGrafanaShare)
		api.DELETE("/settings/grafana/:id/shares/:userId", middleware.RequirePermission("connections", "manage"), settingsHandler.DeleteGrafanaShare)
		api.POST("/settings/grafana/:id/visibility", middleware.RequirePermission("connections", "manage"), settingsHandler.UpdateGrafanaVisibility)

		api.GET("/settings/prometheus", middleware.RequirePermission("connections", "read"), settingsHandler.ListPrometheus)
		api.POST("/settings/prometheus", middleware.RequirePermission("connections", "manage"), settingsHandler.SavePrometheus)
		api.POST("/settings/prometheus/:id/active", middleware.RequirePermission("connections", "manage"), settingsHandler.SetActivePrometheus)
		api.DELETE("/settings/prometheus/:id", middleware.RequirePermission("connections", "manage"), settingsHandler.DeletePrometheus)
		api.POST("/settings/prometheus/test", middleware.RequirePermission("connections", "read"), promHandler.TestConnection)
		api.GET("/settings/prometheus/:id/shares", middleware.RequirePermission("connections", "read"), settingsHandler.ListPrometheusShares)
		api.POST("/settings/prometheus/:id/shares", middleware.RequirePermission("connections", "manage"), settingsHandler.AddPrometheusShare)
		api.DELETE("/settings/prometheus/:id/shares/:userId", middleware.RequirePermission("connections", "manage"), settingsHandler.DeletePrometheusShare)
		api.POST("/settings/prometheus/:id/visibility", middleware.RequirePermission("connections", "manage"), settingsHandler.UpdatePrometheusVisibility)

		api.GET("/settings/database", middleware.RequirePermission("settings", "read"), settingsHandler.GetDatabaseConfig)
		api.POST("/settings/database", middleware.RequirePermission("settings", "manage"), settingsHandler.UpdateDatabaseConfig)
		api.POST("/settings/database/test", middleware.RequirePermission("settings", "read"), settingsHandler.TestDatabaseConnection)
		api.GET("/settings/threads", middleware.RequirePermission("settings", "read"), settingsHandler.GetServiceThreads)
		api.POST("/settings/threads", middleware.RequirePermission("settings", "manage"), settingsHandler.UpdateServiceThreads)

		// Monitoring Views / Slide Show (Feature: slideshow)
		api.GET("/monitoring-views", middleware.RequirePermission("slideshow", "read"), settingsHandler.ListMonitoringViews)
		api.POST("/monitoring-views", middleware.RequirePermission("slideshow", "manage"), settingsHandler.SaveMonitoringView)
		api.DELETE("/monitoring-views/:id", middleware.RequirePermission("slideshow", "manage"), settingsHandler.DeleteMonitoringView)

		// Vaultwarden / Bitwarden E2EE Integration (Feature: security)
		api.GET("/vaultwarden/config", middleware.RequirePermission("security", "read"), vaultwardenHandler.GetConfig)
		api.GET("/vaultwarden/configs", middleware.RequirePermission("security", "read"), vaultwardenHandler.ListConfigs)
		api.POST("/vaultwarden/config", middleware.RequirePermission("security", "manage"), vaultwardenHandler.SaveConfig)
		api.POST("/vaultwarden/test", middleware.RequirePermission("security", "read"), vaultwardenHandler.TestConnection)
		api.GET("/vaultwarden/ciphers", middleware.RequirePermission("security", "read"), vaultwardenHandler.GetCiphers)
		api.POST("/vaultwarden/ciphers", middleware.RequirePermission("security", "manage"), vaultwardenHandler.CreateCipher)
		api.PUT("/vaultwarden/ciphers/:id", middleware.RequirePermission("security", "manage"), vaultwardenHandler.UpdateCipher)
		api.DELETE("/vaultwarden/ciphers/:id", middleware.RequirePermission("security", "manage"), vaultwardenHandler.DeleteCipher)
		api.POST("/vaultwarden/sync", middleware.RequirePermission("security", "manage"), vaultwardenHandler.SyncVault)
		api.DELETE("/vaultwarden/config", middleware.RequirePermission("security", "manage"), vaultwardenHandler.DeleteConfig)
		api.GET("/vaultwarden/folders", middleware.RequirePermission("security", "read"), vaultwardenHandler.GetFolders)
		api.POST("/vaultwarden/folders", middleware.RequirePermission("security", "manage"), vaultwardenHandler.CreateFolder)
		api.PUT("/vaultwarden/folders/:id", middleware.RequirePermission("security", "manage"), vaultwardenHandler.UpdateFolder)
		api.DELETE("/vaultwarden/folders/:id", middleware.RequirePermission("security", "manage"), vaultwardenHandler.DeleteFolder)
		api.GET("/vaultwarden/shares", middleware.RequirePermission("security", "read"), vaultwardenHandler.ListShares)
		api.POST("/vaultwarden/shares", middleware.RequirePermission("security", "manage"), vaultwardenHandler.AddShare)
		api.DELETE("/vaultwarden/shares/:userId", middleware.RequirePermission("security", "manage"), vaultwardenHandler.DeleteShare)
		api.POST("/vaultwarden/visibility", middleware.RequirePermission("security", "manage"), vaultwardenHandler.UpdateVisibility)

		// Docker & Container Management (Feature: infrastructure & connections)
		api.GET("/docker/connections", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "read"), dockerHandler.ListConnections)
		api.POST("/docker/connections", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "manage"), dockerHandler.SaveConnection)
		api.POST("/docker/connections/test", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "read"), dockerHandler.TestConnection)
		api.DELETE("/docker/connections/:id", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "manage"), dockerHandler.DeleteConnection)
		api.GET("/docker/connections/:id/shares", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "read"), dockerHandler.ListConnectionShares)
		api.POST("/docker/connections/:id/shares", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "manage"), dockerHandler.AddConnectionShare)
		api.DELETE("/docker/connections/:id/shares/:userId", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "manage"), dockerHandler.DeleteConnectionShare)
		api.POST("/docker/connections/:id/visibility", middleware.RequireAnyPermission([]string{"connections", "infrastructure"}, "manage"), dockerHandler.UpdateConnectionVisibility)
		api.GET("/docker/containers", middleware.RequirePermission("infrastructure", "read"), dockerHandler.ListContainers)
		api.GET("/docker/containers/:id", middleware.RequirePermission("infrastructure", "read"), dockerHandler.GetContainer)
		api.GET("/docker/containers/:id/inspect", middleware.RequirePermission("infrastructure", "read"), dockerHandler.InspectContainer)
		api.POST("/docker/containers/:id/edit", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.EditContainer)
		api.PUT("/docker/containers/:id", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.EditContainer)
		api.POST("/docker/containers/:id/start", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.StartContainer)
		api.POST("/docker/containers/:id/stop", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.StopContainer)
		api.POST("/docker/containers/:id/restart", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.RestartContainer)
		api.POST("/docker/containers/:id/pause", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.PauseContainer)
		api.POST("/docker/containers/:id/unpause", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.UnpauseContainer)
		api.DELETE("/docker/containers/:id", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.DeleteContainer)
		api.POST("/docker/containers/:id/visibility", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.UpdateVisibility)
		api.GET("/docker/containers/:id/shares", middleware.RequirePermission("infrastructure", "read"), dockerHandler.ListShares)
		api.POST("/docker/containers/:id/shares", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.AddShare)
		api.DELETE("/docker/containers/:id/shares/:userId", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.DeleteShare)
		api.GET("/docker/users", middleware.RequirePermission("infrastructure", "read"), dockerHandler.ListUsers)
		api.GET("/docker/containers/:id/logs", middleware.RequirePermission("infrastructure", "read"), dockerHandler.GetLogs)
		api.GET("/docker/containers/:id/stats", middleware.RequirePermission("infrastructure", "read"), dockerHandler.GetStats)
		api.POST("/docker/containers/deploy", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.DeployContainer)
		api.GET("/docker/images", middleware.RequirePermission("infrastructure", "read"), dockerHandler.ListImages)
		api.POST("/docker/images/pull", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.PullImage)
		api.DELETE("/docker/images/:id", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.DeleteImage)
		api.GET("/docker/networks", middleware.RequirePermission("infrastructure", "read"), dockerHandler.ListNetworks)
		api.POST("/docker/networks", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.CreateNetwork)
		api.DELETE("/docker/networks/:id", middleware.RequirePermission("infrastructure", "manage"), dockerHandler.DeleteNetwork)
		api.GET("/docker/system/info", middleware.RequirePermission("infrastructure", "read"), dockerHandler.GetSystemInfo)

		// Visual Reporting (Feature: reports)
		api.GET("/reports", middleware.RequirePermission("reports", "read"), reportHandler.ListReports)
		api.POST("/reports", middleware.RequirePermission("reports", "manage"), reportHandler.CreateReport)
		api.GET("/reports/:id", middleware.RequirePermission("reports", "read"), reportHandler.GetReport)
		api.PUT("/reports/:id", middleware.RequirePermission("reports", "manage"), reportHandler.UpdateReport)
		api.DELETE("/reports/:id", middleware.RequirePermission("reports", "manage"), reportHandler.DeleteReport)
		api.POST("/reports/:id/widgets", middleware.RequirePermission("reports", "manage"), reportHandler.CreateWidget)
		api.POST("/reports/widgets", middleware.RequirePermission("reports", "manage"), reportHandler.CreateWidget)
		api.PUT("/reports/widgets/:widgetId", middleware.RequirePermission("reports", "manage"), reportHandler.UpdateWidget)
		api.DELETE("/reports/widgets/:widgetId", middleware.RequirePermission("reports", "manage"), reportHandler.DeleteWidget)
		api.POST("/reports/query-data", middleware.RequirePermission("reports", "read"), reportHandler.QueryWidgetData)

		// Status Pages (Feature: status_pages)
		api.GET("/status-pages", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "read"), statusPageHandler.ListPages)
		api.POST("/status-pages", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.CreatePage)
		api.GET("/status-pages/sources", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "read"), statusPageHandler.GetSourceOptions)
		api.GET("/status-pages/:id", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "read"), statusPageHandler.GetPage)
		api.PUT("/status-pages/:id", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.UpdatePage)
		api.DELETE("/status-pages/:id", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.DeletePage)
		api.GET("/status-pages/:id/live", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "read"), statusPageHandler.GetLiveReportAdmin)
		api.POST("/status-pages/:id/groups", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.SaveGroup)
		api.DELETE("/status-pages/groups/:groupId", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.DeleteGroup)
		api.POST("/status-pages/:id/items", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.SaveItem)
		api.DELETE("/status-pages/items/:itemId", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.DeleteItem)
		api.POST("/status-pages/:id/incidents", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.SaveIncident)
		api.DELETE("/status-pages/incidents/:incidentId", middleware.RequireAnyPermission([]string{"status_pages", "slideshow", "monitoring_instances"}, "manage"), statusPageHandler.DeleteIncident)
	}

	// Serve Static Frontend files (if built in web/dist)
	webDist := filepath.Join(".", "web", "dist")
	if _, err := os.Stat(webDist); err == nil {
		r.Static("/assets", filepath.Join(webDist, "assets"))
		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(webDist, "index.html"))
		})
	}

	// 9. HTTP Server with Graceful Shutdown
	// Do NOT set ReadTimeout or WriteTimeout here as they sever hijacked WebSocket connections (SSH Terminal) and long downloads
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("Server", fmt.Sprintf("HTTP Server listening on http://localhost:%d", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server", "Failed to start HTTP server", err)
		}
	}()

	// Wait for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server", "Gracefully shutting down Hephaestus server...")
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	cronSched.Stop()
	workerPool.Stop()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error("Server", "Server forced shutdown", err)
	}

	logger.Info("Server", "Hephaestus server stopped cleanly.")
}
