package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type VaultwardenRepository struct{}

func NewVaultwardenRepository() *VaultwardenRepository {
	return &VaultwardenRepository{}
}

// GetConfig returns the raw config with decrypted MasterPassword (for internal service use)
func (r *VaultwardenRepository) GetConfig(ctx context.Context) (*domain.VaultwardenConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	var cfg domain.VaultwardenConfig
	var encPassword string
	var cachedJSON []byte

	err = pool.QueryRow(ctx, `
		SELECT id, name, server_url, email, master_password_encrypted, is_active, last_synced_at, cached_ciphers, created_at, updated_at
		FROM vaultwarden_configs
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(
		&cfg.ID,
		&cfg.Name,
		&cfg.ServerURL,
		&cfg.Email,
		&encPassword,
		&cfg.IsActive,
		&cfg.LastSyncedAt,
		&cachedJSON,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query vaultwarden_configs: %w", err)
	}

	if encPassword != "" {
		decrypted, err := config.DecryptText(encPassword)
		if err == nil {
			cfg.MasterPassword = decrypted
		} else {
			cfg.MasterPassword = encPassword
		}
	}

	if len(cachedJSON) > 0 {
		_ = json.Unmarshal(cachedJSON, &cfg.CachedCiphers)
	}

	return &cfg, nil
}

// GetConfigPublic returns the config with MasterPassword stripped/masked for API responses
func (r *VaultwardenRepository) GetConfigPublic(ctx context.Context) (*domain.VaultwardenConfig, error) {
	cfg, err := r.GetConfig(ctx)
	if err != nil || cfg == nil {
		return cfg, err
	}
	// Never return plaintext password to client
	cfg.MasterPassword = ""
	return cfg, nil
}

// SaveConfig saves or updates the Vaultwarden connection configuration
func (r *VaultwardenRepository) SaveConfig(ctx context.Context, cfg domain.VaultwardenConfig) (*domain.VaultwardenConfig, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("vw-%s", uuid.New().String()[:8])
	}
	if cfg.Name == "" {
		cfg.Name = "Vaultwarden"
	}

	var encPassword string
	if cfg.MasterPassword != "" {
		encrypted, err := config.EncryptText(cfg.MasterPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt master password: %w", err)
		}
		encPassword = encrypted
	} else {
		// Retain existing password if not provided
		existing, err := r.GetConfig(ctx)
		if err == nil && existing != nil && existing.MasterPassword != "" {
			encPassword, _ = config.EncryptText(existing.MasterPassword)
		}
	}

	now := time.Now()
	_, err = pool.Exec(ctx, `
		INSERT INTO vaultwarden_configs (
			id, name, server_url, email, master_password_encrypted, is_active, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			server_url = EXCLUDED.server_url,
			email = EXCLUDED.email,
			master_password_encrypted = CASE WHEN EXCLUDED.master_password_encrypted <> '' THEN EXCLUDED.master_password_encrypted ELSE vaultwarden_configs.master_password_encrypted END,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`, cfg.ID, cfg.Name, cfg.ServerURL, cfg.Email, encPassword, cfg.IsActive, now)

	if err != nil {
		return nil, fmt.Errorf("failed to save vaultwarden config: %w", err)
	}

	return r.GetConfigPublic(ctx)
}

// UpdateCachedCiphers persists decrypted ciphers in JSONB cache for fast retrieval
func (r *VaultwardenRepository) UpdateCachedCiphers(ctx context.Context, id string, ciphers []domain.VaultCredentialItem, lastSynced time.Time) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	rawJSON, err := json.Marshal(ciphers)
	if err != nil {
		return fmt.Errorf("failed to marshal ciphers: %w", err)
	}

	_, err = pool.Exec(ctx, `
		UPDATE vaultwarden_configs
		SET cached_ciphers = $1, last_synced_at = $2, updated_at = NOW()
		WHERE id = $3
	`, rawJSON, lastSynced, id)

	return err
}

// DeleteConfig removes the Vaultwarden integration configuration
func (r *VaultwardenRepository) DeleteConfig(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if id == "" || id == "active" {
		_, err = pool.Exec(ctx, `DELETE FROM vaultwarden_configs`)
	} else {
		_, err = pool.Exec(ctx, `DELETE FROM vaultwarden_configs WHERE id = $1`, id)
	}
	return err
}
