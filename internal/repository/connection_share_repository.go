package repository

import (
	"context"
	"fmt"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
)

type ConnectionShareRepository struct{}

func NewConnectionShareRepository() *ConnectionShareRepository {
	return &ConnectionShareRepository{}
}

// ListShares returns all users granted access to a specific connection
func (r *ConnectionShareRepository) ListShares(ctx context.Context, shareTable, fkCol, configID string) ([]domain.ConnectionShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.%s, s.user_id, u.username, s.permission,
		       s.shared_by, COALESCE(sb.username, 'Admin') AS shared_by_username, s.created_at
		FROM %s s
		JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE s.%s = $1
		ORDER BY s.created_at DESC
	`, fkCol, shareTable, fkCol)

	rows, err := pool.Query(ctx, query, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []domain.ConnectionShare
	for rows.Next() {
		var s domain.ConnectionShare
		if err := rows.Scan(
			&s.ID, &s.ConfigID, &s.UserID, &s.Username, &s.Permission,
			&s.SharedBy, &s.SharedByUsername, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	if shares == nil {
		shares = []domain.ConnectionShare{}
	}
	return shares, nil
}

// AddShare grants a user read or manage access to a connection
func (r *ConnectionShareRepository) AddShare(ctx context.Context, shareTable, fkCol, configID string, targetUserID int, permission string, sharedBy int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if permission != "manage" {
		permission = "read"
	}

	shareID := fmt.Sprintf("sh-%s", uuid.New().String()[:8])
	query := fmt.Sprintf(`
		INSERT INTO %s (id, %s, user_id, permission, shared_by, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (%s, user_id) DO UPDATE SET
			permission = EXCLUDED.permission,
			shared_by = EXCLUDED.shared_by,
			created_at = NOW()
	`, shareTable, fkCol, fkCol)

	_, err = pool.Exec(ctx, query, shareID, configID, targetUserID, permission, sharedBy)
	return err
}

// DeleteShare revokes a user's access to a connection
func (r *ConnectionShareRepository) DeleteShare(ctx context.Context, shareTable, fkCol, configID string, targetUserID int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND user_id = $2`, shareTable, fkCol)
	_, err = pool.Exec(ctx, query, configID, targetUserID)
	return err
}

// CheckAccess validates if a user can view or manage a specific connection
func (r *ConnectionShareRepository) CheckAccess(ctx context.Context, configTable, shareTable, fkCol, configID string, userID int, userRole string) (hasAccess bool, isOwner bool, perm string, err error) {
	if domain.IsAdminRole(userRole) {
		return true, true, "manage", nil
	}

	pool, err := database.GetPool()
	if err != nil {
		return false, false, "", err
	}

	query := fmt.Sprintf(`
		SELECT 
			c.user_id,
			COALESCE(c.visibility, 'private'),
			COALESCE(c.user_id = $2, false) AS is_owner,
			COALESCE(s.permission, '') AS share_perm
		FROM %s c
		LEFT JOIN %s s ON c.id = s.%s AND s.user_id = $2
		WHERE c.id = $1
	`, configTable, shareTable, fkCol)

	var ownerID *int
	var visibility string
	var ownerBool bool
	var sharePerm string
	err = pool.QueryRow(ctx, query, configID, userID).Scan(&ownerID, &visibility, &ownerBool, &sharePerm)
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

// UpdateVisibility toggles visibility between 'private' and 'public'
func (r *ConnectionShareRepository) UpdateVisibility(ctx context.Context, configTable, configID string, visibility string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if visibility != "public" {
		visibility = "private"
	}

	query := fmt.Sprintf(`UPDATE %s SET visibility = $1 WHERE id = $2`, configTable)
	_, err = pool.Exec(ctx, query, visibility, configID)
	return err
}
