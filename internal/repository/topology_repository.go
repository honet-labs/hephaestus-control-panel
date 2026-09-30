package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TopologyRepository struct{}

func NewTopologyRepository() *TopologyRepository {
	return &TopologyRepository{}
}

// Sheets
func (r *TopologyRepository) ListSheets(ctx context.Context, userID int, userRole string) ([]domain.TopologySheet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	isAdmin := domain.IsAdminRole(userRole)
	var rows pgx.Rows

	if isAdmin || (userID == 0 && userRole == "ADMIN") {
		query := `
			SELECT 
				s.id, s.name, s.sort_order, s.user_id, 
				COALESCE(u.username, 'Admin') AS owner_username,
				COALESCE(s.visibility, 'private') AS visibility,
				s.created_at, s.updated_at,
				(SELECT COUNT(*) FROM topology_sheet_shares WHERE sheet_id = s.id) AS shares_count
			FROM topology_sheets s
			LEFT JOIN users u ON s.user_id = u.id
			ORDER BY s.sort_order ASC, s.id ASC
		`
		rows, err = pool.Query(ctx, query)
	} else {
		query := `
			SELECT 
				s.id, s.name, s.sort_order, s.user_id, 
				COALESCE(u.username, 'Admin') AS owner_username,
				COALESCE(s.visibility, 'private') AS visibility,
				s.created_at, s.updated_at,
				(SELECT COUNT(*) FROM topology_sheet_shares WHERE sheet_id = s.id) AS shares_count,
				COALESCE(tss.permission, '') AS share_perm
			FROM topology_sheets s
			LEFT JOIN users u ON s.user_id = u.id
			LEFT JOIN topology_sheet_shares tss ON s.id = tss.sheet_id AND tss.user_id = $1
			WHERE s.visibility = 'public'
			   OR s.user_id = $1
			   OR tss.user_id = $1
			ORDER BY s.sort_order ASC, s.id ASC
		`
		rows, err = pool.Query(ctx, query, userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sheets []domain.TopologySheet
	for rows.Next() {
		var s domain.TopologySheet
		var sharePerm string
		if isAdmin || (userID == 0 && userRole == "ADMIN") {
			if err := rows.Scan(&s.ID, &s.Name, &s.SortOrder, &s.UserID, &s.OwnerUsername, &s.Visibility, &s.CreatedAt, &s.UpdatedAt, &s.SharesCount); err != nil {
				return nil, err
			}
			s.IsOwner = true
			s.UserPermission = "manage"
		} else {
			if err := rows.Scan(&s.ID, &s.Name, &s.SortOrder, &s.UserID, &s.OwnerUsername, &s.Visibility, &s.CreatedAt, &s.UpdatedAt, &s.SharesCount, &sharePerm); err != nil {
				return nil, err
			}
			s.IsOwner = (s.UserID != nil && *s.UserID == userID)
			if s.IsOwner {
				s.UserPermission = "manage"
			} else if sharePerm != "" {
				s.UserPermission = sharePerm
			} else if s.Visibility == "public" {
				s.UserPermission = "public"
			} else {
				s.UserPermission = "read"
			}
		}
		sheets = append(sheets, s)
	}
	if sheets == nil {
		sheets = []domain.TopologySheet{}
	}
	return sheets, nil
}

func (r *TopologyRepository) CreateSheet(ctx context.Context, name string, sortOrder int, userID *int, visibility string) (*domain.TopologySheet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	if visibility != "public" {
		visibility = "private"
	}
	var s domain.TopologySheet
	s.Name = name
	s.SortOrder = sortOrder
	s.UserID = userID
	s.Visibility = visibility
	s.IsOwner = true
	s.UserPermission = "manage"
	s.SharesCount = 0

	err = pool.QueryRow(ctx, `
		INSERT INTO topology_sheets (name, sort_order, user_id, visibility) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, created_at, updated_at
	`, name, sortOrder, userID, visibility).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TopologyRepository) UpdateSheet(ctx context.Context, id int, name string, sortOrder int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE topology_sheets SET name = $1, sort_order = $2, updated_at = NOW() WHERE id = $3`, name, sortOrder, id)
	return err
}

func (r *TopologyRepository) UpdateSheetVisibility(ctx context.Context, id int, visibility string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	if visibility != "public" {
		visibility = "private"
	}
	_, err = pool.Exec(ctx, `UPDATE topology_sheets SET visibility = $1, updated_at = NOW() WHERE id = $2`, visibility, id)
	return err
}

func (r *TopologyRepository) DeleteSheet(ctx context.Context, id int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM topology_sheets WHERE id = $1`, id)
	return err
}

func (r *TopologyRepository) CheckSheetAccess(ctx context.Context, sheetID int, userID int, userRole string) (hasAccess bool, isOwner bool, perm string, err error) {
	if domain.IsAdminRole(userRole) {
		return true, true, "manage", nil
	}

	pool, err := database.GetPool()
	if err != nil {
		return false, false, "", err
	}

	query := `
		SELECT 
			s.user_id,
			COALESCE(s.visibility, 'private'),
			(s.user_id = $2) AS is_owner,
			COALESCE(tss.permission, '') AS share_perm
		FROM topology_sheets s
		LEFT JOIN topology_sheet_shares tss ON s.id = tss.sheet_id AND tss.user_id = $2
		WHERE s.id = $1
	`
	var ownerID *int
	var visibility string
	var ownerBool bool
	var sharePerm string
	err = pool.QueryRow(ctx, query, sheetID, userID).Scan(&ownerID, &visibility, &ownerBool, &sharePerm)
	if err != nil {
		return false, false, "", err
	}

	if ownerBool {
		return true, true, "manage", nil
	}
	if sharePerm != "" {
		return true, false, sharePerm, nil
	}
	if visibility == "public" {
		return true, false, "read", nil
	}

	return false, false, "", nil
}

// Sheet Shares
func (r *TopologyRepository) ListSheetShares(ctx context.Context, sheetID int) ([]domain.TopologySheetShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT s.id, s.sheet_id, s.user_id, u.username, s.permission,
		       s.shared_by, COALESCE(sb.username, 'Admin') AS shared_by_username, s.created_at
		FROM topology_sheet_shares s
		JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE s.sheet_id = $1
		ORDER BY s.created_at DESC
	`
	rows, err := pool.Query(ctx, query, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []domain.TopologySheetShare
	for rows.Next() {
		var s domain.TopologySheetShare
		if err := rows.Scan(&s.ID, &s.SheetID, &s.UserID, &s.Username, &s.Permission, &s.SharedBy, &s.SharedByUsername, &s.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	if shares == nil {
		shares = []domain.TopologySheetShare{}
	}
	return shares, nil
}

func (r *TopologyRepository) AddSheetShare(ctx context.Context, sheetID int, targetUserID int, permission string, sharedBy int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if permission != "manage" {
		permission = "read"
	}

	shareID := fmt.Sprintf("tss-%s", uuid.New().String()[:8])
	query := `
		INSERT INTO topology_sheet_shares (id, sheet_id, user_id, permission, shared_by, created_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (sheet_id, user_id) DO UPDATE SET
			permission = EXCLUDED.permission,
			shared_by = EXCLUDED.shared_by,
			created_at = CURRENT_TIMESTAMP
	`
	_, err = pool.Exec(ctx, query, shareID, sheetID, targetUserID, permission, sharedBy)
	return err
}

func (r *TopologyRepository) DeleteSheetShare(ctx context.Context, sheetID int, targetUserID int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM topology_sheet_shares WHERE sheet_id = $1 AND user_id = $2`, sheetID, targetUserID)
	return err
}

func (r *TopologyRepository) ListAvailableUsers(ctx context.Context) ([]map[string]interface{}, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, username, role FROM users ORDER BY username ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int
		var username, role string
		if err := rows.Scan(&id, &username, &role); err != nil {
			continue
		}
		users = append(users, map[string]interface{}{
			"id":       id,
			"username": username,
			"role":     role,
		})
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	return users, nil
}

// Devices
func (r *TopologyRepository) ListDevices(ctx context.Context, sheetID *int) ([]domain.TopologyDevice, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if sheetID != nil {
		query := `
			SELECT d.id, d.name, d.ip_address, d.device_type, d.status, d.sources, d.labels, d.interfaces,
			       $1::int AS sheet_id,
			       COALESCE(sn.x, d.x, 220) AS x,
			       COALESCE(sn.y, d.y, 130) AS y,
			       d.created_at
			FROM topology_devices d
			LEFT JOIN topology_sheet_nodes sn ON sn.device_id = d.id AND sn.sheet_id = $1
			WHERE (
				sn.sheet_id = $1
				OR d.sheet_id = $1
				OR d.id IN (
					SELECT source_id FROM topology_edges WHERE sheet_id = $1
					UNION
					SELECT target_id FROM topology_edges WHERE sheet_id = $1
				)
			)
			ORDER BY d.name ASC
		`
		rows, err = pool.Query(ctx, query, *sheetID)
	} else {
		query := `
			SELECT id, name, ip_address, device_type, status, sources, labels, interfaces, sheet_id, x, y, created_at
			FROM topology_devices
			ORDER BY name ASC
		`
		rows, err = pool.Query(ctx, query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.TopologyDevice
	for rows.Next() {
		var d domain.TopologyDevice
		var labelsRaw, ifacesRaw []byte
		if err := rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.DeviceType, &d.Status, &d.Sources, &labelsRaw, &ifacesRaw, &d.SheetID, &d.X, &d.Y, &d.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(labelsRaw, &d.Labels)
		_ = json.Unmarshal(ifacesRaw, &d.Interfaces)
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []domain.TopologyDevice{}
	}
	return devices, nil
}

func (r *TopologyRepository) SaveDevice(ctx context.Context, d domain.TopologyDevice) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	labelsJSON, _ := json.Marshal(d.Labels)
	if len(labelsJSON) == 0 {
		labelsJSON = []byte("{}")
	}
	ifacesJSON, _ := json.Marshal(d.Interfaces)
	if len(ifacesJSON) == 0 {
		ifacesJSON = []byte("[]")
	}

	query := `INSERT INTO topology_devices (id, name, ip_address, device_type, status, sources, labels, interfaces, sheet_id, x, y)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, ip_address = EXCLUDED.ip_address, device_type = EXCLUDED.device_type,
                status = EXCLUDED.status, sources = EXCLUDED.sources, labels = EXCLUDED.labels,
                interfaces = EXCLUDED.interfaces,
                sheet_id = COALESCE(topology_devices.sheet_id, EXCLUDED.sheet_id),
                x = COALESCE(EXCLUDED.x, topology_devices.x),
                y = COALESCE(EXCLUDED.y, topology_devices.y)`
	_, err = pool.Exec(ctx, query, d.ID, d.Name, d.IPAddress, d.DeviceType, d.Status, d.Sources, labelsJSON, ifacesJSON, d.SheetID, d.X, d.Y)
	if err != nil {
		return err
	}

	// Persist per-sheet coordinate if sheetID and coordinates are provided
	if d.SheetID != nil && *d.SheetID > 0 && d.X != nil && d.Y != nil {
		snQuery := `
			INSERT INTO topology_sheet_nodes (sheet_id, device_id, x, y, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (sheet_id, device_id) DO UPDATE SET
				x = EXCLUDED.x,
				y = EXCLUDED.y,
				updated_at = NOW()
		`
		_, _ = pool.Exec(ctx, snQuery, *d.SheetID, d.ID, *d.X, *d.Y)
	}

	return nil
}

func (r *TopologyRepository) UpdatePosition(ctx context.Context, id string, x, y float64, sheetID *int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	// Always keep global default position updated
	_, _ = pool.Exec(ctx, `UPDATE topology_devices SET x = $1, y = $2 WHERE id = $3`, x, y, id)

	// If sheet is specified, persist sheet-scoped coordinate
	if sheetID != nil && *sheetID > 0 {
		query := `
			INSERT INTO topology_sheet_nodes (sheet_id, device_id, x, y, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (sheet_id, device_id) DO UPDATE SET
				x = EXCLUDED.x,
				y = EXCLUDED.y,
				updated_at = NOW()
		`
		_, err = pool.Exec(ctx, query, *sheetID, id, x, y)
		return err
	}
	return nil
}

func (r *TopologyRepository) DeleteDevice(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `DELETE FROM topology_sheet_nodes WHERE device_id = $1`, id)
	_, _ = pool.Exec(ctx, `DELETE FROM topology_edges WHERE source_id = $1 OR target_id = $1`, id)
	_, err = pool.Exec(ctx, `DELETE FROM topology_devices WHERE id = $1`, id)
	return err
}

func (r *TopologyRepository) RemoveDeviceFromCanvas(ctx context.Context, id string, sheetID *int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if sheetID != nil && *sheetID > 0 {
		// 1. Delete edges attached to this device on this sheet
		_, _ = pool.Exec(ctx, `DELETE FROM topology_edges WHERE (source_id = $1 OR target_id = $1) AND (sheet_id = $2 OR sheet_id IS NULL)`, id, *sheetID)
		// 2. Remove node from sheet_nodes table
		_, err = pool.Exec(ctx, `DELETE FROM topology_sheet_nodes WHERE sheet_id = $1 AND device_id = $2`, *sheetID, id)
		// 3. Clear sheet_id from topology_devices if it was pinned to this sheet
		_, _ = pool.Exec(ctx, `UPDATE topology_devices SET sheet_id = NULL WHERE id = $1 AND sheet_id = $2`, id, *sheetID)
		return err
	}

	// If no sheetID, remove globally from canvas
	_, _ = pool.Exec(ctx, `DELETE FROM topology_edges WHERE source_id = $1 OR target_id = $1`, id)
	_, _ = pool.Exec(ctx, `DELETE FROM topology_sheet_nodes WHERE device_id = $1`, id)
	_, err = pool.Exec(ctx, `UPDATE topology_devices SET sheet_id = NULL, x = NULL, y = NULL WHERE id = $1`, id)
	return err
}

// Edges
func (r *TopologyRepository) ListEdges(ctx context.Context, sheetID *int) ([]domain.TopologyEdge, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, source_id, target_id, label, source_label, target_label, edge_type, sheet_id, created_at 
              FROM topology_edges WHERE ($1::int IS NULL OR sheet_id = $1)`
	rows, err := pool.Query(ctx, query, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []domain.TopologyEdge
	for rows.Next() {
		var e domain.TopologyEdge
		if err := rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Label, &e.SourceLabel, &e.TargetLabel, &e.EdgeType, &e.SheetID, &e.CreatedAt); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, nil
}

func (r *TopologyRepository) SaveEdge(ctx context.Context, e domain.TopologyEdge) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if e.ID > 0 {
		query := `UPDATE topology_edges 
                  SET source_id = $1, target_id = $2, label = $3, source_label = $4, target_label = $5, edge_type = $6 
                  WHERE id = $7`
		_, err = pool.Exec(ctx, query, e.SourceID, e.TargetID, e.Label, e.SourceLabel, e.TargetLabel, e.EdgeType, e.ID)
		return err
	}

	query := `INSERT INTO topology_edges (source_id, target_id, label, source_label, target_label, edge_type, sheet_id)
              VALUES ($1, $2, $3, $4, $5, $6, $7)
              ON CONFLICT (source_id, target_id, sheet_id) DO UPDATE SET
                label = EXCLUDED.label, source_label = EXCLUDED.source_label,
                target_label = EXCLUDED.target_label, edge_type = EXCLUDED.edge_type`
	_, err = pool.Exec(ctx, query, e.SourceID, e.TargetID, e.Label, e.SourceLabel, e.TargetLabel, e.EdgeType, e.SheetID)
	if err != nil {
		return err
	}

	if e.SheetID != nil && *e.SheetID > 0 {
		// Ensure connected devices have an entry in topology_sheet_nodes so their position is pinned to this sheet
		_, _ = pool.Exec(ctx, `
			INSERT INTO topology_sheet_nodes (sheet_id, device_id, x, y)
			SELECT $1, d.id, COALESCE(d.x, 220), COALESCE(d.y, 130)
			FROM topology_devices d
			WHERE d.id IN ($2, $3)
			ON CONFLICT (sheet_id, device_id) DO NOTHING
		`, *e.SheetID, e.SourceID, e.TargetID)
	}

	return nil
}

func (r *TopologyRepository) DeleteEdge(ctx context.Context, id int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM topology_edges WHERE id = $1`, id)
	return err
}

