package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("record not found")

// MetricSnapshot is the persisted metric row.
type MetricSnapshot struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	CapturedAt    time.Time `json:"captured_at"`
	CPUPct        float64   `json:"cpu_pct"`
	MemoryPct     float64   `json:"memory_pct"`
	ReqPerSec     float64   `json:"req_per_sec"`
	ErrorRatePct  float64   `json:"error_rate_pct"`
	P99LatencyMS  float64   `json:"p99_latency_ms"`
	PodCount      int       `json:"pod_count"`
	ReadyPodCount int       `json:"ready_pod_count"`
	PodRestarts   int       `json:"pod_restarts"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}

// Alert is the persisted alert row.
type Alert struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Rule        string     `json:"rule,omitempty"`
	Severity    string     `json:"severity"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Metric      string     `json:"metric,omitempty"`
	Value       float64    `json:"value,omitempty"`
	Threshold   float64    `json:"threshold,omitempty"`
	Status      string     `json:"status"`
	FiredAt     time.Time  `json:"fired_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TelemetrySource is the persisted configuration for scraping real project metrics.
type TelemetrySource struct {
	ProjectID             string     `json:"project_id"`
	SourceType            string     `json:"source_type"`
	MetricsURL            string     `json:"metrics_url"`
	ScrapeIntervalSeconds int        `json:"scrape_interval_seconds"`
	Enabled               bool       `json:"enabled"`
	LastScrapedAt         *time.Time `json:"last_scraped_at,omitempty"`
	LastError             string     `json:"last_error,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// UpsertTelemetrySourceParams contains telemetry source upsert fields.
type UpsertTelemetrySourceParams struct {
	ProjectID             string
	SourceType            string
	MetricsURL            string
	ScrapeIntervalSeconds int
	Enabled               bool
}

// CreateSnapshotParams contains snapshot insert fields.
type CreateSnapshotParams struct {
	ProjectID     string
	CapturedAt    time.Time
	CPUPct        float64
	MemoryPct     float64
	ReqPerSec     float64
	ErrorRatePct  float64
	P99LatencyMS  float64
	PodCount      int
	ReadyPodCount int
	PodRestarts   int
	Source        string
}

// CreateAlertParams contains alert insert fields.
type CreateAlertParams struct {
	ProjectID   string
	Rule        string
	Severity    string
	Title       string
	Description string
	Metric      string
	Value       float64
	Threshold   float64
	FiredAt     time.Time
}

// Store persists observability data.
type Store struct {
	db *sql.DB
}

// New constructs a store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateSnapshot inserts a metric snapshot.
func (s *Store) CreateSnapshot(ctx context.Context, params CreateSnapshotParams) (MetricSnapshot, error) {
	const query = `
		INSERT INTO metric_snapshots (
			project_id, captured_at, cpu_pct, memory_pct, req_per_sec,
			error_rate_pct, p99_latency_ms, pod_count, ready_pod_count,
			pod_restarts, source
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, project_id, captured_at, cpu_pct, memory_pct, req_per_sec,
			error_rate_pct, p99_latency_ms, pod_count, ready_pod_count, pod_restarts,
			source, created_at`

	row := s.db.QueryRowContext(ctx, query,
		params.ProjectID,
		params.CapturedAt.UTC(),
		params.CPUPct,
		params.MemoryPct,
		params.ReqPerSec,
		params.ErrorRatePct,
		params.P99LatencyMS,
		params.PodCount,
		params.ReadyPodCount,
		params.PodRestarts,
		nullString(params.Source, "synthetic"),
	)
	snapshot, err := scanSnapshot(row)
	if err != nil {
		return MetricSnapshot{}, fmt.Errorf("insert metric snapshot: %w", err)
	}
	return snapshot, nil
}

