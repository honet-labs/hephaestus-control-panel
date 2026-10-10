package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ServerInventoryRepository struct{}

func NewServerInventoryRepository() *ServerInventoryRepository {
	return &ServerInventoryRepository{}
}

func (r *ServerInventoryRepository) List(ctx context.Context, userID int, userRole string, search string, status string, osType string) ([]domain.ServerInventoryItem, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(search) != "" {
		searchTerm := "%" + strings.TrimSpace(strings.ToLower(search)) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(si.server_name) LIKE $%d OR LOWER(si.ip_address) LIKE $%d OR LOWER(si.processor_model) LIKE $%d OR LOWER(si.gpu_model) LIKE $%d OR LOWER(si.notes) LIKE $%d)", argIdx, argIdx, argIdx, argIdx, argIdx))
		args = append(args, searchTerm)
		argIdx++
	}

	if strings.TrimSpace(status) != "" && status != "all" {
		conditions = append(conditions, fmt.Sprintf("LOWER(si.status) = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(status)))
		argIdx++
	}

	if strings.TrimSpace(osType) != "" && osType != "all" {
		conditions = append(conditions, fmt.Sprintf("LOWER(si.os_type) = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(osType)))
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT 
			si.id, si.remote_host_id, rh.name AS remote_host_name,
			si.server_name, si.ip_address, si.os_version, si.os_type, si.architecture_type,
			si.processor_model, si.total_core, si.total_memory, si.total_dimm_memory,
			si.total_storage_size, si.total_disk_count, si.total_network_interfaces,
			si.gpu_model, si.gpu_type, si.total_vram, si.status, si.notes,
			si.user_id, u.username AS owner_username,
			si.last_synced_at, si.created_at, si.updated_at
		FROM server_inventory si
		LEFT JOIN remote_host_configs rh ON si.remote_host_id = rh.id
		LEFT JOIN users u ON si.user_id = u.id
		%s
		ORDER BY si.server_name ASC, si.ip_address ASC
	`, whereClause)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query server inventory: %w", err)
	}
	defer rows.Close()

	var list []domain.ServerInventoryItem
	for rows.Next() {
		var item domain.ServerInventoryItem
		if err := rows.Scan(
			&item.ID, &item.RemoteHostID, &item.RemoteHostName,
			&item.ServerName, &item.IPAddress, &item.OSVersion, &item.OSType, &item.ArchitectureType,
			&item.ProcessorModel, &item.TotalCore, &item.TotalMemory, &item.TotalDimmMemory,
			&item.TotalStorageSize, &item.TotalDiskCount, &item.TotalNetworkInterfaces,
			&item.GPUModel, &item.GPUType, &item.TotalVRAM, &item.Status, &item.Notes,
			&item.UserID, &item.OwnerUsername,
			&item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan server inventory item: %w", err)
		}
		list = append(list, item)
	}

	return list, nil
}

func (r *ServerInventoryRepository) GetByID(ctx context.Context, id string) (*domain.ServerInventoryItem, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			si.id, si.remote_host_id, rh.name AS remote_host_name,
			si.server_name, si.ip_address, si.os_version, si.os_type, si.architecture_type,
			si.processor_model, si.total_core, si.total_memory, si.total_dimm_memory,
			si.total_storage_size, si.total_disk_count, si.total_network_interfaces,
			si.gpu_model, si.gpu_type, si.total_vram, si.status, si.notes,
			si.user_id, u.username AS owner_username,
			si.last_synced_at, si.created_at, si.updated_at
		FROM server_inventory si
		LEFT JOIN remote_host_configs rh ON si.remote_host_id = rh.id
		LEFT JOIN users u ON si.user_id = u.id
		WHERE si.id = $1
	`

	var item domain.ServerInventoryItem
	err = pool.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.RemoteHostID, &item.RemoteHostName,
		&item.ServerName, &item.IPAddress, &item.OSVersion, &item.OSType, &item.ArchitectureType,
		&item.ProcessorModel, &item.TotalCore, &item.TotalMemory, &item.TotalDimmMemory,
		&item.TotalStorageSize, &item.TotalDiskCount, &item.TotalNetworkInterfaces,
		&item.GPUModel, &item.GPUType, &item.TotalVRAM, &item.Status, &item.Notes,
		&item.UserID, &item.OwnerUsername,
		&item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *ServerInventoryRepository) GetByRemoteHostID(ctx context.Context, remoteHostID string) (*domain.ServerInventoryItem, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			si.id, si.remote_host_id, rh.name AS remote_host_name,
			si.server_name, si.ip_address, si.os_version, si.os_type, si.architecture_type,
			si.processor_model, si.total_core, si.total_memory, si.total_dimm_memory,
			si.total_storage_size, si.total_disk_count, si.total_network_interfaces,
			si.gpu_model, si.gpu_type, si.total_vram, si.status, si.notes,
			si.user_id, u.username AS owner_username,
			si.last_synced_at, si.created_at, si.updated_at
		FROM server_inventory si
		LEFT JOIN remote_host_configs rh ON si.remote_host_id = rh.id
		LEFT JOIN users u ON si.user_id = u.id
		WHERE si.remote_host_id = $1
		LIMIT 1
	`

	var item domain.ServerInventoryItem
	err = pool.QueryRow(ctx, query, remoteHostID).Scan(
		&item.ID, &item.RemoteHostID, &item.RemoteHostName,
		&item.ServerName, &item.IPAddress, &item.OSVersion, &item.OSType, &item.ArchitectureType,
		&item.ProcessorModel, &item.TotalCore, &item.TotalMemory, &item.TotalDimmMemory,
		&item.TotalStorageSize, &item.TotalDiskCount, &item.TotalNetworkInterfaces,
		&item.GPUModel, &item.GPUType, &item.TotalVRAM, &item.Status, &item.Notes,
		&item.UserID, &item.OwnerUsername,
		&item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *ServerInventoryRepository) Create(ctx context.Context, item *domain.ServerInventoryItem) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if item.ID == "" {
		item.ID = "srv-" + uuid.New().String()[:8]
	}
	if item.UserID != nil && *item.UserID <= 0 {
		item.UserID = nil
	}
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	query := `
		INSERT INTO server_inventory (
			id, remote_host_id, server_name, ip_address, os_version, os_type, architecture_type,
			processor_model, total_core, total_memory, total_dimm_memory,
			total_storage_size, total_disk_count, total_network_interfaces,
			gpu_model, gpu_type, total_vram, status, notes, user_id, last_synced_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23
		)
	`

	_, err = pool.Exec(ctx, query,
		item.ID, item.RemoteHostID, item.ServerName, item.IPAddress, item.OSVersion, item.OSType, item.ArchitectureType,
		item.ProcessorModel, item.TotalCore, item.TotalMemory, item.TotalDimmMemory,
		item.TotalStorageSize, item.TotalDiskCount, item.TotalNetworkInterfaces,
		item.GPUModel, item.GPUType, item.TotalVRAM, item.Status, item.Notes, item.UserID, item.LastSyncedAt, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (r *ServerInventoryRepository) Update(ctx context.Context, item *domain.ServerInventoryItem) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	item.UpdatedAt = time.Now()

	query := `
		UPDATE server_inventory SET
			remote_host_id = $2,
			server_name = $3,
			ip_address = $4,
			os_version = $5,
			os_type = $6,
			architecture_type = $7,
			processor_model = $8,
			total_core = $9,
			total_memory = $10,
			total_dimm_memory = $11,
			total_storage_size = $12,
			total_disk_count = $13,
			total_network_interfaces = $14,
			gpu_model = $15,
			gpu_type = $16,
			total_vram = $17,
			status = $18,
			notes = $19,
			last_synced_at = COALESCE($20, last_synced_at),
			updated_at = $21
		WHERE id = $1
	`

	_, err = pool.Exec(ctx, query,
		item.ID, item.RemoteHostID, item.ServerName, item.IPAddress, item.OSVersion, item.OSType, item.ArchitectureType,
		item.ProcessorModel, item.TotalCore, item.TotalMemory, item.TotalDimmMemory,
		item.TotalStorageSize, item.TotalDiskCount, item.TotalNetworkInterfaces,
		item.GPUModel, item.GPUType, item.TotalVRAM, item.Status, item.Notes, item.LastSyncedAt, item.UpdatedAt,
	)
	return err
}

func (r *ServerInventoryRepository) UpsertFromProbe(ctx context.Context, item *domain.ServerInventoryItem) (*domain.ServerInventoryItem, bool, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, false, err
	}

	if item.UserID != nil && *item.UserID <= 0 {
		item.UserID = nil
	}

	// 1. Try finding existing by remote_host_id or ip_address
	var existingID string
	var queryFind string
	if item.RemoteHostID != nil && *item.RemoteHostID != "" {
		queryFind = `SELECT id FROM server_inventory WHERE remote_host_id = $1 LIMIT 1`
		_ = pool.QueryRow(ctx, queryFind, *item.RemoteHostID).Scan(&existingID)
	}

	if existingID == "" && item.IPAddress != "" && item.IPAddress != "N/A" {
		queryFind = `SELECT id FROM server_inventory WHERE ip_address = $1 LIMIT 1`
		_ = pool.QueryRow(ctx, queryFind, item.IPAddress).Scan(&existingID)
	}

	if existingID != "" {
		item.ID = existingID
		if err := r.Update(ctx, item); err != nil {
			return nil, false, err
		}
		updated, _ := r.GetByID(ctx, existingID)
		return updated, false, nil // false = updated
	}

	// 2. Insert new
	if err := r.Create(ctx, item); err != nil {
		return nil, false, err
	}
	created, _ := r.GetByID(ctx, item.ID)
	return created, true, nil // true = created
}

func (r *ServerInventoryRepository) Delete(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, "DELETE FROM server_inventory WHERE id = $1", id)
	return err
}

func (r *ServerInventoryRepository) GetStats(ctx context.Context) (*domain.ServerInventoryStats, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			COUNT(*) AS total_servers,
			COUNT(CASE WHEN LOWER(status) = 'active' THEN 1 END) AS active_servers,
			COUNT(CASE WHEN remote_host_id IS NOT NULL AND remote_host_id != '' THEN 1 END) AS synced_from_remote,
			COUNT(CASE WHEN remote_host_id IS NULL OR remote_host_id = '' THEN 1 END) AS manual_servers,
			COUNT(CASE WHEN gpu_model IS NOT NULL AND gpu_model != '' AND gpu_model != 'N/A' THEN 1 END) AS with_gpu_count
		FROM server_inventory
	`

	var stats domain.ServerInventoryStats
	err = pool.QueryRow(ctx, query).Scan(
		&stats.TotalServers,
		&stats.ActiveServers,
		&stats.SyncedFromRemote,
		&stats.ManualServers,
		&stats.WithGPUCount,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
