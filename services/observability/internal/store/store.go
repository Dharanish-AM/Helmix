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
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
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
	Severity    string
	Title       string
	Description string
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
		SELECT id, project_id, severity, title, description, status, created_at
		FROM alerts
		WHERE project_id = $1 AND status = 'open'
		ORDER BY created_at DESC`

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
		SELECT id, project_id, severity, title, description, status, created_at
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
		INSERT INTO alerts (project_id, severity, title, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, project_id, severity, title, description, status, created_at`

	alert, err := scanAlert(s.db.QueryRowContext(ctx, query, params.ProjectID, params.Severity, params.Title, nullString(params.Description, "")))
	if err != nil {
		return Alert{}, fmt.Errorf("create alert: %w", err)
	}
	return alert, nil
}

// ResolveAlertsExcept resolves open alerts not included in the active title set.
func (s *Store) ResolveAlertsExcept(ctx context.Context, projectID string, activeTitles []string) error {
	const queryNoActive = `UPDATE alerts SET status = 'resolved' WHERE project_id = $1 AND status = 'open'`
	const queryWithActive = `UPDATE alerts SET status = 'resolved' WHERE project_id = $1 AND status = 'open' AND NOT (title = ANY($2))`

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

type snapshotScanner interface{ Scan(dest ...any) error }
type alertScanner interface{ Scan(dest ...any) error }

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
	if err := scanner.Scan(&alert.ID, &alert.ProjectID, &alert.Severity, &alert.Title, &description, &alert.Status, &alert.CreatedAt); err != nil {
		return Alert{}, err
	}
	if description.Valid {
		alert.Description = description.String
	}
	return alert, nil
}

func nullString(value, fallback string) any {
	if value == "" {
		return fallback
	}
	return value
}