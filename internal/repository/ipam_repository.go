package repository

import (
	"context"
	"fmt"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type IpamRepository struct{}

func NewIpamRepository() *IpamRepository {
	return &IpamRepository{}
}

// ListSubnets returns all configured subnets
func (r *IpamRepository) ListSubnets(ctx context.Context) ([]domain.IpamSubnet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, cidr, gateway, vlan_id, vrf, description, scan_interval,
		       last_scanned_at, next_scan_at, total_ips, total_used_ips, total_unused_ips,
		       user_id, created_at, updated_at
		FROM ipam_subnets
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subnets []domain.IpamSubnet
	for rows.Next() {
		var s domain.IpamSubnet
		err := rows.Scan(
			&s.ID, &s.Name, &s.CIDR, &s.Gateway, &s.VlanID, &s.VRF, &s.Description, &s.ScanInterval,
			&s.LastScannedAt, &s.NextScanAt, &s.TotalIPs, &s.TotalUsedIPs, &s.TotalUnusedIPs,
			&s.UserID, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, s)
	}

	return subnets, nil
}

// GetSubnetByID returns a single subnet by ID
func (r *IpamRepository) GetSubnetByID(ctx context.Context, id string) (*domain.IpamSubnet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, cidr, gateway, vlan_id, vrf, description, scan_interval,
		       last_scanned_at, next_scan_at, total_ips, total_used_ips, total_unused_ips,
		       user_id, created_at, updated_at
		FROM ipam_subnets
		WHERE id = $1
	`
	var s domain.IpamSubnet
	err = pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.CIDR, &s.Gateway, &s.VlanID, &s.VRF, &s.Description, &s.ScanInterval,
		&s.LastScannedAt, &s.NextScanAt, &s.TotalIPs, &s.TotalUsedIPs, &s.TotalUnusedIPs,
		&s.UserID, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// GetSubnetByCIDR returns a subnet matching CIDR
func (r *IpamRepository) GetSubnetByCIDR(ctx context.Context, cidr string) (*domain.IpamSubnet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, cidr, gateway, vlan_id, vrf, description, scan_interval,
		       last_scanned_at, next_scan_at, total_ips, total_used_ips, total_unused_ips,
		       user_id, created_at, updated_at
		FROM ipam_subnets
		WHERE cidr = $1
	`
	var s domain.IpamSubnet
	err = pool.QueryRow(ctx, query, cidr).Scan(
		&s.ID, &s.Name, &s.CIDR, &s.Gateway, &s.VlanID, &s.VRF, &s.Description, &s.ScanInterval,
		&s.LastScannedAt, &s.NextScanAt, &s.TotalIPs, &s.TotalUsedIPs, &s.TotalUnusedIPs,
		&s.UserID, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// CreateSubnet inserts a new subnet
func (r *IpamRepository) CreateSubnet(ctx context.Context, s *domain.IpamSubnet) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ipam_subnets (
			id, name, cidr, gateway, vlan_id, vrf, description, scan_interval,
			last_scanned_at, next_scan_at, total_ips, total_used_ips, total_unused_ips,
			user_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW()
		)
	`
	_, err = pool.Exec(ctx, query,
		s.ID, s.Name, s.CIDR, s.Gateway, s.VlanID, s.VRF, s.Description, s.ScanInterval,
		s.LastScannedAt, s.NextScanAt, s.TotalIPs, s.TotalUsedIPs, s.TotalUnusedIPs,
		s.UserID,
	)
	return err
}

// UpdateSubnet updates subnet configuration
func (r *IpamRepository) UpdateSubnet(ctx context.Context, s *domain.IpamSubnet) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		UPDATE ipam_subnets SET
			name = $2, gateway = $3, vlan_id = $4, vrf = $5, description = $6,
			scan_interval = $7, next_scan_at = $8, updated_at = NOW()
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query,
		s.ID, s.Name, s.Gateway, s.VlanID, s.VRF, s.Description, s.ScanInterval, s.NextScanAt,
	)
	return err
}

// UpdateSubnetScanStats updates post-scan totals and timestamps
func (r *IpamRepository) UpdateSubnetScanStats(ctx context.Context, id string, totalIPs, usedIPs, unusedIPs int, lastScannedAt time.Time, nextScanAt *time.Time) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		UPDATE ipam_subnets SET
			total_ips = $2, total_used_ips = $3, total_unused_ips = $4,
			last_scanned_at = $5, next_scan_at = $6, updated_at = NOW()
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query, id, totalIPs, usedIPs, unusedIPs, lastScannedAt, nextScanAt)
	return err
}

// DeleteSubnet removes a subnet and cascades to addresses & scan logs
func (r *IpamRepository) DeleteSubnet(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, "DELETE FROM ipam_subnets WHERE id = $1", id)
	return err
}

// GetSubnetsDueForScan returns subnets where scheduled scan is due
func (r *IpamRepository) GetSubnetsDueForScan(ctx context.Context) ([]domain.IpamSubnet, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, cidr, gateway, vlan_id, vrf, description, scan_interval,
		       last_scanned_at, next_scan_at, total_ips, total_used_ips, total_unused_ips,
		       user_id, created_at, updated_at
		FROM ipam_subnets
		WHERE scan_interval != 'manual' 
		  AND next_scan_at IS NOT NULL 
		  AND next_scan_at <= NOW()
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subnets []domain.IpamSubnet
	for rows.Next() {
		var s domain.IpamSubnet
		err := rows.Scan(
			&s.ID, &s.Name, &s.CIDR, &s.Gateway, &s.VlanID, &s.VRF, &s.Description, &s.ScanInterval,
			&s.LastScannedAt, &s.NextScanAt, &s.TotalIPs, &s.TotalUsedIPs, &s.TotalUnusedIPs,
			&s.UserID, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, s)
	}

	return subnets, nil
}

// ListAddressesBySubnet returns all recorded IP addresses for a subnet
func (r *IpamRepository) ListAddressesBySubnet(ctx context.Context, subnetID string) ([]domain.IpamAddress, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, subnet_id, ip_address, status, hostname, mac_address, device_type,
		       is_online, response_time_ms, last_seen_at, notes, created_at, updated_at
		FROM ipam_addresses
		WHERE subnet_id = $1
		ORDER BY inet(ip_address) ASC
	`
	rows, err := pool.Query(ctx, query, subnetID)
	if err != nil {
		// Fallback to text sort if inet conversion fails
		fallbackQuery := `
			SELECT id, subnet_id, ip_address, status, hostname, mac_address, device_type,
			       is_online, response_time_ms, last_seen_at, notes, created_at, updated_at
			FROM ipam_addresses
			WHERE subnet_id = $1
			ORDER BY ip_address ASC
		`
		rows, err = pool.Query(ctx, fallbackQuery, subnetID)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var addresses []domain.IpamAddress
	for rows.Next() {
		var a domain.IpamAddress
		err := rows.Scan(
			&a.ID, &a.SubnetID, &a.IPAddress, &a.Status, &a.Hostname, &a.MACAddress, &a.DeviceType,
			&a.IsOnline, &a.ResponseTimeMS, &a.LastSeenAt, &a.Notes, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}

	return addresses, nil
}

// GetAddress retrieves an IP record by ID
func (r *IpamRepository) GetAddress(ctx context.Context, id string) (*domain.IpamAddress, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, subnet_id, ip_address, status, hostname, mac_address, device_type,
		       is_online, response_time_ms, last_seen_at, notes, created_at, updated_at
		FROM ipam_addresses
		WHERE id = $1
	`
	var a domain.IpamAddress
	err = pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.SubnetID, &a.IPAddress, &a.Status, &a.Hostname, &a.MACAddress, &a.DeviceType,
		&a.IsOnline, &a.ResponseTimeMS, &a.LastSeenAt, &a.Notes, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// GetAddressByIP retrieves an IP record in a subnet by IP string
func (r *IpamRepository) GetAddressByIP(ctx context.Context, subnetID, ip string) (*domain.IpamAddress, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, subnet_id, ip_address, status, hostname, mac_address, device_type,
		       is_online, response_time_ms, last_seen_at, notes, created_at, updated_at
		FROM ipam_addresses
		WHERE subnet_id = $1 AND ip_address = $2
	`
	var a domain.IpamAddress
	err = pool.QueryRow(ctx, query, subnetID, ip).Scan(
		&a.ID, &a.SubnetID, &a.IPAddress, &a.Status, &a.Hostname, &a.MACAddress, &a.DeviceType,
		&a.IsOnline, &a.ResponseTimeMS, &a.LastSeenAt, &a.Notes, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// UpsertAddress creates or updates an IP record on conflict
func (r *IpamRepository) UpsertAddress(ctx context.Context, a *domain.IpamAddress) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ipam_addresses (
			id, subnet_id, ip_address, status, hostname, mac_address, device_type,
			is_online, response_time_ms, last_seen_at, notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW()
		)
		ON CONFLICT (subnet_id, ip_address) DO UPDATE SET
			status = CASE 
				WHEN EXCLUDED.status = 'discovered' AND ipam_addresses.status IN ('active', 'reserved') 
				THEN ipam_addresses.status 
				ELSE EXCLUDED.status 
			END,
			hostname = CASE 
				WHEN EXCLUDED.hostname != '' THEN EXCLUDED.hostname 
				ELSE ipam_addresses.hostname 
			END,
			mac_address = CASE 
				WHEN EXCLUDED.mac_address != '' THEN EXCLUDED.mac_address 
				ELSE ipam_addresses.mac_address 
			END,
			device_type = CASE 
				WHEN EXCLUDED.device_type != '' AND EXCLUDED.device_type != 'Unknown' THEN EXCLUDED.device_type 
				ELSE ipam_addresses.device_type 
			END,
			is_online = EXCLUDED.is_online,
			response_time_ms = EXCLUDED.response_time_ms,
			last_seen_at = COALESCE(EXCLUDED.last_seen_at, ipam_addresses.last_seen_at),
			notes = CASE 
				WHEN EXCLUDED.notes != '' THEN EXCLUDED.notes 
				ELSE ipam_addresses.notes 
			END,
			updated_at = NOW()
	`
	_, err = pool.Exec(ctx, query,
		a.ID, a.SubnetID, a.IPAddress, a.Status, a.Hostname, a.MACAddress, a.DeviceType,
		a.IsOnline, a.ResponseTimeMS, a.LastSeenAt, a.Notes,
	)
	return err
}

// UpdateAddress updates user editable fields of an IP address
func (r *IpamRepository) UpdateAddress(ctx context.Context, id string, req *domain.UpdateIpamAddressRequest) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		UPDATE ipam_addresses SET
			status = $2, hostname = $3, mac_address = $4, device_type = $5,
			notes = $6, updated_at = NOW()
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query, id, req.Status, req.Hostname, req.MACAddress, req.DeviceType, req.Notes)
	return err
}

// DeleteAddress releases an allocated IP address
func (r *IpamRepository) DeleteAddress(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, "DELETE FROM ipam_addresses WHERE id = $1", id)
	return err
}

// SaveScanLog saves a scan audit record
func (r *IpamRepository) SaveScanLog(ctx context.Context, log *domain.IpamScanLog) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ipam_scan_logs (
			id, subnet_id, started_at, finished_at, duration_ms, scanned_ips,
			found_active, status, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, err = pool.Exec(ctx, query,
		log.ID, log.SubnetID, log.StartedAt, log.FinishedAt, log.DurationMS, log.ScannedIPs,
		log.FoundActive, log.Status, log.ErrorMessage,
	)
	return err
}

// ListScanLogs returns recent scan logs for a subnet
func (r *IpamRepository) ListScanLogs(ctx context.Context, subnetID string, limit int) ([]domain.IpamScanLog, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, subnet_id, started_at, finished_at, duration_ms, scanned_ips,
		       found_active, status, error_message
		FROM ipam_scan_logs
		WHERE subnet_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`
	rows, err := pool.Query(ctx, query, subnetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.IpamScanLog
	for rows.Next() {
		var l domain.IpamScanLog
		err := rows.Scan(
			&l.ID, &l.SubnetID, &l.StartedAt, &l.FinishedAt, &l.DurationMS, &l.ScannedIPs,
			&l.FoundActive, &l.Status, &l.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return logs, nil
}

// GetSummaryStats aggregates top-level IPAM metrics
func (r *IpamRepository) GetSummaryStats(ctx context.Context) (*domain.IpamSummaryStats, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var stats domain.IpamSummaryStats

	// Subnets count & total monitored host capacity
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_ips), 0), COALESCE(SUM(total_used_ips), 0), COALESCE(SUM(total_unused_ips), 0)
		FROM ipam_subnets
	`).Scan(&stats.TotalSubnets, &stats.TotalMonitored, &stats.TotalUsedIPs, &stats.TotalUnusedIPs)
	if err != nil {
		return nil, err
	}

	// Address-level breakdowns
	err = pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'discovered'),
			COUNT(*) FILTER (WHERE is_online = true)
		FROM ipam_addresses
	`).Scan(&stats.TotalDiscovered, &stats.TotalOnline)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
