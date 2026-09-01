package repository

import (
	"context"
	"encoding/json"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type TopologyRepository struct{}

func NewTopologyRepository() *TopologyRepository {
	return &TopologyRepository{}
}

// Sheets
func (r *TopologyRepository) ListSheets(ctx context.Context) ([]domain.TopologySheet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, name, sort_order, created_at, updated_at FROM topology_sheets ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sheets []domain.TopologySheet
	for rows.Next() {
		var s domain.TopologySheet
		if err := rows.Scan(&s.ID, &s.Name, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sheets = append(sheets, s)
	}
	return sheets, nil
}

func (r *TopologyRepository) CreateSheet(ctx context.Context, name string, sortOrder int) (*domain.TopologySheet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	var s domain.TopologySheet
	s.Name = name
	s.SortOrder = sortOrder
	err = pool.QueryRow(ctx, `INSERT INTO topology_sheets (name, sort_order) VALUES ($1, $2) RETURNING id, created_at, updated_at`, name, sortOrder).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
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

func (r *TopologyRepository) DeleteSheet(ctx context.Context, id int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM topology_sheets WHERE id = $1`, id)
	return err
}

// Devices
func (r *TopologyRepository) ListDevices(ctx context.Context, sheetID *int) ([]domain.TopologyDevice, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, ip_address, device_type, status, sources, labels, interfaces, sheet_id, x, y, created_at 
              FROM topology_devices WHERE ($1::int IS NULL OR sheet_id = $1) ORDER BY name ASC`
	rows, err := pool.Query(ctx, query, sheetID)
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
                interfaces = EXCLUDED.interfaces, sheet_id = EXCLUDED.sheet_id, x = EXCLUDED.x, y = EXCLUDED.y`
	_, err = pool.Exec(ctx, query, d.ID, d.Name, d.IPAddress, d.DeviceType, d.Status, d.Sources, labelsJSON, ifacesJSON, d.SheetID, d.X, d.Y)
	return err
}

func (r *TopologyRepository) UpdatePosition(ctx context.Context, id string, x, y float64) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE topology_devices SET x = $1, y = $2 WHERE id = $3`, x, y, id)
	return err
}

func (r *TopologyRepository) DeleteDevice(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM topology_devices WHERE id = $1`, id)
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

	query := `INSERT INTO topology_edges (source_id, target_id, label, source_label, target_label, edge_type, sheet_id)
              VALUES ($1, $2, $3, $4, $5, $6, $7)
              ON CONFLICT (source_id, target_id, sheet_id) DO UPDATE SET
                label = EXCLUDED.label, source_label = EXCLUDED.source_label,
                target_label = EXCLUDED.target_label, edge_type = EXCLUDED.edge_type`
	_, err = pool.Exec(ctx, query, e.SourceID, e.TargetID, e.Label, e.SourceLabel, e.TargetLabel, e.EdgeType, e.SheetID)
	return err
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
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `INSERT INTO device_ping_results (device_id, ip, reachable, latency_ms, checked_at)
              VALUES ($1, $2, $3, $4, $5)
              ON CONFLICT (device_id) DO UPDATE SET
                ip = EXCLUDED.ip, reachable = EXCLUDED.reachable,
                latency_ms = EXCLUDED.latency_ms, checked_at = EXCLUDED.checked_at`
	_, err = pool.Exec(ctx, query, res.DeviceID, res.IP, res.Reachable, res.LatencyMS, res.CheckedAt)
	if err != nil {
		return err
	}

	status := "offline"
	if res.Reachable {
		status = "online"
	}
	_, _ = pool.Exec(ctx, `UPDATE topology_devices SET status = $1 WHERE id = $2`, status, res.DeviceID)
	return nil
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
