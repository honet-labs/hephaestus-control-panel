package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type EventService struct {
	eventRepo         *repository.EventRepository
	monService        *MonitoringInstanceService
	openSearchService *OpenSearchService
	backupRepo        *repository.BackupRepository

	cacheMu         sync.RWMutex
	cachedSummary   domain.EventSummary
	activeEventsMap map[string]*domain.SystemEvent // key: sourceType + ":" + sourceId + ":" + eventType
	stopChan        chan struct{}
}

func NewEventService(
	eventRepo *repository.EventRepository,
	monService *MonitoringInstanceService,
	openSearchService *OpenSearchService,
	backupRepo *repository.BackupRepository,
) *EventService {
	return &EventService{
		eventRepo:         eventRepo,
		monService:        monService,
		openSearchService: openSearchService,
		backupRepo:        backupRepo,
		activeEventsMap:   make(map[string]*domain.SystemEvent),
		stopChan:          make(chan struct{}),
	}
}

// StartBackgroundEngine starts the background evaluator and in-memory cache maintainer
func (s *EventService) StartBackgroundEngine() {
	go func() {
		// Warm-up on startup: load existing active events from DB into in-memory map
		warmupCtx, cancelWarmup := context.WithTimeout(context.Background(), 5*time.Second)
		s.warmup(warmupCtx)
		cancelWarmup()

		// Initial evaluation
		time.Sleep(3 * time.Second)
		evalCtx, cancelEval := context.WithTimeout(context.Background(), 10*time.Second)
		s.EvaluateAll(evalCtx)
		cancelEval()

		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				loopCtx, cancelLoop := context.WithTimeout(context.Background(), 15*time.Second)
				s.EvaluateAll(loopCtx)
				cancelLoop()
			}
		}
	}()
	logger.Info("EventEngine", "Unified Operational Event & Incident engine started (Interval: 20s)")
}

func (s *EventService) Stop() {
	close(s.stopChan)
}

