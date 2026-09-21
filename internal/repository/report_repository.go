package repository

import (
	"context"
	"encoding/json"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type ReportRepository struct{}

func NewReportRepository() *ReportRepository {
	return &ReportRepository{}
}

// -----------------------------------------------------------------------------
// Visual Reports CRUD
// -----------------------------------------------------------------------------

func (r *ReportRepository) ListReports(ctx context.Context) ([]domain.VisualReport, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, mode, page_orientation, header_config, user_id, created_at, updated_at
		FROM visual_reports
		ORDER BY updated_at DESC, created_at DESC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []domain.VisualReport
	for rows.Next() {
		var rep domain.VisualReport
		var headerJSON []byte

		if err := rows.Scan(
			&rep.ID, &rep.Name, &rep.Description, &rep.Mode, &rep.PageOrientation,
			&headerJSON, &rep.UserID, &rep.CreatedAt, &rep.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if len(headerJSON) > 0 {
			_ = json.Unmarshal(headerJSON, &rep.HeaderConfig)
		}

		// Count widgets for this report
		widgets, _ := r.ListWidgets(ctx, rep.ID)
		rep.Widgets = widgets

		reports = append(reports, rep)
	}

	if reports == nil {
		reports = []domain.VisualReport{}
	}
	return reports, nil
}

func (r *ReportRepository) GetReportByID(ctx context.Context, id string) (*domain.VisualReport, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, mode, page_orientation, header_config, user_id, created_at, updated_at
		FROM visual_reports
		WHERE id = $1
	`
	var rep domain.VisualReport
	var headerJSON []byte

	err = pool.QueryRow(ctx, query, id).Scan(
		&rep.ID, &rep.Name, &rep.Description, &rep.Mode, &rep.PageOrientation,
		&headerJSON, &rep.UserID, &rep.CreatedAt, &rep.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(headerJSON) > 0 {
		_ = json.Unmarshal(headerJSON, &rep.HeaderConfig)
	}

	widgets, err := r.ListWidgets(ctx, rep.ID)
	if err == nil {
		rep.Widgets = widgets
	}

	return &rep, nil
}

func (r *ReportRepository) CreateReport(ctx context.Context, rep *domain.VisualReport) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	headerJSON, err := json.Marshal(rep.HeaderConfig)
	if err != nil {
		headerJSON = []byte("{}")
	}

	now := time.Now()
	rep.CreatedAt = now
	rep.UpdatedAt = now

	query := `
		INSERT INTO visual_reports (id, name, description, mode, page_orientation, header_config, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = pool.Exec(ctx, query,
		rep.ID, rep.Name, rep.Description, rep.Mode, rep.PageOrientation,
		headerJSON, rep.UserID, rep.CreatedAt, rep.UpdatedAt,
	)
	return err
}

func (r *ReportRepository) UpdateReport(ctx context.Context, rep *domain.VisualReport) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	headerJSON, err := json.Marshal(rep.HeaderConfig)
	if err != nil {
		headerJSON = []byte("{}")
	}

	rep.UpdatedAt = time.Now()

	query := `
		UPDATE visual_reports
		SET name = $2, description = $3, mode = $4, page_orientation = $5, header_config = $6, updated_at = $7
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query,
		rep.ID, rep.Name, rep.Description, rep.Mode, rep.PageOrientation,
		headerJSON, rep.UpdatedAt,
	)
	return err
}

func (r *ReportRepository) DeleteReport(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `DELETE FROM visual_reports WHERE id = $1`, id)
	return err
}

// -----------------------------------------------------------------------------
// Visual Report Widgets CRUD
// -----------------------------------------------------------------------------

func (r *ReportRepository) ListWidgets(ctx context.Context, reportID string) ([]domain.VisualReportWidget, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, report_id, page_number, title, chart_type, source_type, source_config, time_range, theme, width_percent, sort_order, created_at
		FROM visual_report_widgets
		WHERE report_id = $1
		ORDER BY page_number ASC, sort_order ASC, created_at ASC
	`
	rows, err := pool.Query(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []domain.VisualReportWidget
	for rows.Next() {
		var w domain.VisualReportWidget
		var configJSON []byte

		if err := rows.Scan(
			&w.ID, &w.ReportID, &w.PageNumber, &w.Title, &w.ChartType,
			&w.SourceType, &configJSON, &w.TimeRange, &w.Theme,
			&w.WidthPercent, &w.SortOrder, &w.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(configJSON) > 0 {
			_ = json.Unmarshal(configJSON, &w.SourceConfig)
		}
		if w.SourceConfig == nil {
			w.SourceConfig = make(map[string]interface{})
		}

		widgets = append(widgets, w)
	}

	if widgets == nil {
		widgets = []domain.VisualReportWidget{}
	}
	return widgets, nil
}

func (r *ReportRepository) GetWidgetByID(ctx context.Context, id string) (*domain.VisualReportWidget, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, report_id, page_number, title, chart_type, source_type, source_config, time_range, theme, width_percent, sort_order, created_at
		FROM visual_report_widgets
		WHERE id = $1
	`
	var w domain.VisualReportWidget
	var configJSON []byte

	err = pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.ReportID, &w.PageNumber, &w.Title, &w.ChartType,
		&w.SourceType, &configJSON, &w.TimeRange, &w.Theme,
		&w.WidthPercent, &w.SortOrder, &w.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(configJSON) > 0 {
		_ = json.Unmarshal(configJSON, &w.SourceConfig)
	}
	if w.SourceConfig == nil {
		w.SourceConfig = make(map[string]interface{})
	}

	return &w, nil
}

func (r *ReportRepository) CreateWidget(ctx context.Context, w *domain.VisualReportWidget) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(w.SourceConfig)
	if err != nil {
		configJSON = []byte("{}")
	}

	w.CreatedAt = time.Now()

	query := `
		INSERT INTO visual_report_widgets (id, report_id, page_number, title, chart_type, source_type, source_config, time_range, theme, width_percent, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = pool.Exec(ctx, query,
		w.ID, w.ReportID, w.PageNumber, w.Title, w.ChartType,
		w.SourceType, configJSON, w.TimeRange, w.Theme,
		w.WidthPercent, w.SortOrder, w.CreatedAt,
	)
	if err == nil {
		_, _ = pool.Exec(ctx, `UPDATE visual_reports SET updated_at = NOW() WHERE id = $1`, w.ReportID)
	}
	return err
}

func (r *ReportRepository) UpdateWidget(ctx context.Context, w *domain.VisualReportWidget) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(w.SourceConfig)
	if err != nil {
		configJSON = []byte("{}")
	}

	query := `
		UPDATE visual_report_widgets
		SET page_number = $2, title = $3, chart_type = $4, source_type = $5, source_config = $6, time_range = $7, theme = $8, width_percent = $9, sort_order = $10
		WHERE id = $1
	`
	_, err = pool.Exec(ctx, query,
		w.ID, w.PageNumber, w.Title, w.ChartType,
		w.SourceType, configJSON, w.TimeRange, w.Theme,
		w.WidthPercent, w.SortOrder,
	)
	if err == nil {
		_, _ = pool.Exec(ctx, `UPDATE visual_reports SET updated_at = NOW() WHERE id = $1`, w.ReportID)
	}
	return err
}

func (r *ReportRepository) DeleteWidget(ctx context.Context, id string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	var reportID string
	_ = pool.QueryRow(ctx, `SELECT report_id FROM visual_report_widgets WHERE id = $1`, id).Scan(&reportID)

	_, err = pool.Exec(ctx, `DELETE FROM visual_report_widgets WHERE id = $1`, id)
	if err == nil && reportID != "" {
		_, _ = pool.Exec(ctx, `UPDATE visual_reports SET updated_at = NOW() WHERE id = $1`, reportID)
	}
	return err
}
