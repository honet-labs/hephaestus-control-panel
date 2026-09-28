package repository

import (
	"context"
	"errors"
	"fmt"
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
		if err := rows.Scan(
			&inst.ID, &inst.Name, &inst.Host, &inst.IPAddress, &inst.Port, &inst.InstanceType,
			&inst.GroupName, &tags, &inst.PrometheusTarget, &inst.RemoteHostID,
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
	err = pool.QueryRow(ctx, query, userID, id).Scan(
		&inst.ID, &inst.Name, &inst.Host, &inst.IPAddress, &inst.Port, &inst.InstanceType,
		&inst.GroupName, &tags, &inst.PrometheusTarget, &inst.RemoteHostID,
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
		inst.Visibility = "public"
	}
	if inst.Port == 0 {
		inst.Port = 8889
	}
	if inst.Tags == nil {
		inst.Tags = []string{}
	}

	query := `
		INSERT INTO monitoring_instances (
			id, name, host, ip_address, port, instance_type, group_name, tags,
			prometheus_target, remote_host_id, user_id, visibility, alert_enabled,
			notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		RETURNING created_at, updated_at
	`

	return pool.QueryRow(ctx, query,
		inst.ID, inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.UserID, inst.Visibility, inst.AlertEnabled, inst.Notes,
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
			visibility = $10,
			alert_enabled = $11,
			notes = $12,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $13
	`

	cmdTag, err := pool.Exec(ctx, query,
		inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.Visibility, inst.AlertEnabled, inst.Notes, inst.ID,
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

	if ownerID != nil && *ownerID == userID {
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
		UserID:           &targetUserID,
		Visibility:       "public",
		AlertEnabled:     true,
		Notes:            fmt.Sprintf("Synchronized from Remote Host: %s", host.Name),
		UpdatedAt:        time.Now(),
	}

	if err == nil && existingID != "" {
		inst.ID = existingID
		updateQuery := `
			UPDATE monitoring_instances SET
				name = $1, host = $2, ip_address = $3, group_name = $4, tags = $5,
				prometheus_target = $6, updated_at = CURRENT_TIMESTAMP
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
			prometheus_target, remote_host_id, user_id, visibility, alert_enabled,
			notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		RETURNING created_at, updated_at
	`
	err = pool.QueryRow(ctx, insertQuery,
		inst.ID, inst.Name, inst.Host, inst.IPAddress, inst.Port, inst.InstanceType,
		inst.GroupName, inst.Tags, inst.PrometheusTarget, inst.RemoteHostID,
		inst.UserID, inst.Visibility, inst.AlertEnabled, inst.Notes,
	).Scan(&inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return inst, nil
}
