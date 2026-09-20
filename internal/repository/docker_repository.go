package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DockerRepository struct{}

func NewDockerRepository() *DockerRepository {
	return &DockerRepository{}
}

// ListConnections retrieves all configured Docker hosts
func (r *DockerRepository) ListConnections(ctx context.Context) ([]domain.DockerConnection, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		SELECT id, name, host_type, socket_path, tcp_url, remote_host_id,
		       ssh_host, ssh_port, ssh_user, ssh_auth, is_active, is_default, created_at, updated_at
		FROM docker_connections
		ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query docker_connections: %w", err)
	}
	defer rows.Close()

	var connections []domain.DockerConnection
	for rows.Next() {
		var c domain.DockerConnection
		if err := rows.Scan(
			&c.ID, &c.Name, &c.HostType, &c.SocketPath, &c.TcpURL, &c.RemoteHostID,
			&c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &c.IsActive, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		connections = append(connections, c)
	}

	// Auto-seed default local connection if table is completely empty
	if len(connections) == 0 {
		defaultConn := domain.DockerConnection{
			ID:         "docker-local-default",
			Name:       "Local Docker Host",
			HostType:   "local",
			SocketPath: "/var/run/docker.sock",
			IsActive:   true,
			IsDefault:  true,
		}
		saved, err := r.SaveConnection(ctx, defaultConn)
		if err == nil && saved != nil {
			connections = append(connections, *saved)
		}
	}

	return connections, nil
}

// GetConnectionByID retrieves a Docker connection by ID and decrypts SSH secrets
func (r *DockerRepository) GetConnectionByID(ctx context.Context, id string) (*domain.DockerConnection, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var c domain.DockerConnection
	var encPassword, encKey *string

	err = pool.QueryRow(ctx, `
		SELECT id, name, host_type, socket_path, tcp_url, remote_host_id,
		       ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password_encrypted, ssh_key_encrypted,
		       is_active, is_default, created_at, updated_at
		FROM docker_connections
		WHERE id = $1
	`, id).Scan(
		&c.ID, &c.Name, &c.HostType, &c.SocketPath, &c.TcpURL, &c.RemoteHostID,
		&c.SSHHost, &c.SSHPort, &c.SSHUser, &c.SSHAuth, &encPassword, &encKey,
		&c.IsActive, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get docker connection: %w", err)
	}

	if encPassword != nil && *encPassword != "" {
		if dec, err := config.DecryptText(*encPassword); err == nil {
			c.SSHPassword = &dec
		} else {
			c.SSHPassword = encPassword
		}
	}

	if encKey != nil && *encKey != "" {
		if dec, err := config.DecryptText(*encKey); err == nil {
			c.SSHKey = &dec
		} else {
			c.SSHKey = encKey
		}
	}

	return &c, nil
}

// GetDefaultConnection returns the active default Docker connection
func (r *DockerRepository) GetDefaultConnection(ctx context.Context) (*domain.DockerConnection, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var id string
	err = pool.QueryRow(ctx, `
		SELECT id FROM docker_connections
		WHERE is_active = true
		ORDER BY is_default DESC, created_at ASC
		LIMIT 1
	`).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Trigger list to auto-seed local connection
			list, _ := r.ListConnections(ctx)
			if len(list) > 0 {
				return r.GetConnectionByID(ctx, list[0].ID)
			}
			return nil, nil
		}
		return nil, err
	}

	return r.GetConnectionByID(ctx, id)
}

// SaveConnection creates or updates a Docker connection configuration
func (r *DockerRepository) SaveConnection(ctx context.Context, c domain.DockerConnection) (*domain.DockerConnection, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	if c.ID == "" {
		c.ID = fmt.Sprintf("docker-%s", uuid.New().String()[:8])
	}
	if c.Name == "" {
		c.Name = "Docker Host"
	}
	if c.HostType == "" {
		c.HostType = "local"
	}
	if c.SocketPath == "" {
		c.SocketPath = "/var/run/docker.sock"
	}

	// Encrypt SSH secrets if provided
	var encPassword, encKey *string
	if c.SSHPassword != nil && *c.SSHPassword != "" {
		enc, err := config.EncryptText(*c.SSHPassword)
		if err == nil {
			encPassword = &enc
		}
	} else if c.ID != "" {
		// Retain existing password if not updated
		existing, _ := r.GetConnectionByID(ctx, c.ID)
		if existing != nil && existing.SSHPassword != nil {
			enc, _ := config.EncryptText(*existing.SSHPassword)
			encPassword = &enc
		}
	}

	if c.SSHKey != nil && *c.SSHKey != "" {
		enc, err := config.EncryptText(*c.SSHKey)
		if err == nil {
			encKey = &enc
		}
	} else if c.ID != "" {
		existing, _ := r.GetConnectionByID(ctx, c.ID)
		if existing != nil && existing.SSHKey != nil {
			enc, _ := config.EncryptText(*existing.SSHKey)
			encKey = &enc
		}
	}

	// If this connection is marked as default, unset other defaults
	if c.IsDefault {
		_, _ = pool.Exec(ctx, `UPDATE docker_connections SET is_default = false WHERE id <> $1`, c.ID)
	}

	now := time.Now()
	_, err = pool.Exec(ctx, `
		INSERT INTO docker_connections (
			id, name, host_type, socket_path, tcp_url, remote_host_id,
			ssh_host, ssh_port, ssh_user, ssh_auth, ssh_password_encrypted, ssh_key_encrypted,
			is_active, is_default, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			host_type = EXCLUDED.host_type,
			socket_path = EXCLUDED.socket_path,
			tcp_url = EXCLUDED.tcp_url,
			remote_host_id = EXCLUDED.remote_host_id,
			ssh_host = EXCLUDED.ssh_host,
			ssh_port = EXCLUDED.ssh_port,
			ssh_user = EXCLUDED.ssh_user,
			ssh_auth = EXCLUDED.ssh_auth,
			ssh_password_encrypted = CASE WHEN EXCLUDED.ssh_password_encrypted IS NOT NULL THEN EXCLUDED.ssh_password_encrypted ELSE docker_connections.ssh_password_encrypted END,
			ssh_key_encrypted = CASE WHEN EXCLUDED.ssh_key_encrypted IS NOT NULL THEN EXCLUDED.ssh_key_encrypted ELSE docker_connections.ssh_key_encrypted END,
			is_active = EXCLUDED.is_active,
			is_default = EXCLUDED.is_default,
			updated_at = EXCLUDED.updated_at
	`, c.ID, c.Name, c.HostType, c.SocketPath, c.TcpURL, c.RemoteHostID,
		c.SSHHost, c.SSHPort, c.SSHUser, c.SSHAuth, encPassword, encKey,
		c.IsActive, c.IsDefault, now)

	if err != nil {
		return nil, fmt.Errorf("failed to save docker connection: %w", err)
	}

	return r.GetConnectionByID(ctx, c.ID)
}

// SetDefaultConnection marks a connection as default
func (r *DockerRepository) SetDefaultConnection(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE docker_connections SET is_default = false`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE docker_connections SET is_default = true, is_active = true WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DeleteConnection removes a Docker connection
func (r *DockerRepository) DeleteConnection(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `DELETE FROM docker_connections WHERE id = $1`, id)
	return err
}
