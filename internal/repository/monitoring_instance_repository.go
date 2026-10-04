package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MonitoringInstanceRepository struct{}

func NewMonitoringInstanceRepository() *MonitoringInstanceRepository {
	return &MonitoringInstanceRepository{}
}

func (r *MonitoringInstanceRepository) List(ctx context.Context, userID int, userRole string, groupFilter string, tagFilter string) ([]*domain.MonitoringInstance, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)

	var query strings.Builder
	var args []interface{}
	argIdx := 1

	if isAdmin {
		query.WriteString(`
			SELECT 
				m.id, m.name, m.host, COALESCE(m.ip_address, ''), m.port, m.instance_type, 
				m.group_name, m.tags, COALESCE(m.prometheus_target, ''), m.remote_host_id, 
				CASE 
					WHEN m.metric_source = 'prometheus' THEN 'prometheus'
					WHEN m.remote_host_id IS NOT NULL AND m.remote_host_id != '' THEN 'ssh'
					WHEN m.metric_source = 'ssh' THEN 'ssh'
					ELSE 'prometheus'
				END AS metric_source, m.last_metrics, m.last_metrics_at,
				m.user_id, COALESCE(u.username, 'Admin') AS owner_username, m.visibility, 
				m.alert_enabled, COALESCE(m.notes, ''),
				(m.user_id = $1 OR m.user_id IS NULL) AS is_owner,
				'manage' AS user_permission,
				(SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count,
				m.created_at, m.updated_at
			FROM monitoring_instances m
			LEFT JOIN users u ON m.user_id = u.id
			WHERE 1=1
		`)
		args = append(args, userID)
		argIdx++
	} else {
		query.WriteString(`
			SELECT 
				m.id, m.name, m.host, COALESCE(m.ip_address, ''), m.port, m.instance_type, 
				m.group_name, m.tags, COALESCE(m.prometheus_target, ''), m.remote_host_id, 
				CASE 
					WHEN m.metric_source = 'prometheus' THEN 'prometheus'
					WHEN m.remote_host_id IS NOT NULL AND m.remote_host_id != '' THEN 'ssh'
					WHEN m.metric_source = 'ssh' THEN 'ssh'
					ELSE 'prometheus'
				END AS metric_source, m.last_metrics, m.last_metrics_at,
				m.user_id, COALESCE(u.username, 'System') AS owner_username, m.visibility, 
				m.alert_enabled, COALESCE(m.notes, ''),
				(m.user_id = $1) AS is_owner,
				CASE 
					WHEN m.user_id = $1 THEN 'manage'
					WHEN mis.permission IS NOT NULL THEN mis.permission
					ELSE 'read'
				END AS user_permission,
				(SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count,
				m.created_at, m.updated_at
			FROM monitoring_instances m
			LEFT JOIN users u ON m.user_id = u.id
			LEFT JOIN monitoring_instance_shares mis ON m.id = mis.instance_id AND mis.user_id = $1
			WHERE (m.user_id = $1 OR m.visibility = 'public' OR mis.user_id = $1)
		`)
		args = append(args, userID)
		argIdx++
	}

	if groupFilter != "" && groupFilter != "all" {
		query.WriteString(fmt.Sprintf(" AND m.group_name = $%d", argIdx))
		args = append(args, groupFilter)
		argIdx++
	}

	if tagFilter != "" && tagFilter != "all" {
		query.WriteString(fmt.Sprintf(" AND $%d = ANY(m.tags)", argIdx))
		args = append(args, tagFilter)
		argIdx++
	}

	query.WriteString(" ORDER BY m.group_name ASC, m.name ASC")

	rows, err := pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitoring instances: %w", err)
	}
	defer rows.Close()

	var list []*domain.MonitoringInstance
	for rows.Next() {
		inst := &domain.MonitoringInstance{}
		var tags []string
		var lastMetricsBytes []byte
		if err := rows.Scan(
			&inst.ID, &inst.Name, &inst.Host, &inst.IPAddress, &inst.Port, &inst.InstanceType,
			&inst.GroupName, &tags, &inst.PrometheusTarget, &inst.RemoteHostID,
			&inst.MetricSource, &lastMetricsBytes, &inst.LastMetricsAt,
			&inst.UserID, &inst.OwnerUsername, &inst.Visibility,
			&inst.AlertEnabled, &inst.Notes,
			&inst.IsOwner, &inst.UserPermission, &inst.SharesCount,
			&inst.CreatedAt, &inst.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan monitoring instance: %w", err)
		}
		if tags == nil {
			tags = []string{}
		}
		inst.Tags = tags
		if len(lastMetricsBytes) > 0 {
			var lm domain.InstanceLiveMetrics
			if err := json.Unmarshal(lastMetricsBytes, &lm); err == nil {
				inst.LiveMetrics = &lm
			}
		}
		list = append(list, inst)
	}

	return list, nil
}