// ListSnapshots returns recent snapshots for a project.
func (s *Store) ListSnapshots(ctx context.Context, projectID string, since time.Time) ([]MetricSnapshot, error) {
	const query = `
		SELECT id, project_id, captured_at, cpu_pct, memory_pct, req_per_sec,
			error_rate_pct, p99_latency_ms, pod_count, ready_pod_count, pod_restarts,
			source, created_at
		FROM metric_snapshots
		WHERE project_id = $1 AND captured_at >= $2
		ORDER BY captured_at ASC`

	rows, err := s.db.QueryContext(ctx, query, projectID, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []MetricSnapshot
	for rows.Next() {
		snapshot, scanErr := scanSnapshot(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan snapshot: %w", scanErr)
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshots: %w", err)
	}
	return snapshots, nil
}

// GetLatestSnapshot returns the newest snapshot for a project.
func (s *Store) GetLatestSnapshot(ctx context.Context, projectID string) (MetricSnapshot, error) {
	const query = `
		SELECT id, project_id, captured_at, cpu_pct, memory_pct, req_per_sec,
			error_rate_pct, p99_latency_ms, pod_count, ready_pod_count, pod_restarts,
			source, created_at
		FROM metric_snapshots
		WHERE project_id = $1
		ORDER BY captured_at DESC
		LIMIT 1`

	snapshot, err := scanSnapshot(s.db.QueryRowContext(ctx, query, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return MetricSnapshot{}, ErrNotFound
	}
	if err != nil {
		return MetricSnapshot{}, fmt.Errorf("get latest snapshot: %w", err)
	}
	return snapshot, nil
}

// ListOpenAlerts returns all open alerts for a project.
func (s *Store) ListOpenAlerts(ctx context.Context, projectID string) ([]Alert, error) {
	const query = `
		SELECT id, project_id, rule, severity, title, description, metric, value, threshold, status, fired_at, resolved_at, created_at
		FROM alerts
		WHERE project_id = $1 AND status = 'open'
		ORDER BY fired_at DESC`

	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list open alerts: %w", err)
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		alert, scanErr := scanAlert(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan alert: %w", scanErr)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts: %w", err)
	}
	return alerts, nil
}

// FindOpenAlertByTitle looks up an open alert for deduplication.
func (s *Store) FindOpenAlertByTitle(ctx context.Context, projectID, title string) (*Alert, error) {
	const query = `
		SELECT id, project_id, rule, severity, title, description, metric, value, threshold, status, fired_at, resolved_at, created_at
		FROM alerts
		WHERE project_id = $1 AND title = $2 AND status = 'open'
		LIMIT 1`

	alert, err := scanAlert(s.db.QueryRowContext(ctx, query, projectID, title))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find open alert by title: %w", err)
	}
	return &alert, nil
}

// CreateAlert inserts a new alert.
func (s *Store) CreateAlert(ctx context.Context, params CreateAlertParams) (Alert, error) {
	const query = `
		INSERT INTO alerts (project_id, rule, severity, title, description, metric, value, threshold, fired_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, project_id, rule, severity, title, description, metric, value, threshold, status, fired_at, resolved_at, created_at`

	alert, err := scanAlert(s.db.QueryRowContext(ctx, query, params.ProjectID, nullString(params.Rule, ""), params.Severity, params.Title, nullString(params.Description, ""), nullString(params.Metric, ""), params.Value, params.Threshold, params.FiredAt.UTC()))
	if err != nil {
		return Alert{}, fmt.Errorf("create alert: %w", err)
	}
	return alert, nil
}

// ResolveAlertsExcept resolves open alerts not included in the active title set.
func (s *Store) ResolveAlertsExcept(ctx context.Context, projectID string, activeTitles []string) error {
	const queryNoActive = `UPDATE alerts SET status = 'resolved', resolved_at = now() WHERE project_id = $1 AND status = 'open'`
	const queryWithActive = `UPDATE alerts SET status = 'resolved', resolved_at = now() WHERE project_id = $1 AND status = 'open' AND NOT (title = ANY($2))`

	if len(activeTitles) == 0 {
		if _, err := s.db.ExecContext(ctx, queryNoActive, projectID); err != nil {
			return fmt.Errorf("resolve alerts: %w", err)
		}
		return nil
	}

	if _, err := s.db.ExecContext(ctx, queryWithActive, projectID, activeTitles); err != nil {
		return fmt.Errorf("resolve alerts except active: %w", err)
	}
	return nil
}

// GetProjectContext returns the org identifier for a project.
func (s *Store) GetProjectContext(ctx context.Context, projectID string) (string, error) {
	const query = `SELECT org_id FROM projects WHERE id = $1`
	var orgID string
	if err := s.db.QueryRowContext(ctx, query, projectID).Scan(&orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get project context: %w", err)
	}
	return orgID, nil
}

// GetTelemetrySource returns the configured telemetry source for a project.
func (s *Store) GetTelemetrySource(ctx context.Context, projectID string) (TelemetrySource, error) {
	const query = `
		SELECT project_id, source_type, metrics_url, scrape_interval_seconds, enabled, last_scraped_at, last_error, created_at, updated_at
		FROM project_telemetry_sources
		WHERE project_id = $1`

	source, err := scanTelemetrySource(s.db.QueryRowContext(ctx, query, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return TelemetrySource{}, ErrNotFound
	}
	if err != nil {
		return TelemetrySource{}, fmt.Errorf("get telemetry source: %w", err)
	}
	return source, nil
}

// UpsertTelemetrySource creates or updates the configured telemetry source for a project.
func (s *Store) UpsertTelemetrySource(ctx context.Context, params UpsertTelemetrySourceParams) (TelemetrySource, error) {
	const query = `
		INSERT INTO project_telemetry_sources (
			project_id, source_type, metrics_url, scrape_interval_seconds, enabled, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (project_id)
		DO UPDATE SET
			source_type = EXCLUDED.source_type,
			metrics_url = EXCLUDED.metrics_url,
			scrape_interval_seconds = EXCLUDED.scrape_interval_seconds,
			enabled = EXCLUDED.enabled,
			updated_at = now()
		RETURNING project_id, source_type, metrics_url, scrape_interval_seconds, enabled, last_scraped_at, last_error, created_at, updated_at`

	source, err := scanTelemetrySource(s.db.QueryRowContext(ctx, query,
		params.ProjectID,
		nullString(params.SourceType, "helmix-json"),
		params.MetricsURL,
		params.ScrapeIntervalSeconds,
		params.Enabled,
	))
	if err != nil {
		return TelemetrySource{}, fmt.Errorf("upsert telemetry source: %w", err)
	}
	return source, nil
}

// RecordTelemetryScrape updates the scrape status for a project telemetry source.
func (s *Store) RecordTelemetryScrape(ctx context.Context, projectID string, scrapedAt time.Time, scrapeErr error) error {
	const query = `
		UPDATE project_telemetry_sources
		SET last_scraped_at = $2,
			last_error = $3,
			updated_at = now()
		WHERE project_id = $1`

	lastError := ""
	if scrapeErr != nil {
		lastError = scrapeErr.Error()
	}
	if _, err := s.db.ExecContext(ctx, query, projectID, scrapedAt.UTC(), nullIfEmpty(lastError)); err != nil {
		return fmt.Errorf("record telemetry scrape: %w", err)
	}
	return nil
}

type snapshotScanner interface{ Scan(dest ...any) error }
type alertScanner interface{ Scan(dest ...any) error }
type telemetrySourceScanner interface{ Scan(dest ...any) error }

func scanSnapshot(scanner snapshotScanner) (MetricSnapshot, error) {
	var snapshot MetricSnapshot
	if err := scanner.Scan(
		&snapshot.ID,
		&snapshot.ProjectID,
		&snapshot.CapturedAt,
		&snapshot.CPUPct,
		&snapshot.MemoryPct,
		&snapshot.ReqPerSec,
		&snapshot.ErrorRatePct,
		&snapshot.P99LatencyMS,
		&snapshot.PodCount,
		&snapshot.ReadyPodCount,
		&snapshot.PodRestarts,
		&snapshot.Source,
		&snapshot.CreatedAt,
	); err != nil {
		return MetricSnapshot{}, err
	}
	return snapshot, nil
}

func scanAlert(scanner alertScanner) (Alert, error) {
	var alert Alert
	var description sql.NullString
	var rule sql.NullString
	var metric sql.NullString
	var value sql.NullFloat64
	var threshold sql.NullFloat64
	var resolvedAt sql.NullTime
	if err := scanner.Scan(&alert.ID, &alert.ProjectID, &rule, &alert.Severity, &alert.Title, &description, &metric, &value, &threshold, &alert.Status, &alert.FiredAt, &resolvedAt, &alert.CreatedAt); err != nil {
		return Alert{}, err
	}
	if rule.Valid {
		alert.Rule = rule.String
	}
	if description.Valid {
		alert.Description = description.String
	}
	if metric.Valid {
		alert.Metric = metric.String
	}
	if value.Valid {
		alert.Value = value.Float64
	}
	if threshold.Valid {
		alert.Threshold = threshold.Float64
	}
	if resolvedAt.Valid {
		resolved := resolvedAt.Time
		alert.ResolvedAt = &resolved
	}
	return alert, nil
}

func scanTelemetrySource(scanner telemetrySourceScanner) (TelemetrySource, error) {
	var source TelemetrySource
	var lastScrapedAt sql.NullTime
	var lastError sql.NullString
	if err := scanner.Scan(
		&source.ProjectID,
		&source.SourceType,
		&source.MetricsURL,
		&source.ScrapeIntervalSeconds,
		&source.Enabled,
		&lastScrapedAt,
		&lastError,
		&source.CreatedAt,
		&source.UpdatedAt,
	); err != nil {
		return TelemetrySource{}, err
	}
	if lastScrapedAt.Valid {
		scrapedAt := lastScrapedAt.Time
		source.LastScrapedAt = &scrapedAt
	}
	if lastError.Valid {
		source.LastError = lastError.String
	}
	return source, nil
}

func nullString(value, fallback string) any {
	if value == "" {
		return fallback
	}
	return value
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
