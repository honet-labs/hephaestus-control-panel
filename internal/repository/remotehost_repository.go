package repository

import (
	"context"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type RemoteHostRepository struct{}

func NewRemoteHostRepository() *RemoteHostRepository {
	return &RemoteHostRepository{}
}

func (r *RemoteHostRepository) List(ctx context.Context) ([]domain.RemoteHostConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, host, port, username, auth_type, group_name, tags, created_at 
              FROM remote_host_configs ORDER BY group_name ASC, name ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.RemoteHostConfig
	for rows.Next() {
		var c domain.RemoteHostConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.GroupName, &c.Tags, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *RemoteHostRepository) GetByID(ctx context.Context, id string) (*domain.RemoteHostConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, host, port, username, auth_type, group_name, tags, created_at 
              FROM remote_host_configs WHERE id = $1`
	var c domain.RemoteHostConfig
	err = pool.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.GroupName, &c.Tags, &c.CreatedAt)
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

	query := `SELECT id, name, host, port, username, auth_type, password, ssh_key, group_name, tags, created_at 
              FROM remote_host_configs WHERE id = $1`
	var c domain.RemoteHostConfig
	err = pool.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.AuthType, &c.Password, &c.SSHKey, &c.GroupName, &c.Tags, &c.CreatedAt)
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

func (r *RemoteHostRepository) Save(ctx context.Context, cfg domain.RemoteHostConfig) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
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

	query := `INSERT INTO remote_host_configs (id, name, host, port, username, auth_type, password, ssh_key, group_name, tags)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
              ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, host = EXCLUDED.host, port = EXCLUDED.port, username = EXCLUDED.username,
                auth_type = EXCLUDED.auth_type,
                password = COALESCE($7, remote_host_configs.password),
                ssh_key = COALESCE($8, remote_host_configs.ssh_key),
                group_name = EXCLUDED.group_name, tags = EXCLUDED.tags`

	_, err = pool.Exec(ctx, query, cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.Username, cfg.AuthType, encPassword, encKey, cfg.GroupName, cfg.Tags)
	return err
}

func (r *RemoteHostRepository) Delete(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `DELETE FROM remote_host_configs WHERE id = $1`, id)
	return err
}
