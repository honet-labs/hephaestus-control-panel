package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
	// 1. Load Configurations
	cfg := config.LoadConfig()

	// 2. Initialize Structured Logger with file rotation
	logger.InitLogger(cfg.LogsDir)
	logger.Info("Server", fmt.Sprintf("Starting Hephaestus Control Panel (HCP) v2.0.0 on port %d...", cfg.Port))

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

	// 5. Initialize Background Worker Pool & Scheduler
	workerPool := queue.InitWorkerPool(5)
	cronSched := scheduler.InitCronScheduler(backupRepo, userRepo)

	// 6. Initialize Services
	authService := services.NewAuthService(userRepo, configRepo)
	sshService := services.NewSSHService(remoteRepo)
	wsService := services.NewWsTerminalService(sshService)
	backupService := services.NewBackupService(backupRepo, sshService)
	snmpService := services.NewSnmpService(snmpRepo, cfg.DataDir)
	_ = services.NewIcmpPingService(topologyRepo) // Registers ICMP worker
	topologyService := services.NewTopologyService(topologyRepo, configRepo)
	openSearchService := services.NewOpenSearchService()
	promService := services.NewPrometheusService(configRepo, sshService)
	vpsService := services.NewVpsService(remoteRepo, sshService)
	grokService := services.NewGrokService()
	dpService := services.NewDataPrepperService(sshService)
	systemService := services.NewSystemService()

	// 7. Initialize HTTP Handlers
	authHandler := handlers.NewAuthHandler(authService)
	setupHandler := handlers.NewSetupHandler(authService)
	remoteHostHandler := handlers.NewRemoteHostHandler(remoteRepo, sshService, wsService, authService)
	topologyHandler := handlers.NewTopologyHandler(topologyRepo, topologyService)
	backupHandler := handlers.NewBackupHandler(backupRepo, backupService, cronSched)
	snmpHandler := handlers.NewSnmpHandler(snmpRepo, snmpService)
	openSearchHandler := handlers.NewOpenSearchHandler(openSearchService)
	promHandler := handlers.NewPrometheusHandler(promService)
	vpsHandler := handlers.NewVpsHandler(vpsService)
	grokHandler := handlers.NewGrokHandler(grokService)
	dpHandler := handlers.NewDataPrepperHandler(dpService)
	settingsHandler := handlers.NewSettingsHandler(configRepo, userRepo, systemService)
	logsHandler := handlers.NewLogsHandler()
	queueHandler := handlers.NewQueueHandler()

	// 8. Gin Router Setup
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLoggerMiddleware())

	// CORS Setup - Allow dynamic origin resolution for custom host IP, domain, and ports
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOriginFunc = func(origin string) bool {
		return true
	}
	corsConfig.AllowCredentials = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-Requested-With"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	// Healthcheck & Metrics
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "UP",
			"service":   "Hephaestus Control Panel (HCP)",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"database":  database.IsConnected(),
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

	// WebSocket Endpoints (WebSocket handles auth via handshake message)
	r.GET("/ws/remote-host", remoteHostHandler.HandleWebSocketTerminal)
	r.GET("/ws/logs", logsHandler.StreamLogsWebSocket)

	// Protected API Routes (with general rate limiting)
	api := r.Group("/api/v1")
	api.Use(middleware.GeneralRateLimitMiddleware())
	api.Use(middleware.AuthMiddleware(authService))
	{
		// User & Profile
		api.GET("/auth/me", authHandler.GetCurrentUser)
		api.POST("/auth/change-password", authHandler.ChangePassword)

		// Remote Hosts & SFTP
		api.GET("/remote-host", remoteHostHandler.List)
		api.GET("/remote-host/:id", remoteHostHandler.GetByID)
		api.POST("/remote-host", remoteHostHandler.Save)
		api.DELETE("/remote-host/:id", remoteHostHandler.Delete)
		api.POST("/remote-host/test", remoteHostHandler.TestConnection)
		api.GET("/remote-host/:id/sftp/list", remoteHostHandler.SftpList)
		api.POST("/remote-host/:id/sftp/upload", remoteHostHandler.SftpUpload)
		api.GET("/remote-host/:id/sftp/download", remoteHostHandler.SftpDownload)

		// Topology
		api.GET("/topology", topologyHandler.GetGraph)
		api.GET("/topology/sheets", topologyHandler.ListSheets)
		api.POST("/topology/sheets", topologyHandler.CreateSheet)
		api.PUT("/topology/sheets/:id", topologyHandler.UpdateSheet)
		api.DELETE("/topology/sheets/:id", topologyHandler.DeleteSheet)
		api.POST("/topology/devices", topologyHandler.SaveDevice)
		api.PUT("/topology/devices/:id/position", topologyHandler.UpdatePosition)
		api.DELETE("/topology/devices/:id", topologyHandler.DeleteDevice)
		api.POST("/topology/edges", topologyHandler.SaveEdge)
		api.DELETE("/topology/edges/:id", topologyHandler.DeleteEdge)
		api.GET("/topology/discover/prometheus", topologyHandler.DiscoverPrometheus)
		api.GET("/topology/discover/subnet", topologyHandler.ScanSubnet)

		// Backups
		api.GET("/backup/databases", backupHandler.ListDBConfigs)
		api.POST("/backup/databases", backupHandler.SaveDBConfig)
		api.DELETE("/backup/databases/:id", backupHandler.DeleteDBConfig)
		api.GET("/backup/destinations", backupHandler.ListDestinations)
		api.POST("/backup/destinations", backupHandler.SaveDestination)
		api.DELETE("/backup/destinations/:id", backupHandler.DeleteDestination)
		api.GET("/backup/schedules", backupHandler.ListSchedules)
		api.POST("/backup/schedules", backupHandler.SaveSchedule)
		api.DELETE("/backup/schedules/:id", backupHandler.DeleteSchedule)
		api.POST("/backup/run", backupHandler.RunBackup)
		api.GET("/backup/history", backupHandler.ListHistory)
		api.DELETE("/backup/history/:id", backupHandler.DeleteHistory)

		// SNMP
		api.POST("/snmp/query", snmpHandler.Query)
		api.GET("/snmp/mibs", snmpHandler.ListMibs)
		api.POST("/snmp/mibs", snmpHandler.ImportMib)
		api.DELETE("/snmp/mibs/:name", snmpHandler.DeleteMib)
		api.GET("/snmp/translate", snmpHandler.TranslateOID)

		// OpenSearch Cluster Management
		api.GET("/opensearch/health", openSearchHandler.GetHealth)
		api.GET("/opensearch/nodes", openSearchHandler.GetNodesStats)
		api.GET("/opensearch/nodes/info", openSearchHandler.GetNodesInfo)
		api.GET("/opensearch/indices", openSearchHandler.GetIndices)
		api.GET("/opensearch/shards", openSearchHandler.GetShards)
		api.GET("/opensearch/config", openSearchHandler.GetConfig)
		api.POST("/opensearch/config", openSearchHandler.SaveConfig)
		api.POST("/opensearch/test", openSearchHandler.TestConnection)

		// Prometheus & PromQL
		api.GET("/prometheus/query", promHandler.Query)
		api.POST("/prometheus/reload", promHandler.Reload)

		// VPS Telemetry, Processes, and Services
		api.GET("/vps/:id/metrics", vpsHandler.GetMetrics)
		api.GET("/vps/:id/processes", vpsHandler.GetProcesses)
		api.POST("/vps/:id/processes/:pid/kill", vpsHandler.KillProcess)
		api.GET("/vps/:id/services", vpsHandler.GetServices)
		api.POST("/vps/:id/control", vpsHandler.ControlService)

		// Grok Debugger
		api.POST("/grok/test", grokHandler.Test)
		api.GET("/grok/patterns", grokHandler.GetPatterns)

		// Data Prepper
		api.GET("/dataprepper/pipelines", dpHandler.ListPipelines)
		api.POST("/dataprepper/validate", dpHandler.ValidateYAML)

		// Live Logs & Queue
		api.GET("/logs/recent", logsHandler.GetRecentLogs)
		api.GET("/queue/jobs", queueHandler.ListJobs)
		api.GET("/queue/jobs/:id", queueHandler.GetJob)
		api.POST("/queue/jobs/:id/cancel", queueHandler.CancelJob)

		// Settings & System Stats
		api.GET("/settings/system", settingsHandler.GetSystemStats)
		api.GET("/settings/activity-logs", settingsHandler.ListActivityLogs)
		api.GET("/settings/users", settingsHandler.ListUsers)
		api.POST("/settings/users", settingsHandler.CreateUser)
		api.DELETE("/settings/users/:id", settingsHandler.DeleteUser)
		api.GET("/settings/grafana", settingsHandler.ListGrafana)
		api.POST("/settings/grafana", settingsHandler.SaveGrafana)
		api.POST("/settings/grafana/:id/active", settingsHandler.SetActiveGrafana)
		api.DELETE("/settings/grafana/:id", settingsHandler.DeleteGrafana)
		api.GET("/settings/prometheus", settingsHandler.ListPrometheus)
		api.POST("/settings/prometheus", settingsHandler.SavePrometheus)
		api.POST("/settings/prometheus/:id/active", settingsHandler.SetActivePrometheus)
		api.DELETE("/settings/prometheus/:id", settingsHandler.DeletePrometheus)
		api.GET("/settings/database", settingsHandler.GetDatabaseConfig)
		api.POST("/settings/database", settingsHandler.UpdateDatabaseConfig)
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
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
