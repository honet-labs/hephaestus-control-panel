package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DockerRepository struct{}

func NewDockerRepository() *DockerRepository {
	return &DockerRepository{}
}

// ensureTable guarantees that docker_connections exists even if schema migrations had partial failures
func (r *DockerRepository) ensureTable(ctx context.Context, pool *pgxpool.Pool) {
	_, _ = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS docker_connections (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			host_type VARCHAR(50) NOT NULL DEFAULT 'local',
			socket_path VARCHAR(255) DEFAULT '/var/run/docker.sock',
			tcp_url VARCHAR(255),
			remote_host_id VARCHAR(50) REFERENCES remote_host_configs(id) ON DELETE SET NULL,
			ssh_host VARCHAR(255),
			ssh_port INTEGER DEFAULT 22,
			ssh_user VARCHAR(255),
			ssh_auth VARCHAR(50) DEFAULT 'password',
			ssh_password_encrypted TEXT,
			ssh_key_encrypted TEXT,
			is_active BOOLEAN DEFAULT true,
			is_default BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS docker_container_metadata (
			connection_id VARCHAR(50) NOT NULL,
			container_id VARCHAR(100) NOT NULL,
			container_name VARCHAR(255) NOT NULL,
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			username VARCHAR(100),
			visibility VARCHAR(20) NOT NULL DEFAULT 'public',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(connection_id, container_id)
		);
		CREATE INDEX IF NOT EXISTS idx_docker_container_meta_user ON docker_container_metadata(user_id);

		CREATE TABLE IF NOT EXISTS docker_container_shares (
			id VARCHAR(50) PRIMARY KEY,
			connection_id VARCHAR(50) NOT NULL,
			container_id VARCHAR(100) NOT NULL,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			permission VARCHAR(20) NOT NULL DEFAULT 'read',
			shared_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(connection_id, container_id, user_id)
		);
		CREATE INDEX IF NOT EXISTS idx_docker_container_shares_container ON docker_container_shares(connection_id, container_id);
		CREATE INDEX IF NOT EXISTS idx_docker_container_shares_user ON docker_container_shares(user_id);
	`)
}

// ListConnections retrieves all configured Docker hosts
func (r *DockerRepository) ListConnections(ctx context.Context) ([]domain.DockerConnection, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	r.ensureTable(ctx, pool)

	rows, err := pool.Query(ctx, `
		SELECT id, name, host_type, COALESCE(socket_path, ''), COALESCE(tcp_url, ''), remote_host_id,
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
		if c.HostType == "local" || c.HostType == "socket" || c.HostType == "" {
			c.Driver = "socket"
			c.HostType = "local"
			if c.SocketPath == "" {
				c.SocketPath = "/var/run/docker.sock"
			}
		} else {
			c.Driver = c.HostType
		}
		connections = append(connections, c)
	}

	// Auto-seed default local connection if table is completely empty
	if len(connections) == 0 {
		defaultConn := domain.DockerConnection{
			ID:         "docker-local-default",
			Name:       "Local Docker Host",
			HostType:   "local",
			Driver:     "socket",
			SocketPath: "/var/run/docker.sock",
			IsActive:   true,
			IsDefault:  true,
		}
		saved, err := r.SaveConnection(ctx, defaultConn)
		if err == nil && saved != nil {
			connections = append(connections, *saved)
		} else {
			connections = append(connections, defaultConn)
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

	r.ensureTable(ctx, pool)

	var c domain.DockerConnection
	var encPassword, encKey *string

	err = pool.QueryRow(ctx, `
		SELECT id, name, host_type, COALESCE(socket_path, ''), COALESCE(tcp_url, ''), remote_host_id,
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

	if c.HostType == "local" || c.HostType == "socket" || c.HostType == "" {
		c.Driver = "socket"
		c.HostType = "local"
		if c.SocketPath == "" {
			c.SocketPath = "/var/run/docker.sock"
		}
	} else {
		c.Driver = c.HostType
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

	r.ensureTable(ctx, pool)

	if c.ID == "" {
		c.ID = fmt.Sprintf("docker-%s", uuid.New().String()[:8])
	}
	if c.Name == "" {
		c.Name = "Docker Host"
	}
	if c.HostType == "" {
		c.HostType = "local"
	}
	if c.HostType == "local" || c.HostType == "socket" {
		c.HostType = "local"
		c.Driver = "socket"
		if c.SocketPath == "" {
			c.SocketPath = "/var/run/docker.sock"
		}
	} else {
		c.Driver = c.HostType
	}

	if c.RemoteHostID != nil && *c.RemoteHostID == "" {
		c.RemoteHostID = nil
	}
	if c.SSHHost != nil && *c.SSHHost == "" {
		c.SSHHost = nil
	}
	if c.SSHUser != nil && *c.SSHUser == "" {
		c.SSHUser = nil
	}
	if c.SSHAuth != nil && *c.SSHAuth == "" {
		c.SSHAuth = nil
	}
	if c.SSHPassword != nil && *c.SSHPassword == "" {
		c.SSHPassword = nil
	}
	if c.SSHKey != nil && *c.SSHKey == "" {
		c.SSHKey = nil
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

// -------------------------------------------------------------
// Container Metadata & Visibility Persistence
// -------------------------------------------------------------

func (r *DockerRepository) SaveContainerMetadata(ctx context.Context, meta domain.ContainerMetadata) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	r.ensureTable(ctx, pool)

	if meta.Visibility == "" {
		meta.Visibility = "public"
	}

	query := `
		INSERT INTO docker_container_metadata (connection_id, container_id, container_name, user_id, username, visibility, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (connection_id, container_id) DO UPDATE SET
			container_name = EXCLUDED.container_name,
			user_id = COALESCE(EXCLUDED.user_id, docker_container_metadata.user_id),
			username = COALESCE(EXCLUDED.username, docker_container_metadata.username),
			visibility = EXCLUDED.visibility,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = pool.Exec(ctx, query, meta.ConnectionID, meta.ContainerID, meta.ContainerName, meta.UserID, meta.Username, meta.Visibility)
	return err
}

func (r *DockerRepository) ListContainerMetadata(ctx context.Context, connectionID string) (map[string]*domain.ContainerMetadata, error) {
	pool, err := database.GetPool()
	if err != nil {
		return make(map[string]*domain.ContainerMetadata), nil
	}
	r.ensureTable(ctx, pool)

	rows, err := pool.Query(ctx, `
		SELECT connection_id, container_id, container_name, user_id, COALESCE(username, ''), visibility, created_at, updated_at
		FROM docker_container_metadata
		WHERE connection_id = $1 OR connection_id = '' OR $1 = ''
	`, connectionID)
	if err != nil {
		return make(map[string]*domain.ContainerMetadata), nil
	}
	defer rows.Close()

	result := make(map[string]*domain.ContainerMetadata)
	for rows.Next() {
		var m domain.ContainerMetadata
		if err := rows.Scan(&m.ConnectionID, &m.ContainerID, &m.ContainerName, &m.UserID, &m.Username, &m.Visibility, &m.CreatedAt, &m.UpdatedAt); err == nil {
			// Index by full ID, short ID, and name for flexible matching
			result[m.ContainerID] = &m
			if len(m.ContainerID) >= 12 {
				result[m.ContainerID[:12]] = &m
			}
			if m.ContainerName != "" {
				result["name:"+strings.TrimPrefix(m.ContainerName, "/")] = &m
			}
		}
	}
	return result, nil
}

func (r *DockerRepository) UpdateContainerVisibility(ctx context.Context, connectionID, containerID, visibility, containerName string, userID *int, username string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	r.ensureTable(ctx, pool)

	if visibility != "private" {
		visibility = "public"
	}

	query := `
		INSERT INTO docker_container_metadata (connection_id, container_id, container_name, user_id, username, visibility, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (connection_id, container_id) DO UPDATE SET
			visibility = $6,
			container_name = CASE WHEN $3 <> '' THEN $3 ELSE docker_container_metadata.container_name END,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = pool.Exec(ctx, query, connectionID, containerID, containerName, userID, username, visibility)
	return err
}

func (r *DockerRepository) DeleteContainerMetadata(ctx context.Context, connectionID, containerID string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `DELETE FROM docker_container_shares WHERE (connection_id = $1 OR connection_id = '') AND (container_id = $2 OR container_id LIKE $2 || '%' OR $2 LIKE container_id || '%')`, connectionID, containerID)
	_, err = pool.Exec(ctx, `DELETE FROM docker_container_metadata WHERE (connection_id = $1 OR connection_id = '') AND (container_id = $2 OR container_id LIKE $2 || '%')`, connectionID, containerID)
	return err
}

// ==================== CONTAINER ACCESS SHARING METHODS ====================

func (r *DockerRepository) ListShares(ctx context.Context, connectionID, containerID string) ([]domain.DockerContainerShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	r.ensureTable(ctx, pool)

	query := `
		SELECT s.id, s.connection_id, s.container_id, s.user_id, u.username, s.permission,
		       s.shared_by, COALESCE(sb.username, 'Admin') AS shared_by_username, s.created_at
		FROM docker_container_shares s
		JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE (s.connection_id = $1 OR s.connection_id = '' OR $1 = '')
		  AND (s.container_id = $2 OR s.container_id LIKE $2 || '%' OR $2 LIKE s.container_id || '%')
		ORDER BY s.created_at DESC
	`
	rows, err := pool.Query(ctx, query, connectionID, containerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []domain.DockerContainerShare
	for rows.Next() {
		var s domain.DockerContainerShare
		if err := rows.Scan(&s.ID, &s.ConnectionID, &s.ContainerID, &s.UserID, &s.Username, &s.Permission, &s.SharedBy, &s.SharedByUsername, &s.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	return shares, nil
}

func (r *DockerRepository) AddShare(ctx context.Context, connectionID, containerID string, targetUserID int, permission string, sharedBy int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	r.ensureTable(ctx, pool)

	if permission == "" {
		permission = "read"
	}
	permission = strings.ToLower(permission)
	if permission != "read" && permission != "manage" {
		permission = "read"
	}

	shareID := fmt.Sprintf("dcs-%s", uuid.New().String()[:8])
	query := `
		INSERT INTO docker_container_shares (id, connection_id, container_id, user_id, permission, shared_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (connection_id, container_id, user_id) DO UPDATE SET
			permission = EXCLUDED.permission,
			shared_by = EXCLUDED.shared_by,
			created_at = CURRENT_TIMESTAMP
	`
	_, err = pool.Exec(ctx, query, shareID, connectionID, containerID, targetUserID, permission, sharedBy)
	return err
}

func (r *DockerRepository) DeleteShare(ctx context.Context, connectionID, containerID string, targetUserID int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	query := `DELETE FROM docker_container_shares 
	          WHERE (connection_id = $1 OR connection_id = '' OR $1 = '')
	            AND (container_id = $2 OR container_id LIKE $2 || '%' OR $2 LIKE container_id || '%')
	            AND user_id = $3`
	_, err = pool.Exec(ctx, query, connectionID, containerID, targetUserID)
	return err
}

func (r *DockerRepository) ListContainerSharesMap(ctx context.Context, connectionID string) (map[string][]domain.DockerContainerShare, error) {
	pool, err := database.GetPool()
	if err != nil {
		return make(map[string][]domain.DockerContainerShare), nil
	}
	r.ensureTable(ctx, pool)

	query := `
		SELECT s.id, s.connection_id, s.container_id, s.user_id, u.username, s.permission,
		       s.shared_by, COALESCE(sb.username, 'Admin') AS shared_by_username, s.created_at
		FROM docker_container_shares s
		JOIN users u ON s.user_id = u.id
		LEFT JOIN users sb ON s.shared_by = sb.id
		WHERE s.connection_id = $1 OR s.connection_id = '' OR $1 = ''
		ORDER BY s.created_at DESC
	`
	rows, err := pool.Query(ctx, query, connectionID)
	if err != nil {
		return make(map[string][]domain.DockerContainerShare), nil
	}
	defer rows.Close()

	result := make(map[string][]domain.DockerContainerShare)
	for rows.Next() {
		var s domain.DockerContainerShare
		if err := rows.Scan(&s.ID, &s.ConnectionID, &s.ContainerID, &s.UserID, &s.Username, &s.Permission, &s.SharedBy, &s.SharedByUsername, &s.CreatedAt); err == nil {
			result[s.ContainerID] = append(result[s.ContainerID], s)
			if len(s.ContainerID) >= 12 {
				result[s.ContainerID[:12]] = append(result[s.ContainerID[:12]], s)
			}
		}
	}
	return result, nil
}

func (r *DockerRepository) ListAvailableUsers(ctx context.Context) ([]map[string]interface{}, error) {
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
		if err := rows.Scan(&id, &username, &role); err == nil {
			users = append(users, map[string]interface{}{
				"id":       id,
				"username": username,
				"role":     role,
			})
		}
	}
	return users, nil
}