func (r *MonitoringInstanceRepository) GetByID(ctx context.Context, id string, userID int, userRole string) (*domain.MonitoringInstance, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)

	var query string
	if isAdmin {
		query = `
			SELECT 
				m.id, m.name, m.host, COALESCE(m.ip_address, ''), m.port, m.instance_type, 
				m.group_name, m.tags, COALESCE(m.prometheus_target, ''), m.remote_host_id, 
				CASE 
					WHEN m.metric_source = 'prometheus' THEN 'prometheus'
					WHEN m.remote_host_id IS NOT NULL AND m.remote_host_id != '' THEN 'ssh'
					WHEN m.metric_source = 'ssh' THEN 'ssh'
					ELSE 'prometheus'
				END AS metric_source, m.last_metrics, m.last_metrics_at,
				m.user_id, COALESCE(u.username, 'Admin') AS owner_username, m.visibility, 
				m.alert_enabled, COALESCE(m.notes, ''),
				(m.user_id = $1 OR m.user_id IS NULL) AS is_owner,
				'manage' AS user_permission,
				(SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count,
				m.created_at, m.updated_at
			FROM monitoring_instances m
			LEFT JOIN users u ON m.user_id = u.id
			WHERE m.id = $2
		`
	} else {
		query = `
			SELECT 
				m.id, m.name, m.host, COALESCE(m.ip_address, ''), m.port, m.instance_type, 
				m.group_name, m.tags, COALESCE(m.prometheus_target, ''), m.remote_host_id, 
				CASE 
					WHEN m.metric_source = 'prometheus' THEN 'prometheus'
					WHEN m.remote_host_id IS NOT NULL AND m.remote_host_id != '' THEN 'ssh'
					WHEN m.metric_source = 'ssh' THEN 'ssh'
					ELSE 'prometheus'
				END AS metric_source, m.last_metrics, m.last_metrics_at,
				m.user_id, COALESCE(u.username, 'System') AS owner_username, m.visibility, 
				m.alert_enabled, COALESCE(m.notes, ''),
				(m.user_id = $1) AS is_owner,
				CASE 
					WHEN m.user_id = $1 THEN 'manage'
					WHEN mis.permission IS NOT NULL THEN mis.permission
					ELSE 'read'
				END AS user_permission,
				(SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count,
				m.created_at, m.updated_at
			FROM monitoring_instances m
			LEFT JOIN users u ON m.user_id = u.id
			LEFT JOIN monitoring_instance_shares mis ON m.id = mis.instance_id AND mis.user_id = $1
			WHERE m.id = $2 AND (m.user_id = $1 OR m.visibility = 'public' OR mis.user_id = $1)
		`
	}

	inst := &domain.MonitoringInstance{}
	var tags []string
	var lastMetricsBytes []byte
	err = pool.QueryRow(ctx, query, userID, id).Scan(
		&inst.ID, &inst.Name, &inst.Host, &inst.IPAddress, &inst.Port, &inst.InstanceType,
		&inst.GroupName, &tags, &inst.PrometheusTarget, &inst.RemoteHostID,
		&inst.MetricSource, &lastMetricsBytes, &inst.LastMetricsAt,
		&inst.UserID, &inst.OwnerUsername, &inst.Visibility,
		&inst.AlertEnabled, &inst.Notes,
		&inst.IsOwner, &inst.UserPermission, &inst.SharesCount,
		&inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("monitoring instance not found or access denied")
		}
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	inst.Tags = tags
	if len(lastMetricsBytes) > 0 {
		var lm domain.InstanceLiveMetrics
		if err := json.Unmarshal(lastMetricsBytes, &lm); err == nil {
			inst.LiveMetrics = &lm
		}
	}

	return inst, nil
}

