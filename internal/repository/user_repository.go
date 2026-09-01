package repository

import (
	"context"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, username, password_hash, role, force_password_change, created_at FROM users WHERE username = $1`
	row := pool.QueryRow(ctx, query, username)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int) (*domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, username, password_hash, role, force_password_change, created_at FROM users WHERE id = $1`
	row := pool.QueryRow(ctx, query, id)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `SELECT id, username, role, force_password_change, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) Create(ctx context.Context, username, passwordHash, role string, forceChange bool) (*domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO users (username, password_hash, role, force_password_change) 
              VALUES ($1, $2, $3, $4) RETURNING id, created_at`
	var u domain.User
	u.Username = username
	u.Role = role
	u.ForcePasswordChange = forceChange
	err = pool.QueryRow(ctx, query, username, passwordHash, role, forceChange).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int, passwordHash string, forceChange bool) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `UPDATE users SET password_hash = $1, force_password_change = $2 WHERE id = $3`, passwordHash, forceChange, id)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// Session methods
func (r *UserRepository) CreateSession(ctx context.Context, userID int, tokenHash string, duration time.Duration) (*domain.UserSession, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(duration)
	var s domain.UserSession
	s.UserID = userID
	s.Token = tokenHash
	s.ExpiresAt = expiresAt

	query := `INSERT INTO user_sessions (user_id, token, expires_at) VALUES ($1, $2, $3) RETURNING id, created_at`
	err = pool.QueryRow(ctx, query, userID, tokenHash, expiresAt).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *UserRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*domain.UserSession, *domain.User, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, nil, err
	}

	query := `SELECT s.id, s.user_id, s.token, s.expires_at, s.created_at,
                     u.id, u.username, u.role, u.force_password_change, u.created_at
              FROM user_sessions s
              JOIN users u ON s.user_id = u.id
              WHERE s.token = $1 AND s.expires_at > NOW()`
	row := pool.QueryRow(ctx, query, tokenHash)

	var s domain.UserSession
	var u domain.User
	err = row.Scan(&s.ID, &s.UserID, &s.Token, &s.ExpiresAt, &s.CreatedAt,
		&u.ID, &u.Username, &u.Role, &u.ForcePasswordChange, &u.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	// Extend session sliding window (max 7 days)
	_, _ = pool.Exec(ctx, `UPDATE user_sessions SET expires_at = LEAST(NOW() + INTERVAL '24 hours', created_at + INTERVAL '7 days') WHERE token = $1`, tokenHash)

	return &s, &u, nil
}

func (r *UserRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM user_sessions WHERE token = $1`, tokenHash)
	return err
}

func (r *UserRepository) CleanExpiredSessions(ctx context.Context) (int64, error) {
	pool, err := database.GetPool()
	if err != nil {
		return 0, err
	}
	res, err := pool.Exec(ctx, `DELETE FROM user_sessions WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// Activity Log methods
func (r *UserRepository) LogActivity(ctx context.Context, module, action, details, status string, userID *int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `INSERT INTO activity_logs (module, action, details, status, user_id) VALUES ($1, $2, $3, $4, $5)`,
		module, action, details, status, userID)
	return err
}

func (r *UserRepository) ListActivityLogs(ctx context.Context, limit, offset int) ([]domain.ActivityLog, int, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, 0, err
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM activity_logs`).Scan(&count)

	query := `SELECT l.id, l.timestamp, l.module, l.action, l.details, l.status, l.user_id, u.username
              FROM activity_logs l
              LEFT JOIN users u ON l.user_id = u.id
              ORDER BY l.timestamp DESC LIMIT $1 OFFSET $2`
	rows, err := pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []domain.ActivityLog
	for rows.Next() {
		var l domain.ActivityLog
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Module, &l.Action, &l.Details, &l.Status, &l.UserID, &l.Username); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, count, nil
}
