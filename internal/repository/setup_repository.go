package repository

import (
	"context"
	"fmt"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

// Serialize setup in PostgreSQL as well as in-process. A missing config flag
// must never allow a second administrator to be provisioned publicly.
func (r *UserRepository) CreateInitialAdmin(ctx context.Context, username, passwordHash string) (*domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "LOCK TABLE users IN EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users)").Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("setup has already been completed; use the administrator recovery CLI")
	}
	user := &domain.User{Username: username, Role: "ADMIN", Permissions: map[string]string{"*": "manage"}}
	if err = tx.QueryRow(ctx, "INSERT INTO users (username, password_hash, role, force_password_change) VALUES ($1, $2, 'ADMIN', false) RETURNING id, created_at", username, passwordHash).Scan(&user.ID, &user.CreatedAt); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO app_config (key, value) VALUES ('setup_completed', 'true') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return user, nil
}
