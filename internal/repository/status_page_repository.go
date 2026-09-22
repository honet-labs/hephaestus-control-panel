package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type StatusPageRepository struct{}

func NewStatusPageRepository() *StatusPageRepository {
	return &StatusPageRepository{}
}

// -----------------------------------------------------------------------------
// Status Pages CRUD
// -----------------------------------------------------------------------------

func (r *StatusPageRepository) ListPages(ctx context.Context) ([]domain.StatusPage, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, title, slug, description, footer_text, theme, refresh_interval,
		       is_public, is_published, show_tags, custom_css, user_id, created_at, updated_at
		FROM status_pages
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []domain.StatusPage
	for rows.Next() {
		var p domain.StatusPage
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Slug, &p.Description, &p.FooterText, &p.Theme,
			&p.RefreshInterval, &p.IsPublic, &p.IsPublished, &p.ShowTags,
			&p.CustomCSS, &p.UserID, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Count items & groups
		items, _ := r.ListItems(ctx, p.ID)
		p.Items = items
		groups, _ := r.ListGroups(ctx, p.ID)
		p.Groups = groups
		incidents, _ := r.ListIncidents(ctx, p.ID, false)
		p.Incidents = incidents

		pages = append(pages, p)
	}

	return pages, nil
}

func (r *StatusPageRepository) GetPageByID(ctx context.Context, id string) (*domain.StatusPage, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, title, slug, description, footer_text, theme, refresh_interval,
		       is_public, is_published, show_tags, custom_css, user_id, created_at, updated_at
		FROM status_pages
		WHERE id = $1
	`
	var p domain.StatusPage
	err = pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Description, &p.FooterText, &p.Theme,
		&p.RefreshInterval, &p.IsPublic, &p.IsPublished, &p.ShowTags,
		&p.CustomCSS, &p.UserID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	items, _ := r.ListItems(ctx, p.ID)
	p.Items = items
	groups, _ := r.ListGroups(ctx, p.ID)
	p.Groups = groups
	incidents, _ := r.ListIncidents(ctx, p.ID, false)
	p.Incidents = incidents

	return &p, nil
}

func (r *StatusPageRepository) GetPageBySlug(ctx context.Context, slug string) (*domain.StatusPage, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, title, slug, description, footer_text, theme, refresh_interval,
		       is_public, is_published, show_tags, custom_css, user_id, created_at, updated_at
		FROM status_pages
		WHERE LOWER(slug) = LOWER($1)
	`
	var p domain.StatusPage
	err = pool.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Description, &p.FooterText, &p.Theme,
		&p.RefreshInterval, &p.IsPublic, &p.IsPublished, &p.ShowTags,
		&p.CustomCSS, &p.UserID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	items, _ := r.ListItems(ctx, p.ID)
	p.Items = items
	groups, _ := r.ListGroups(ctx, p.ID)
	p.Groups = groups
	incidents, _ := r.ListIncidents(ctx, p.ID, false)
	p.Incidents = incidents

	return &p, nil
}

func (r *StatusPageRepository) CreatePage(ctx context.Context, p *domain.StatusPage) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.RefreshInterval <= 0 {
		p.RefreshInterval = 60
	}
	if p.Theme == "" {
		p.Theme = "auto"
	}

	query := `
		INSERT INTO status_pages (
			id, title, slug, description, footer_text, theme, refresh_interval,
			is_public, is_published, show_tags, custom_css, user_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err = pool.Exec(ctx, query,
		p.ID, p.Title, p.Slug, p.Description, p.FooterText, p.Theme, p.RefreshInterval,
		p.IsPublic, p.IsPublished, p.ShowTags, p.CustomCSS, p.UserID, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *StatusPageRepository) UpdatePage(ctx context.Context, p *domain.StatusPage) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	p.UpdatedAt = time.Now().UTC()
	if p.RefreshInterval <= 0 {
		p.RefreshInterval = 60
	}

	query := `
		UPDATE status_pages
		SET title = $2, slug = $3, description = $4, footer_text = $5,
		    theme = $6, refresh_interval = $7, is_public = $8, is_published = $9,
		    show_tags = $10, custom_css = $11, updated_at = $12
		WHERE id = $1
	`
	res, err := pool.Exec(ctx, query,
		p.ID, p.Title, p.Slug, p.Description, p.FooterText,
		p.Theme, p.RefreshInterval, p.IsPublic, p.IsPublished,
		p.ShowTags, p.CustomCSS, p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("status page not found")
	}
	return nil
}

func (r *StatusPageRepository) DeletePage(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM status_pages WHERE id = $1`
	_, err = pool.Exec(ctx, query, id)
	return err
}

// -----------------------------------------------------------------------------
// Groups CRUD
// -----------------------------------------------------------------------------

