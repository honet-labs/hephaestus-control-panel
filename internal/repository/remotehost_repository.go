package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
)

type RemoteHostRepository struct{}

func NewRemoteHostRepository() *RemoteHostRepository {
	return &RemoteHostRepository{}
}

func (r *RemoteHostRepository) List(ctx context.Context, userID int, userRole string) ([]domain.RemoteHostConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var query string
	var rows database.Rows

	if strings.EqualFold(userRole, "ADMIN") {
		query = `
			SELECT 
				r.id, r.name, r.host, r.port, r.username, r.auth_type, r.group_name, r.tags, r.user_id, r.created_at,
				COALESCE(u.username, 'Admin') AS owner_username,
				(r.user_id = $1 OR r.user_id IS NULL) AS is_owner,
				CASE WHEN (r.user_id = $1 OR r.user_id IS NULL) THEN 'owner' ELSE 'admin' END AS shared_access,
				(SELECT COUNT(*) FROM remote_host_shares rhs WHERE rhs.host_id = r.id) AS shares_count
			FROM remote_host_configs r
			LEFT JOIN users u ON r.user_id = u.id
			ORDER BY r.group_name ASC, r.name ASC
		`
		rows, err = pool.Query(ctx, query, userID)
	} else {
		query = `
			SELECT 
				r.id, r.name, r.host, r.port, r.username, r.auth_type, r.group_name, r.tags, r.user_id, r.created_at,
				COALESCE(u.username, 'System') AS owner_username,
				(r.user_id = $1) AS is_owner,
				CASE 
					WHEN r.user_id = $1 THEN 'owner'
					ELSE COALESCE(rhs.permission, 'read')
				END AS shared_access,
				(SELECT COUNT(*) FROM remote_host_shares s WHERE s.host_id = r.id) AS shares_count
			FROM remote_host_configs r
			LEFT JOIN users u ON r.user_id = u.id
			LEFT JOIN remote_host_shares rhs ON r.id = rhs.host_id AND rhs.user_id = $1
			WHERE r.user_id = $1 OR rhs.user_id = $1
			ORDER BY r.group_name ASC, r.name ASC
		`
		rows, err = pool.Query(ctx, query, userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.RemoteHostConfig
	for rows.Next() {
		var c domain.RemoteHostConfig
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.GroupName, &c.Tags, &c.UserID, &c.CreatedAt,
			&c.OwnerUsername, &c.IsOwner, &c.SharedAccess, &c.SharesCount,
		); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *RemoteHostRepository) GetByID(ctx context.Context, id string, userID int, userRole string) (*domain.RemoteHostConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var query string
	var c domain.RemoteHostConfig

	if strings.EqualFold(userRole, "ADMIN") {
		query = `
			SELECT 
				r.id, r.name, r.host, r.port, r.username, r.auth_type, r.group_name, r.tags, r.user_id, r.created_at,
				COALESCE(u.username, 'Admin') AS owner_username,
				(r.user_id = $2 OR r.user_id IS NULL) AS is_owner,
				CASE WHEN (r.user_id = $2 OR r.user_id IS NULL) THEN 'owner' ELSE 'admin' END AS shared_access,
				(SELECT COUNT(*) FROM remote_host_shares rhs WHERE rhs.host_id = r.id) AS shares_count
			FROM remote_host_configs r
			LEFT JOIN users u ON r.user_id = u.id
			WHERE r.id = $1
		`
		err = pool.QueryRow(ctx, query, id, userID).Scan(
			&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.GroupName, &c.Tags, &c.UserID, &c.CreatedAt,
			&c.OwnerUsername, &c.IsOwner, &c.SharedAccess, &c.SharesCount,
		)
	} else {
		query = `
			SELECT 
				r.id, r.name, r.host, r.port, r.username, r.auth_type, r.group_name, r.tags, r.user_id, r.created_at,
				COALESCE(u.username, 'System') AS owner_username,
				(r.user_id = $2) AS is_owner,
				CASE 
					WHEN r.user_id = $2 THEN 'owner'
					ELSE COALESCE(rhs.permission, 'read')
				END AS shared_access,
				(SELECT COUNT(*) FROM remote_host_shares s WHERE s.host_id = r.id) AS shares_count
			FROM remote_host_configs r
			LEFT JOIN users u ON r.user_id = u.id
			LEFT JOIN remote_host_shares rhs ON r.id = rhs.host_id AND rhs.user_id = $2
			WHERE r.id = $1 AND (r.user_id = $2 OR rhs.user_id = $2)
		`
		err = pool.QueryRow(ctx, query, id, userID).Scan(
			&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.GroupName, &c.Tags, &c.UserID, &c.CreatedAt,
			&c.OwnerUsername, &c.IsOwner, &c.SharedAccess, &c.SharesCount,
		)
	}

	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *RemoteHostRepository) GetRawByID(ctx context.Context, id string) (*domain.RemoteHostConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, host, port, username, auth_type, password, ssh_key, group_name, tags, user_id, created_at 
              FROM remote_host_configs WHERE id = $1`
	var c domain.RemoteHostConfig
	err = pool.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.Password, &c.SSHKey, &c.GroupName, &c.Tags, &c.UserID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}

	if c.Password != nil && *c.Password != "" {
		if decrypted, err := config.DecryptText(*c.Password); err == nil {
			c.Password = &decrypted
		}
	}
	if c.SSHKey != nil && *c.SSHKey != "" {
		if decrypted, err := config.DecryptText(*c.SSHKey); err == nil {
			c.SSHKey = &decrypted
		}
	}

	return &c, nil
}

func (r *RemoteHostRepository) CheckAccess(ctx context.Context, hostID string, userID int, userRole string) (hasAccess bool, isOwner bool, perm string, err error) {
	if strings.EqualFold(userRole, "ADMIN") {
		return true, true, "manage", nil
	}

	pool, err := database.GetPool()
	if err != nil {
		return false, false, "", err
	}

	query := `
		SELECT 
			r.user_id,
			(r.user_id = $2) AS is_owner,
			COALESCE(rhs.permission, '') AS share_perm
		FROM remote_host_configs r
		LEFT JOIN remote_host_shares rhs ON r.id = rhs.host_id AND rhs.user_id = $2
		WHERE r.id = $1
	`
	var ownerID *int
	var ownerBool bool
	var sharePerm string
	err = pool.QueryRow(ctx, query, hostID, userID).Scan(&ownerID, &ownerBool, &sharePerm)
	if err != nil {
		return false, false, "", err
	}

	if ownerBool {
		return true, true, "manage", nil
	}
	if sharePerm != "" {
		return true, false, sharePerm, nil
	}

	return false, false, "", nil
}

func (r *RemoteHostRepository) Save(ctx context.Context, cfg domain.RemoteHostConfig, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	// Check if updating an existing host
	var existingOwnerID *int
	checkQuery := `SELECT user_id FROM remote_host_configs WHERE id = $1`
	err = pool.QueryRow(ctx, checkQuery, cfg.ID).Scan(&existingOwnerID)
	if err == nil {
		// Existing host: only owner or ADMIN can edit host details
		if !strings.EqualFold(userRole, "ADMIN") && (existingOwnerID == nil || *existingOwnerID != userID) {
			return errors.New("permission denied: only the host owner or administrator can edit this server")
		}
	}

	var encPassword, encKey *string
	if cfg.Password != nil && *cfg.Password != "" && *cfg.Password != "********" {
		if enc, err := config.EncryptText(*cfg.Password); err == nil {
			encPassword = &enc
		}
	}
	if cfg.SSHKey != nil && *cfg.SSHKey != "" && *cfg.SSHKey != "********" {
		if enc, err := config.EncryptText(*cfg.SSHKey); err == nil {
			encKey = &enc
		}
	}

	query := `INSERT INTO remote_host_configs (id, name, host, port, username, auth_type, password, ssh_key, group_name, tags, user_id)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, host = EXCLUDED.host, port = EXCLUDED.port, username = EXCLUDED.username,
                auth_type = EXCLUDED.auth_type,
                password = COALESCE($7, remote_host_configs.password),
                ssh_key = COALESCE($8, remote_host_configs.ssh_key),
                group_name = EXCLUDED.group_name, tags = EXCLUDED.tags,
                user_id = COALESCE(remote_host_configs.user_id, EXCLUDED.user_id)`

	_, err = pool.Exec(ctx, query, cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.Username, cfg.AuthType, encPassword, encKey, cfg.GroupName, cfg.Tags, userID)
	return err
}

func (r *RemoteHostRepository) Delete(ctx context.Context, id string, userID int, userRole string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if strings.EqualFold(userRole, "ADMIN") {
		res, err := pool.Exec(ctx, `DELETE FROM remote_host_configs WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return errors.New("host not found")
		}
		return nil
	}

	// Non-admin can only delete their own hosts
	res, err := pool.Exec(ctx, `DELETE FROM remote_host_configs WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("permission denied: only the host owner can delete this server")
	}
	return nil
}

// ==================== SHARE ACCESS METHODS ====================

func (r *RemoteHostRepository) ListShares(ctx context.Context, hostID string) ([]domain.RemoteHostShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			s.id, s.host_id, s.user_id, u.username, s.permission, 
			s.shared_by, COALESCE(sb.username, 'Admin') AS shared_by_username, s.created_at
		FROM remote_host_shares s
		JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE s.host_id = $1
		ORDER BY s.created_at DESC
	`
	rows, err := pool.Query(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []domain.RemoteHostShare
	for rows.Next() {
		var s domain.RemoteHostShare
		if err := rows.Scan(&s.ID, &s.HostID, &s.UserID, &s.Username, &s.Permission, &s.SharedBy, &s.SharedByUsername, &s.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	return shares, nil
}

func (r *RemoteHostRepository) AddShare(ctx context.Context, hostID string, targetUserID int, permission string, sharedBy int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if permission == "" {
		permission = "read"
	}
	permission = strings.ToLower(permission)
	if permission != "read" && permission != "manage" {
		permission = "read"
	}

	shareID := fmt.Sprintf("rhs-%s", uuid.New().String()[:8])
	query := `
		INSERT INTO remote_host_shares (id, host_id, user_id, permission, shared_by, created_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (host_id, user_id) DO UPDATE SET
			permission = EXCLUDED.permission,
			shared_by = EXCLUDED.shared_by,
			created_at = CURRENT_TIMESTAMP
	`
	_, err = pool.Exec(ctx, query, shareID, hostID, targetUserID, permission, sharedBy)
	return err
}

func (r *RemoteHostRepository) DeleteShare(ctx context.Context, hostID string, targetUserID int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM remote_host_shares WHERE host_id = $1 AND user_id = $2`
	_, err = pool.Exec(ctx, query, hostID, targetUserID)
	return err
}

func (r *RemoteHostRepository) ListAvailableUsers(ctx context.Context) ([]map[string]interface{}, error) {
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
	return users, nil
}

