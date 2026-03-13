package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("deployment not found")

// Deployment is the persisted deployment record.
type Deployment struct {
	ID          string     `json:"id"`
	RepoID      string     `json:"repo_id"`
	CommitSHA   string     `json:"commit_sha"`
	Branch      string     `json:"branch"`
	Status      string     `json:"status"`
	Environment string     `json:"environment"`
	ImageTag    string     `json:"image_tag,omitempty"`
	DeployedAt  *time.Time `json:"deployed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateDeploymentParams captures the row values required to start a deployment.
type CreateDeploymentParams struct {
	RepoID      string
	CommitSHA   string
	Branch      string
	Environment string
	ImageTag    string
	Status      string
}

// Store persists deployment state in PostgreSQL.
type Store struct {
	db *sql.DB
}

// New constructs a deployment store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateDeployment inserts a new deployment row.
func (s *Store) CreateDeployment(ctx context.Context, params CreateDeploymentParams) (Deployment, error) {
	const query = `
		INSERT INTO deployments (repo_id, commit_sha, branch, status, environment, image_tag)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at`

	row := s.db.QueryRowContext(ctx, query,
		params.RepoID,
		params.CommitSHA,
		params.Branch,
		params.Status,
		params.Environment,
		nullIfEmpty(params.ImageTag),
	)

	deployment, err := scanDeployment(row)
	if err != nil {
		return Deployment{}, fmt.Errorf("insert deployment: %w", err)
	}
	return deployment, nil
}

// GetDeployment loads a deployment by id.
func (s *Store) GetDeployment(ctx context.Context, deploymentID string) (Deployment, error) {
	const query = `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE id = $1`

	deployment, err := scanDeployment(s.db.QueryRowContext(ctx, query, deploymentID))
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	if err != nil {
		return Deployment{}, fmt.Errorf("get deployment %s: %w", deploymentID, err)
	}
	return deployment, nil
}

// ListDeploymentsByProject returns recent deployments associated with a project.
func (s *Store) ListDeploymentsByProject(ctx context.Context, projectID string, limit int) ([]Deployment, error) {
	if limit <= 0 {
		limit = 10
	}
	const query = `
		SELECT d.id, d.repo_id, d.commit_sha, d.branch, d.status, d.environment, d.image_tag, d.deployed_at, d.created_at
		FROM deployments d
		JOIN repos r ON r.id = d.repo_id
		WHERE r.project_id = $1
		ORDER BY COALESCE(d.deployed_at, d.created_at) DESC, d.created_at DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list deployments for project %s: %w", projectID, err)
	}
	defer rows.Close()

	deployments := make([]Deployment, 0, limit)
	for rows.Next() {
		deployment, scanErr := scanDeployment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan deployment row: %w", scanErr)
		}
		deployments = append(deployments, deployment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deployment rows: %w", err)
	}
	return deployments, nil
}

// LookupProjectID returns the project id for the given repo.
func (s *Store) LookupProjectID(ctx context.Context, repoID string) (string, error) {
	const query = `SELECT project_id FROM repos WHERE id = $1`

	var projectID string
	if err := s.db.QueryRowContext(ctx, query, repoID).Scan(&projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("lookup project for repo %s: %w", repoID, err)
	}
	return projectID, nil
}

// FindActiveDeployment returns the currently live deployment for a repo/environment.
func (s *Store) FindActiveDeployment(ctx context.Context, repoID, environment string) (*Deployment, error) {
	const query = `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE repo_id = $1 AND environment = $2 AND status = 'live'
		ORDER BY COALESCE(deployed_at, created_at) DESC, created_at DESC
		LIMIT 1`

	deployment, err := scanDeployment(s.db.QueryRowContext(ctx, query, repoID, environment))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find active deployment: %w", err)
	}
	return &deployment, nil
}

