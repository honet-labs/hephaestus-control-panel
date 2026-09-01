package services

import (
	"context"
	"runtime"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type SystemService struct {
	startTime time.Time
	version   string
}

func NewSystemService() *SystemService {
	return &SystemService{
		startTime: time.Now(),
		version:   "2.0.0",
	}
}

func (s *SystemService) GetStats(ctx context.Context) domain.SystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbStatus := "DISCONNECTED"
	if database.IsConnected() {
		dbStatus = "CONNECTED"
	}

	return domain.SystemStats{
		AppVersion:     s.version,
		UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
		GoroutineCount: runtime.NumGoroutine(),
		MemoryAllocMB:  float64(m.Alloc) / 1024 / 1024,
		MemoryTotalMB:  float64(m.Sys) / 1024 / 1024,
		CPUUsagePct:    0.5, // Lightweight Go baseline
		DatabaseStatus: dbStatus,
		Timestamp:      time.Now(),
	}
}