// warmup loads active events from DB into memory cache
func (s *EventService) warmup(ctx context.Context) {
	events, err := s.eventRepo.GetActiveEvents(ctx)
	if err != nil {
		logger.Warn("EventEngine", fmt.Sprintf("Failed to warm-up active events: %v", err))
		return
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for i := range events {
		key := fmt.Sprintf("%s:%s:%s", events[i].SourceType, events[i].SourceID, events[i].EventType)
		s.activeEventsMap[key] = &events[i]
	}
	logger.Info("EventEngine", fmt.Sprintf("Warm-up complete: %d active events loaded into memory cache", len(events)))
}

// EvaluateAll runs an evaluation pass across all services using only already-cached data
func (s *EventService) EvaluateAll(ctx context.Context) {
	now := time.Now().UTC()

	var (
		critCount, warnCount, infoCount int
		instStats                       domain.InstanceEventStats
		ctrStats                        domain.ContainerEventStats
		osStats                         domain.OpenSearchEventStats
		bkStats                         domain.BackupEventStats
		activeList                      []domain.SystemEvent
	)

	// 1. Evaluate Server Monitoring Instances (O(N) memory read from existing monService cache)
	if s.monService != nil {
		instances, err := s.monService.List(ctx, 0, "ADMIN", "", "")
		if err == nil {
			instStats.Total = len(instances)
			for _, inst := range instances {
				if inst.LiveMetrics == nil || !inst.LiveMetrics.IsOnline {
					instStats.Offline++
					instStats.Critical++
					critCount++
					s.trackEvent(ctx, &activeList, "instance", inst.ID, inst.Name, "host_offline", "critical",
						"Host Offline / Unreachable",
						fmt.Sprintf("Server '%s' (%s) is offline or unreachable via telemetry.", inst.Name, inst.Host),
						map[string]interface{}{"host": inst.Host, "group": inst.GroupName},
					)
					continue
				}

				// Resolve offline event if back online
				s.resolveEvent(ctx, "instance", inst.ID, "host_offline")

				var cpu, mem, disk float64
				if inst.LiveMetrics.CPUPct != nil {
					cpu = *inst.LiveMetrics.CPUPct
				}
				if inst.LiveMetrics.MemPct != nil {
					mem = *inst.LiveMetrics.MemPct
				}
				if inst.LiveMetrics.DiskPct != nil {
					disk = *inst.LiveMetrics.DiskPct
				}

				maxUsage := cpu
				critRes := "CPU"
				if mem > maxUsage {
					maxUsage = mem
					critRes = "Memory"
				}
				if disk > maxUsage {
					maxUsage = disk
					critRes = "Storage"
				}

				if maxUsage >= 90 {
					instStats.Critical++
					critCount++
					s.trackEvent(ctx, &activeList, "instance", inst.ID, inst.Name, "resource_critical", "critical",
						fmt.Sprintf("Critical %s Utilization", critRes),
						fmt.Sprintf("Server '%s' reached %.1f%% %s utilization (Critical threshold: 90%%).", inst.Name, maxUsage, critRes),
						map[string]interface{}{"maxUsage": maxUsage, "resource": critRes, "host": inst.Host},
					)
				} else if maxUsage >= 80 {
					instStats.Warning++
					warnCount++
					s.trackEvent(ctx, &activeList, "instance", inst.ID, inst.Name, "resource_warning", "warning",
						fmt.Sprintf("High %s Utilization", critRes),
						fmt.Sprintf("Server '%s' reached %.1f%% %s utilization (Warning threshold: 80%%).", inst.Name, maxUsage, critRes),
						map[string]interface{}{"maxUsage": maxUsage, "resource": critRes, "host": inst.Host},
					)
					// Resolve critical if downgraded to warning
					s.resolveEvent(ctx, "instance", inst.ID, "resource_critical")
				} else {
					instStats.Healthy++
					s.resolveEvent(ctx, "instance", inst.ID, "resource_critical")
					s.resolveEvent(ctx, "instance", inst.ID, "resource_warning")
				}
			}
		}

		// 2. Evaluate Docker Containers (O(N) memory read from existing monService.dockerCache)
		containers, err := s.monService.ListDockerContainers(ctx, "")
		if err == nil {
			ctrStats.Total = len(containers)
			for _, c := range containers {
				st := strings.ToLower(c.State)
				if st == "exited" || st == "dead" {
					ctrStats.Stopped++
					ctrStats.Critical++
					critCount++
					s.trackEvent(ctx, &activeList, "container", c.ContainerID, c.ContainerName, "container_stopped", "critical",
						"Container Exited / Stopped",
						fmt.Sprintf("Docker container '%s' on %s is in '%s' state.", c.ContainerName, c.Hostname, c.State),
						map[string]interface{}{"image": c.ImageName, "host": c.Hostname, "state": c.State},
					)
					continue
				}

				// Resolve stopped event if running
				s.resolveEvent(ctx, "container", c.ContainerID, "container_stopped")

				cpu := c.CPUPct
				mem := c.MemPct
				maxUsage := cpu
				critRes := "CPU"
				if mem > maxUsage {
					maxUsage = mem
					critRes = "Memory"
				}

				if maxUsage >= 90 {
					ctrStats.Critical++
					critCount++
					s.trackEvent(ctx, &activeList, "container", c.ContainerID, c.ContainerName, "container_critical", "critical",
						fmt.Sprintf("Critical Container %s", critRes),
						fmt.Sprintf("Container '%s' reached %.1f%% %s usage.", c.ContainerName, maxUsage, critRes),
						map[string]interface{}{"maxUsage": maxUsage, "resource": critRes, "host": c.Hostname},
					)
				} else if maxUsage >= 80 {
					ctrStats.Warning++
					warnCount++
					s.trackEvent(ctx, &activeList, "container", c.ContainerID, c.ContainerName, "container_warning", "warning",
						fmt.Sprintf("High Container %s", critRes),
						fmt.Sprintf("Container '%s' reached %.1f%% %s usage.", c.ContainerName, maxUsage, critRes),
						map[string]interface{}{"maxUsage": maxUsage, "resource": critRes, "host": c.Hostname},
					)
					s.resolveEvent(ctx, "container", c.ContainerID, "container_critical")
				} else {
					ctrStats.Healthy++
					s.resolveEvent(ctx, "container", c.ContainerID, "container_critical")
					s.resolveEvent(ctx, "container", c.ContainerID, "container_warning")
				}
			}
		}
	}

	// 3. Evaluate OpenSearch / Elasticsearch Cluster Health
	osStats.Status = "unconfigured"
	if s.openSearchService != nil {
		activeOS, err := s.openSearchService.GetActiveConfig(ctx)
		if err == nil && activeOS != nil {
			osStats.ClusterName = activeOS.Name
			health, errHealth := s.openSearchService.GetClusterHealth(ctx)
			if errHealth != nil {
				osStats.Status = "offline"
				critCount++
				s.trackEvent(ctx, &activeList, "opensearch", activeOS.ID, activeOS.Name, "cluster_offline", "critical",
					"OpenSearch Cluster Unreachable",
					fmt.Sprintf("Failed to connect to OpenSearch cluster '%s' (%s:%d): %v", activeOS.Name, activeOS.Host, activeOS.Port, errHealth),
					map[string]interface{}{"host": activeOS.Host, "port": activeOS.Port},
				)
			} else {
				s.resolveEvent(ctx, "opensearch", activeOS.ID, "cluster_offline")
				statusStr := strings.ToLower(fmt.Sprintf("%v", health["status"]))
				osStats.Status = statusStr

				if nodes, ok := health["number_of_nodes"].(float64); ok {
					osStats.ActiveNodes = int(nodes)
				}
				if unassigned, ok := health["unassigned_shards"].(float64); ok {
					osStats.UnassignedShards = int(unassigned)
				}

				if statusStr == "red" {
					critCount++
					s.trackEvent(ctx, &activeList, "opensearch", activeOS.ID, activeOS.Name, "cluster_red", "critical",
						"OpenSearch Cluster Health RED",
						fmt.Sprintf("Cluster '%s' is in RED state with %d unassigned shards! Primary shards are unavailable.", activeOS.Name, osStats.UnassignedShards),
						health,
					)
				} else if statusStr == "yellow" {
					warnCount++
					s.trackEvent(ctx, &activeList, "opensearch", activeOS.ID, activeOS.Name, "cluster_yellow", "warning",
						"OpenSearch Cluster Health YELLOW",
						fmt.Sprintf("Cluster '%s' is in YELLOW state with %d unassigned replica shards.", activeOS.Name, osStats.UnassignedShards),
						health,
					)
					s.resolveEvent(ctx, "opensearch", activeOS.ID, "cluster_red")
				} else {
					s.resolveEvent(ctx, "opensearch", activeOS.ID, "cluster_red")
					s.resolveEvent(ctx, "opensearch", activeOS.ID, "cluster_yellow")
				}
			}
		}
	}

	// 4. Evaluate Backup & Restore Operations (Last 24 Hours)
	if s.backupRepo != nil {
		history, _, err := s.backupRepo.ListHistory(ctx, 30, 0)
		if err == nil {
			cutoff := now.Add(-24 * time.Hour)
			for _, b := range history {
				if b.StartedAt.Before(cutoff) {
					continue
				}
				bkStats.Total24h++
				switch strings.ToLower(b.Status) {
				case "failed", "error":
					bkStats.Failed24h++
					critCount++
					s.trackEvent(ctx, &activeList, "backup", b.ID, b.DBName, "backup_failed", "critical",
						"Backup Job Failed",
						fmt.Sprintf("Database/Container backup for '%s' failed: %s", b.DBName, b.ErrorMessage),
						map[string]interface{}{"dbType": b.DBType, "destType": b.DestType, "startedAt": b.StartedAt},
					)
				case "success", "completed":
					bkStats.Success24h++
				case "running":
					bkStats.Running++
				}
			}
		}
	}

	// 5. Update In-Memory Summary Cache (Thread-Safe atomic update)
	s.cacheMu.Lock()
	s.cachedSummary = domain.EventSummary{
		CriticalCount: critCount,
		WarningCount:  warnCount,
		InfoCount:     infoCount,
		Instances:     instStats,
		Containers:    ctrStats,
		OpenSearch:    osStats,
		Backups:       bkStats,
		ActiveEvents:  activeList,
		LastCheckedAt: now,
	}
	s.cacheMu.Unlock()
}

// trackEvent records or maintains an active event using edge-triggering (only inserts when new)
func (s *EventService) trackEvent(
	ctx context.Context,
	activeList *[]domain.SystemEvent,
	sourceType, sourceID, sourceName, eventType, severity, title, message string,
	metadata map[string]interface{},
) {
	key := fmt.Sprintf("%s:%s:%s", sourceType, sourceID, eventType)

	s.cacheMu.Lock()
	existing, exists := s.activeEventsMap[key]
	if !exists {
		// New event: Edge-triggered write to DB
		now := time.Now().UTC()
		newEvent := &domain.SystemEvent{
			ID:         "evt-" + uuid.New().String()[:8],
			SourceType: sourceType,
			SourceID:   sourceID,
			SourceName: sourceName,
			EventType:  eventType,
			Severity:   severity,
			Title:      title,
			Message:    message,
			Metadata:   metadata,
			Status:     "active",
			StartedAt:  now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		s.activeEventsMap[key] = newEvent
		s.cacheMu.Unlock()

		_ = s.eventRepo.Create(ctx, newEvent)
		*activeList = append(*activeList, *newEvent)
		logger.Warn("EventEngine", fmt.Sprintf("[%s] %s: %s", strings.ToUpper(severity), title, message))
		return
	}

	// Already active: keep memory copy up to date
	existing.Message = message
	existing.Severity = severity
	existing.Metadata = metadata
	existing.UpdatedAt = time.Now().UTC()
	item := *existing
	s.cacheMu.Unlock()

	*activeList = append(*activeList, item)
}

// resolveEvent marks an event as resolved if it was previously active
func (s *EventService) resolveEvent(ctx context.Context, sourceType, sourceID, eventType string) {
	key := fmt.Sprintf("%s:%s:%s", sourceType, sourceID, eventType)

	s.cacheMu.Lock()
	existing, exists := s.activeEventsMap[key]
	if !exists {
		s.cacheMu.Unlock()
		return
	}
	delete(s.activeEventsMap, key)
	s.cacheMu.Unlock()

	_ = s.eventRepo.ResolveActiveEvent(ctx, sourceType, sourceID, eventType)
	logger.Info("EventEngine", fmt.Sprintf("[RESOLVED] Event '%s' on %s:%s has cleared.", eventType, sourceType, sourceID))
	_ = existing
}

// GetSummary returns the in-memory cached event summary in microseconds (< 1ms, zero DB load)
func (s *EventService) GetSummary() domain.EventSummary {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.cachedSummary
}

// ListEvents queries the database with pagination and filters
func (s *EventService) ListEvents(ctx context.Context, status, severity, sourceType string, limit, offset int) ([]domain.SystemEvent, int, error) {
	return s.eventRepo.List(ctx, status, severity, sourceType, limit, offset)
}

// AcknowledgeEvent marks an event as acknowledged
func (s *EventService) AcknowledgeEvent(ctx context.Context, id string) error {
	return s.eventRepo.Acknowledge(ctx, id)
}

// ResolveEvent manually marks an event as resolved
func (s *EventService) ResolveEvent(ctx context.Context, id string) error {
	return s.eventRepo.Resolve(ctx, id)
}

// ClearResolved removes old resolved events
func (s *EventService) ClearResolved(ctx context.Context, olderThanDays int) (int64, error) {
	return s.eventRepo.ClearResolved(ctx, olderThanDays)
}