func (r *StatusPageRepository) ListGroups(ctx context.Context, pageID string) ([]domain.StatusPageGroup, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, page_id, name, sort_order, created_at
		FROM status_page_groups
		WHERE page_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := pool.Query(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []domain.StatusPageGroup
	for rows.Next() {
		var g domain.StatusPageGroup
		if err := rows.Scan(&g.ID, &g.PageID, &g.Name, &g.SortOrder, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	return groups, nil
}

func (r *StatusPageRepository) SaveGroup(ctx context.Context, g *domain.StatusPageGroup) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO status_page_groups (id, page_id, name, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			sort_order = EXCLUDED.sort_order
	`
	_, err = pool.Exec(ctx, query, g.ID, g.PageID, g.Name, g.SortOrder, g.CreatedAt)
	return err
}

func (r *StatusPageRepository) DeleteGroup(ctx context.Context, groupID string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM status_page_groups WHERE id = $1`
	_, err = pool.Exec(ctx, query, groupID)
	return err
}

// -----------------------------------------------------------------------------
// Items CRUD
// -----------------------------------------------------------------------------

func (r *StatusPageRepository) ListItems(ctx context.Context, pageID string) ([]domain.StatusPageItem, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, page_id, group_id, name, source_type, source_id, source_config, description, sort_order, created_at
		FROM status_page_items
		WHERE page_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := pool.Query(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.StatusPageItem
	for rows.Next() {
		var item domain.StatusPageItem
		var configBytes []byte

		if err := rows.Scan(
			&item.ID, &item.PageID, &item.GroupID, &item.Name, &item.SourceType,
			&item.SourceID, &configBytes, &item.Description, &item.SortOrder, &item.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(configBytes) > 0 {
			_ = json.Unmarshal(configBytes, &item.SourceConfig)
		}
		if item.SourceConfig == nil {
			item.SourceConfig = make(map[string]interface{})
		}

		items = append(items, item)
	}

	return items, nil
}

func (r *StatusPageRepository) SaveItem(ctx context.Context, item *domain.StatusPageItem) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.SourceConfig == nil {
		item.SourceConfig = make(map[string]interface{})
	}
	configBytes, err := json.Marshal(item.SourceConfig)
	if err != nil {
		configBytes = []byte("{}")
	}

	query := `
		INSERT INTO status_page_items (
			id, page_id, group_id, name, source_type, source_id, source_config, description, sort_order, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			group_id = EXCLUDED.group_id,
			name = EXCLUDED.name,
			source_type = EXCLUDED.source_type,
			source_id = EXCLUDED.source_id,
			source_config = EXCLUDED.source_config,
			description = EXCLUDED.description,
			sort_order = EXCLUDED.sort_order
	`
	_, err = pool.Exec(ctx, query,
		item.ID, item.PageID, item.GroupID, item.Name, item.SourceType,
		item.SourceID, configBytes, item.Description, item.SortOrder, item.CreatedAt,
	)
	return err
}

func (r *StatusPageRepository) DeleteItem(ctx context.Context, itemID string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM status_page_items WHERE id = $1`
	_, err = pool.Exec(ctx, query, itemID)
	return err
}

// -----------------------------------------------------------------------------
// Incidents CRUD
// -----------------------------------------------------------------------------

func (r *StatusPageRepository) ListIncidents(ctx context.Context, pageID string, onlyActive bool) ([]domain.StatusPageIncident, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, page_id, title, status, severity, message, is_active, created_at, updated_at
		FROM status_page_incidents
		WHERE page_id = $1
	`
	if onlyActive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := pool.Query(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []domain.StatusPageIncident
	for rows.Next() {
		var inc domain.StatusPageIncident
		if err := rows.Scan(
			&inc.ID, &inc.PageID, &inc.Title, &inc.Status, &inc.Severity,
			&inc.Message, &inc.IsActive, &inc.CreatedAt, &inc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		incidents = append(incidents, inc)
	}

	return incidents, nil
}

func (r *StatusPageRepository) SaveIncident(ctx context.Context, inc *domain.StatusPageIncident) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = now
	}
	inc.UpdatedAt = now

	query := `
		INSERT INTO status_page_incidents (
			id, page_id, title, status, severity, message, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			status = EXCLUDED.status,
			severity = EXCLUDED.severity,
			message = EXCLUDED.message,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`
	_, err = pool.Exec(ctx, query,
		inc.ID, inc.PageID, inc.Title, inc.Status, inc.Severity,
		inc.Message, inc.IsActive, inc.CreatedAt, inc.UpdatedAt,
	)
	return err
}

func (r *StatusPageRepository) DeleteIncident(ctx context.Context, incidentID string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `DELETE FROM status_page_incidents WHERE id = $1`
	_, err = pool.Exec(ctx, query, incidentID)
	return err
}