// Device Ping Results
func (r *TopologyRepository) SavePingResult(ctx context.Context, res domain.DevicePingResult) error {
	return r.SavePingResultsBatch(ctx, []domain.DevicePingResult{res})
}

// SavePingResultsBatch executes batch insertion and status updates inside a single database transaction
func (r *TopologyRepository) SavePingResultsBatch(ctx context.Context, results []domain.DevicePingResult) error {
	if len(results) == 0 {
		return nil
	}

	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	insertQuery := `INSERT INTO device_ping_results (device_id, ip, reachable, latency_ms, checked_at)
              VALUES ($1, $2, $3, $4, $5)
              ON CONFLICT (device_id) DO UPDATE SET
                ip = EXCLUDED.ip, reachable = EXCLUDED.reachable,
                latency_ms = EXCLUDED.latency_ms, checked_at = EXCLUDED.checked_at`

	updateStatusQuery := `UPDATE topology_devices SET status = $1 WHERE id = $2`

	for _, res := range results {
		if _, err := tx.Exec(ctx, insertQuery, res.DeviceID, res.IP, res.Reachable, res.LatencyMS, res.CheckedAt); err != nil {
			return err
		}

		status := "offline"
		if res.Reachable {
			status = "online"
		}
		if _, err := tx.Exec(ctx, updateStatusQuery, status, res.DeviceID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *TopologyRepository) ListPingResults(ctx context.Context) ([]domain.DevicePingResult, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT device_id, ip, reachable, latency_ms, checked_at FROM device_ping_results ORDER BY checked_at DESC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.DevicePingResult
	for rows.Next() {
		var res domain.DevicePingResult
		if err := rows.Scan(&res.DeviceID, &res.IP, &res.Reachable, &res.LatencyMS, &res.CheckedAt); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}