// FindPreviousDeployment returns the latest non-current live or superseded deployment for rollback context.
func (s *Store) FindPreviousDeployment(ctx context.Context, repoID, environment, currentDeploymentID string) (*Deployment, error) {
	const query = `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE repo_id = $1
		  AND environment = $2
		  AND id <> $3
		  AND status IN ('live', 'superseded')
		ORDER BY COALESCE(deployed_at, created_at) DESC, created_at DESC
		LIMIT 1`

	deployment, err := scanDeployment(s.db.QueryRowContext(ctx, query, repoID, environment, currentDeploymentID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find previous deployment: %w", err)
	}
	return &deployment, nil
}

// MarkDeploymentFailed records a failed deployment attempt.
func (s *Store) MarkDeploymentFailed(ctx context.Context, deploymentID string) error {
	return s.updateStatus(ctx, deploymentID, "failed", nil)
}

// PromoteDeployment marks the target deployment live and supersedes any previous live deployment.
func (s *Store) PromoteDeployment(ctx context.Context, deploymentID string, deployedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin promote transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := scanDeployment(tx.QueryRowContext(ctx, `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE id = $1`, deploymentID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load deployment for promote: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'superseded'
		WHERE repo_id = $1 AND environment = $2 AND status = 'live' AND id <> $3`, current.RepoID, current.Environment, current.ID); err != nil {
		return fmt.Errorf("supersede previous live deployment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'live', deployed_at = $2
		WHERE id = $1`, deploymentID, deployedAt.UTC()); err != nil {
		return fmt.Errorf("mark deployment live: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit promote transaction: %w", err)
	}

	return nil
}

// RollbackDeployment marks the target deployment rolled back and reactivates the previous deployment.
func (s *Store) RollbackDeployment(ctx context.Context, deploymentID string, rolledBackAt time.Time) (Deployment, Deployment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("begin rollback transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := scanDeployment(tx.QueryRowContext(ctx, `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE id = $1`, deploymentID))
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, Deployment{}, ErrNotFound
	}
	if err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("load current deployment for rollback: %w", err)
	}

	previous, err := scanDeployment(tx.QueryRowContext(ctx, `
		SELECT id, repo_id, commit_sha, branch, status, environment, image_tag, deployed_at, created_at
		FROM deployments
		WHERE repo_id = $1
		  AND environment = $2
		  AND id <> $3
		  AND status IN ('live', 'superseded')
		ORDER BY COALESCE(deployed_at, created_at) DESC, created_at DESC
		LIMIT 1`, current.RepoID, current.Environment, current.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, Deployment{}, ErrNotFound
	}
	if err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("load previous deployment for rollback: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'rolled_back'
		WHERE id = $1`, current.ID); err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("mark current deployment rolled_back: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'live', deployed_at = $2
		WHERE id = $1`, previous.ID, rolledBackAt.UTC()); err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("reactivate previous deployment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Deployment{}, Deployment{}, fmt.Errorf("commit rollback transaction: %w", err)
	}

	current.Status = "rolled_back"
	previous.Status = "live"
	rolledBackAtUTC := rolledBackAt.UTC()
	previous.DeployedAt = &rolledBackAtUTC
	return current, previous, nil
}

func (s *Store) updateStatus(ctx context.Context, deploymentID, status string, deployedAt *time.Time) error {
	var err error
	if deployedAt == nil {
		_, err = s.db.ExecContext(ctx, `UPDATE deployments SET status = $2 WHERE id = $1`, deploymentID, status)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE deployments SET status = $2, deployed_at = $3 WHERE id = $1`, deploymentID, status, deployedAt.UTC())
	}
	if err != nil {
		return fmt.Errorf("update deployment %s to %s: %w", deploymentID, status, err)
	}
	return nil
}

type deploymentScanner interface {
	Scan(dest ...any) error
}

func scanDeployment(scanner deploymentScanner) (Deployment, error) {
	var deployment Deployment
	var imageTag sql.NullString
	var deployedAt sql.NullTime
	if err := scanner.Scan(
		&deployment.ID,
		&deployment.RepoID,
		&deployment.CommitSHA,
		&deployment.Branch,
		&deployment.Status,
		&deployment.Environment,
		&imageTag,
		&deployedAt,
		&deployment.CreatedAt,
	); err != nil {
		return Deployment{}, err
	}
	if imageTag.Valid {
		deployment.ImageTag = imageTag.String
	}
	if deployedAt.Valid {
		deployedAtUTC := deployedAt.Time.UTC()
		deployment.DeployedAt = &deployedAtUTC
	}
	return deployment, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}