func (r *MonitoringInstanceRepository) Create(ctx context.Context, inst *domain.MonitoringInstance) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if inst.ID == "" {
		inst.ID = fmt.Sprintf("inst-%s", uuid.New().String()[:8])
	}
	if inst.GroupName == "" {
		inst.GroupName = "Default"
	}
	if inst.InstanceType == "" {
		inst.InstanceType = "server"
	}
	if inst.Visibility == "" {
		inst.Visibility = "private"
	}
	if inst.Port == 0 {
		inst.Port = 8889
	}
	if inst.Tags == nil {
		inst.Tags = []string{}
	}

	if inst.MetricSource == "" {
		inst.MetricSource = "ssh"
	}

	query := `
		INSERT INTO monitoring_instances (
			id, name, host, ip_address, port, instance_type, group_name, tags,
			prometheus_target, remote_host_id, metric_source, user_id, visibility, alert_enabled,
			notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		RETURNING created_at, updated_at
	`

	return pool.QueryRow(ctx, query,
		inst.ID, inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.MetricSource, inst.UserID, inst.Visibility, inst.AlertEnabled, inst.Notes,
	).Scan(&inst.CreatedAt, &inst.UpdatedAt)
}

func (r *MonitoringInstanceRepository) Update(ctx context.Context, inst *domain.MonitoringInstance) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if inst.GroupName == "" {
		inst.GroupName = "Default"
	}
	if inst.Tags == nil {
		inst.Tags = []string{}
	}
	if inst.MetricSource == "" {
		inst.MetricSource = "ssh"
	}

	query := `
		UPDATE monitoring_instances SET
			name = $1,
			host = $2,
			ip_address = $3,
			port = $4,
			instance_type = $5,
			group_name = $6,
			tags = $7,
			prometheus_target = $8,
			remote_host_id = $9,
			metric_source = $10,
			visibility = $11,
			alert_enabled = $12,
			notes = $13,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $14
	`

	cmdTag, err := pool.Exec(ctx, query,
		inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.MetricSource, inst.Visibility, inst.AlertEnabled, inst.Notes, inst.ID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("instance not found or no changes made")
	}
	return nil
}

func (r *MonitoringInstanceRepository) Delete(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM monitoring_instances WHERE id = $1`
	cmdTag, err := pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("instance not found")
	}
	return nil
}

func (r *MonitoringInstanceRepository) ToggleAlert(ctx context.Context, id string, alertEnabled bool) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `UPDATE monitoring_instances SET alert_enabled = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err = pool.Exec(ctx, query, alertEnabled, id)
	return err
}

