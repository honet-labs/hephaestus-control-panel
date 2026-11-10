package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type EventRepository struct{}

func NewEventRepository() *EventRepository {
	return &EventRepository{}
}

// Create inserts a new system event into PostgreSQL
func (r *EventRepository) Create(ctx context.Context, e *domain.SystemEvent) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	metaJSON, _ := json.Marshal(e.Metadata)
	query := `
		INSERT INTO system_events (
			id, source_type, source_id, source_name, event_type,
			severity, title, message, metadata, status, started_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			message = EXCLUDED.message,
			metadata = EXCLUDED.metadata,
			severity = EXCLUDED.severity,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`
	now := time.Now().UTC()
	if e.StartedAt.IsZero() {
		e.StartedAt = now
	}
	e.CreatedAt = now
	e.UpdatedAt = now

	_, err = pool.Exec(ctx, query,
		e.ID, e.SourceType, e.SourceID, e.SourceName, e.EventType,
		e.Severity, e.Title, e.Message, string(metaJSON), e.Status, e.StartedAt, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

// ResolveActiveEvent marks any active event matching source and event_type as resolved
func (r *EventRepository) ResolveActiveEvent(ctx context.Context, sourceType, sourceID, eventType string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query := `
		UPDATE system_events
		SET status = 'resolved', resolved_at = $1, updated_at = $1
		WHERE source_type = $2 AND source_id = $3 AND event_type = $4 AND status = 'active'
	`
	_, err = pool.Exec(ctx, query, now, sourceType, sourceID, eventType)
	return err
}

// GetActiveEvents returns all currently active or acknowledged system events (for cache warm-up on startup)
func (r *EventRepository) GetActiveEvents(ctx context.Context) ([]domain.SystemEvent, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, source_type, source_id, source_name, event_type,
		       severity, title, message, metadata, status, started_at, resolved_at, created_at, updated_at
		FROM system_events
		WHERE status IN ('active', 'acknowledged')
		ORDER BY started_at DESC
		LIMIT 200
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.SystemEvent
	for rows.Next() {
		var e domain.SystemEvent
		var metaRaw []byte
		if err := rows.Scan(
			&e.ID, &e.SourceType, &e.SourceID, &e.SourceName, &e.EventType,
			&e.Severity, &e.Title, &e.Message, &metaRaw, &e.Status,
			&e.StartedAt, &e.ResolvedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, nil
}

// IsEventResolved checks whether an event for given source and event_type has already been resolved
func (r *EventRepository) IsEventResolved(ctx context.Context, sourceType, sourceID, eventType string) bool {
	pool, err := database.GetPool()
	if err != nil {
		return false
	}

	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM system_events 
			WHERE source_type = $1 AND source_id = $2 AND event_type = $3 AND status = 'resolved'
		)
	`
	_ = pool.QueryRow(ctx, query, sourceType, sourceID, eventType).Scan(&exists)
	return exists
}

// GetRecentResolvedKeys returns a map of keys for recently resolved events (within given duration)
func (r *EventRepository) GetRecentResolvedKeys(ctx context.Context, d time.Duration) (map[string]bool, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().UTC().Add(-d)
	query := `
		SELECT source_type, source_id, event_type
		FROM system_events
		WHERE status = 'resolved' AND (resolved_at >= $1 OR updated_at >= $1)
		LIMIT 1000
	`
	rows, err := pool.Query(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]bool)
	for rows.Next() {
		var st, sid, et string
		if err := rows.Scan(&st, &sid, &et); err == nil {
			key := fmt.Sprintf("%s:%s:%s", st, sid, et)
			res[key] = true
		}
	}
	return res, nil
}

// List retrieves paginated events with optional filters for status, severity, and source_type
func (r *EventRepository) List(ctx context.Context, status, severity, sourceType string, limit, offset int) ([]domain.SystemEvent, int, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if status != "" && status != "all" {
		whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if severity != "" && severity != "all" {
		whereClause += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, severity)
		argIdx++
	}
	if sourceType != "" && sourceType != "all" {
		whereClause += fmt.Sprintf(" AND source_type = $%d", argIdx)
		args = append(args, sourceType)
		argIdx++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM system_events %s", whereClause)
	var total int
	if err := pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`
		SELECT id, source_type, source_id, source_name, event_type,
		       severity, title, message, metadata, status, started_at, resolved_at, created_at, updated_at
		FROM system_events
		%s
		ORDER BY 
			CASE WHEN status = 'active' THEN 0 ELSE 1 END,
			started_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []domain.SystemEvent
	for rows.Next() {
		var e domain.SystemEvent
		var metaRaw []byte
		if err := rows.Scan(
			&e.ID, &e.SourceType, &e.SourceID, &e.SourceName, &e.EventType,
			&e.Severity, &e.Title, &e.Message, &metaRaw, &e.Status,
			&e.StartedAt, &e.ResolvedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, total, nil
}

// Acknowledge marks an active event as acknowledged
func (r *EventRepository) Acknowledge(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `UPDATE system_events SET status = 'acknowledged', updated_at = $1 WHERE id = $2`
	_, err = pool.Exec(ctx, query, time.Now().UTC(), id)
	return err
}

// Resolve marks an event as resolved by ID
func (r *EventRepository) Resolve(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query := `UPDATE system_events SET status = 'resolved', resolved_at = $1, updated_at = $1 WHERE id = $2`
	_, err = pool.Exec(ctx, query, now, id)
	return err
}

// ClearResolved removes resolved events older than a specified duration (e.g. 7 days)
func (r *EventRepository) ClearResolved(ctx context.Context, olderThanDays int) (int64, error) {
	pool, err := database.GetPool()
	if err != nil {
		return 0, err
	}

	if olderThanDays <= 0 {
		olderThanDays = 1
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -olderThanDays)
	query := `DELETE FROM system_events WHERE status = 'resolved' AND resolved_at < $1`
	cmdTag, err := pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return cmdTag.RowsAffected(), nil
}
