package scheduler

import (
	"context"
	"fmt"
	"sync"

	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"

	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	cron       *cron.Cron
	backupRepo *repository.BackupRepository
	userRepo   *repository.UserRepository
	entryMap   map[string]cron.EntryID
	mu         sync.Mutex
}

var (
	defaultScheduler *CronScheduler
	schedOnce        sync.Once
)

func InitCronScheduler(backupRepo *repository.BackupRepository, userRepo *repository.UserRepository) *CronScheduler {
	schedOnce.Do(func() {
		c := cron.New(cron.WithSeconds())
		defaultScheduler = &CronScheduler{
			cron:       c,
			backupRepo: backupRepo,
			userRepo:   userRepo,
			entryMap:   make(map[string]cron.EntryID),
		}

		defaultScheduler.registerSystemJobs()
		defaultScheduler.ReloadBackupSchedules(context.Background())
		c.Start()
		logger.Info("Cron", "Cron scheduler engine started.")
	})
	return defaultScheduler
}

func GetScheduler() *CronScheduler {
	return defaultScheduler
}

func (s *CronScheduler) registerSystemJobs() {
	// 1. Periodic expired sessions cleanup (Every 6 hours)
	_, err := s.cron.AddFunc("0 0 */6 * * *", func() {
		ctx := context.Background()
		cleaned, err := s.userRepo.CleanExpiredSessions(ctx)
		if err != nil {
			logger.Error("Cron", "Failed to clean expired sessions", err)
		} else if cleaned > 0 {
			logger.Info("Cron", fmt.Sprintf("Cleaned up %d expired user sessions", cleaned))
		}
	})
	if err != nil {
		logger.Error("Cron", "Failed to schedule session cleanup job", err)
	}

	// 2. Periodic ICMP Device Ping Cycle (Every 60 seconds)
	_, err = s.cron.AddFunc("0 * * * * *", func() {
		wp := queue.GetWorkerPool()
		_, _ = wp.Enqueue("icmp_ping_cycle", map[string]interface{}{}, 0)
	})
	if err != nil {
		logger.Error("Cron", "Failed to schedule ICMP ping cycle", err)
	}

	// 3. Periodic OpenSearch Telemetry Poll (Every 30 seconds)
	_, err = s.cron.AddFunc("*/30 * * * * *", func() {
		wp := queue.GetWorkerPool()
		_, _ = wp.Enqueue("opensearch_poll", map[string]interface{}{}, 0)
	})
	if err != nil {
		logger.Error("Cron", "Failed to schedule OpenSearch poll cycle", err)
	}
}

// ReloadBackupSchedules reloads active backup schedules from database
func (s *CronScheduler) ReloadBackupSchedules(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove previously registered backup schedule entries
	for id, entryID := range s.entryMap {
		s.cron.Remove(entryID)
		delete(s.entryMap, id)
	}

	schedules, err := s.backupRepo.ListSchedules(ctx)
	if err != nil {
		logger.Error("Cron", "Failed to list backup schedules from DB", err)
		return
	}

	for _, sched := range schedules {
		if !sched.IsActive {
			continue
		}

		// Handle standard 5-part cron syntax or 6-part (with seconds)
		cronExpr := sched.CronExpression
		// If 5 parts, prefix with "0 " for robfig/cron withSeconds
		if len(splitFields(cronExpr)) == 5 {
			cronExpr = "0 " + cronExpr
		}

		schedCopy := sched
		entryID, err := s.cron.AddFunc(cronExpr, func() {
			logger.Info("Cron", fmt.Sprintf("Triggering scheduled backup: %s (DB: %s)", schedCopy.Name, schedCopy.DBConfigID))
			wp := queue.GetWorkerPool()
			_, _ = wp.Enqueue("database_backup", map[string]interface{}{
				"dbConfigId":    schedCopy.DBConfigID,
				"destinationId": schedCopy.DestinationID,
				"scheduleId":    schedCopy.ID,
			}, 1)
			_ = s.backupRepo.UpdateScheduleRuns(context.Background(), schedCopy.ID)
		})

		if err != nil {
			logger.Error("Cron", fmt.Sprintf("Invalid cron expression '%s' for schedule '%s'", sched.CronExpression, sched.Name), err)
		} else {
			s.entryMap[sched.ID] = entryID
			logger.Info("Cron", fmt.Sprintf("Registered backup schedule: '%s' [%s]", sched.Name, sched.CronExpression))
		}
	}
}

func (s *CronScheduler) Stop() {
	s.cron.Stop()
	logger.Info("Cron", "Cron scheduler stopped.")
}

func splitFields(s string) []string {
	var fields []string
	curr := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			if curr != "" {
				fields = append(fields, curr)
				curr = ""
			}
		} else {
			curr += string(ch)
		}
	}
	if curr != "" {
		fields = append(fields, curr)
	}
	return fields
}