func (r *MonitoringInstanceRepository) UpdateVisibility(ctx context.Context, id string, visibility string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `UPDATE monitoring_instances SET visibility = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err = pool.Exec(ctx, query, visibility, id)
	return err
}

func (r *MonitoringInstanceRepository) CheckAccess(ctx context.Context, id string, userID int, userRole string) (hasAccess bool, isOwner bool, perm string, err error) {
	pool, err := database.GetPool()
	if err != nil {
		return false, false, "", err
	}

	if domain.IsAdminRole(userRole) {
		return true, true, "manage", nil
	}

	var ownerID *int
	var visibility string
	err = pool.QueryRow(ctx, "SELECT user_id, visibility FROM monitoring_instances WHERE id = $1", id).Scan(&ownerID, &visibility)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, "", nil
		}
		return false, false, "", err
	}

	if ownerID == nil || *ownerID == userID || strings.ToUpper(userRole) == "OPERATOR" {
		return true, true, "manage", nil
	}

	var sharePerm string
	err = pool.QueryRow(ctx, "SELECT permission FROM monitoring_instance_shares WHERE instance_id = $1 AND user_id = $2", id, userID).Scan(&sharePerm)
	if err == nil {
		return true, false, sharePerm, nil
	}

	if visibility == "public" {
		return true, false, "read", nil
	}

	return false, false, "", nil
}

func (r *MonitoringInstanceRepository) ListShares(ctx context.Context, instanceID string) ([]domain.MonitoringInstanceShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			s.id, s.instance_id, s.user_id, COALESCE(u.username, ''), s.permission, 
			s.shared_by, COALESCE(sb.username, ''), s.created_at
		FROM monitoring_instance_shares s
		LEFT JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE s.instance_id = $1
		ORDER BY s.created_at ASC
	`

	rows, err := pool.Query(ctx, query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []domain.MonitoringInstanceShare
	for rows.Next() {
		var s domain.MonitoringInstanceShare
		if err := rows.Scan(
			&s.ID, &s.InstanceID, &s.UserID, &s.Username, &s.Permission,
			&s.SharedBy, &s.SharedByUsername, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}

	return shares, nil
}

func (r *MonitoringInstanceRepository) AddShare(ctx context.Context, share *domain.MonitoringInstanceShare) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if share.ID == "" {
		share.ID = fmt.Sprintf("mishare-%s", uuid.New().String()[:8])
	}

	query := `
		INSERT INTO monitoring_instance_shares (
			id, instance_id, user_id, permission, shared_by, created_at
		) VALUES (
			$1, $2, $3, $4, $5, CURRENT_TIMESTAMP
		)
		ON CONFLICT (instance_id, user_id) 
		DO UPDATE SET permission = EXCLUDED.permission
		RETURNING created_at
	`

	return pool.QueryRow(ctx, query,
		share.ID, share.InstanceID, share.UserID, share.Permission, share.SharedBy,
	).Scan(&share.CreatedAt)
}

func (r *MonitoringInstanceRepository) DeleteShare(ctx context.Context, instanceID string, targetUserID int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM monitoring_instance_shares WHERE instance_id = $1 AND user_id = $2`
	_, err = pool.Exec(ctx, query, instanceID, targetUserID)
	return err
}

func (r *MonitoringInstanceRepository) GetDistinctGroups(ctx context.Context, userID int, userRole string) ([]string, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)
	var query string
	var args []interface{}

	if isAdmin {
		query = `SELECT DISTINCT group_name FROM monitoring_instances WHERE group_name IS NOT NULL AND group_name != '' ORDER BY group_name ASC`
	} else {
		query = `
			SELECT DISTINCT m.group_name 
			FROM monitoring_instances m
			LEFT JOIN monitoring_instance_shares mis ON m.id = mis.instance_id AND mis.user_id = $1
			WHERE (m.user_id = $1 OR m.visibility = 'public' OR mis.user_id = $1)
			  AND m.group_name IS NOT NULL AND m.group_name != ''
			ORDER BY m.group_name ASC
		`
		args = append(args, userID)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err == nil && g != "" {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func (r *MonitoringInstanceRepository) UpsertFromRemoteHost(ctx context.Context, host *domain.RemoteHostConfig, targetUserID int) (*domain.MonitoringInstance, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var existingID string
	err = pool.QueryRow(ctx, "SELECT id FROM monitoring_instances WHERE remote_host_id = $1", host.ID).Scan(&existingID)

	inst := &domain.MonitoringInstance{
		Name:             host.Name,
		Host:             host.Host,
		IPAddress:        host.Host,
		Port:             8889,
		InstanceType:     "server",
		GroupName:        host.GroupName,
		Tags:             host.Tags,
		PrometheusTarget: fmt.Sprintf("%s:8889", host.Host),
		RemoteHostID:     &host.ID,
		MetricSource:     "ssh",
		UserID:           &targetUserID,
		Visibility:       "private",
		AlertEnabled:     true,
		Notes:            fmt.Sprintf("Synchronized from Remote Host: %s", host.Name),
		UpdatedAt:        time.Now(),
	}

	if err == nil && existingID != "" {
		inst.ID = existingID
		updateQuery := `
			UPDATE monitoring_instances SET
				name = $1, host = $2, ip_address = $3, group_name = $4, tags = $5,
				prometheus_target = $6, metric_source = COALESCE(NULLIF(metric_source, ''), 'ssh'),
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $7
			RETURNING created_at, updated_at
		`
		err = pool.QueryRow(ctx, updateQuery,
			inst.Name, inst.Host, inst.IPAddress, inst.GroupName, inst.Tags,
			inst.PrometheusTarget, inst.ID,
		).Scan(&inst.CreatedAt, &inst.UpdatedAt)
		if err != nil {
			return nil, err
		}
		return inst, nil
	}

	// Create new
	inst.ID = fmt.Sprintf("inst-%s", uuid.New().String()[:8])
	inst.CreatedAt = time.Now()
	if inst.GroupName == "" {
		inst.GroupName = "Default"
	}
	if inst.Tags == nil {
		inst.Tags = []string{}
	}

	insertQuery := `
		INSERT INTO monitoring_instances (
			id, name, host, ip_address, port, instance_type, group_name, tags,
			prometheus_target, remote_host_id, metric_source, user_id, visibility, alert_enabled,
			notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		RETURNING created_at, updated_at
	`
	err = pool.QueryRow(ctx, insertQuery,
		inst.ID, inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.MetricSource, inst.UserID, inst.Visibility, inst.AlertEnabled, inst.Notes,
	).Scan(&inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return inst, nil
}

// SaveLiveMetrics updates the last known live telemetry snapshot and timestamp for an instance
func (r *MonitoringInstanceRepository) SaveLiveMetrics(ctx context.Context, id string, metrics *domain.InstanceLiveMetrics) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	metricBytes, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	query := `UPDATE monitoring_instances SET last_metrics = $1, last_metrics_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err = pool.Exec(ctx, query, metricBytes, id)
	return err
}

// SaveMetricsHistory appends a single metric record for trend graphs
func (r *MonitoringInstanceRepository) SaveMetricsHistory(ctx context.Context, instanceID string, cpuPct, memPct, diskPct, netMB float64, source string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	if source == "" {
		source = "ssh"
	}
	query := `INSERT INTO instance_metrics_history (instance_id, cpu_pct, mem_pct, disk_pct, net_total_mb, source, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)`
	_, err = pool.Exec(ctx, query, instanceID, cpuPct, memPct, diskPct, netMB, source)
	return err
}

// GetMetricsHistoryFromDB returns stored history points from PostgreSQL when Prometheus is unavailable
func (r *MonitoringInstanceRepository) GetMetricsHistoryFromDB(ctx context.Context, instanceID string, startTime time.Time) (*domain.InstanceHistoryResponse, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT 
			COALESCE(cpu_pct, 0),
			COALESCE(mem_pct, 0),
			COALESCE(disk_pct, 0),
			COALESCE(net_total_mb, 0),
			EXTRACT(EPOCH FROM created_at)::BIGINT
		FROM instance_metrics_history
		WHERE instance_id = $1 AND created_at >= $2
		ORDER BY created_at ASC
	`
	rows, err := pool.Query(ctx, query, instanceID, startTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &domain.InstanceHistoryResponse{
		InstanceID: instanceID,
		CPU:        []domain.MetricHistoryPoint{},
		Memory:     []domain.MetricHistoryPoint{},
		Disk:       []domain.MetricHistoryPoint{},
		NetIn:      []domain.MetricHistoryPoint{},
		NetOut:     []domain.MetricHistoryPoint{},
	}

	for rows.Next() {
		var cpu, mem, disk, netVal float64
		var ts int64
		if err := rows.Scan(&cpu, &mem, &disk, &netVal, &ts); err == nil {
			resp.CPU = append(resp.CPU, domain.MetricHistoryPoint{Timestamp: ts, Value: math.Round(cpu*100) / 100})
			resp.Memory = append(resp.Memory, domain.MetricHistoryPoint{Timestamp: ts, Value: math.Round(mem*100) / 100})
			resp.Disk = append(resp.Disk, domain.MetricHistoryPoint{Timestamp: ts, Value: math.Round(disk*100) / 100})
			resp.NetIn = append(resp.NetIn, domain.MetricHistoryPoint{Timestamp: ts, Value: math.Round(netVal*100) / 100})
		}
	}
	return resp, nil
}

// PruneMetricsHistory deletes history points older than maxDays (default 7 days) to prevent database bloat
func (r *MonitoringInstanceRepository) PruneMetricsHistory(ctx context.Context, maxDays int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	if maxDays <= 0 {
		maxDays = 7
	}
	query := fmt.Sprintf("DELETE FROM instance_metrics_history WHERE created_at < NOW() - INTERVAL '%d days'", maxDays)
	_, err = pool.Exec(ctx, query)
	return err
}
