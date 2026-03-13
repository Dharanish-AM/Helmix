package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/your-org/helmix/services/repo-analyzer/internal/analyzer"
)

// RepoMetadata contains repo-to-project/org linkage required for event publication.
type RepoMetadata struct {
	RepoID        string
	ProjectID     string
	OrgID         string
	GitHubRepo    string
	DefaultBranch string
}

// Store wraps PostgreSQL access for repo-analyzer.
type Store struct {
	db *sql.DB
}

// Open connects to PostgreSQL and validates connectivity.
func Open(databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// GetRepoMetadata loads the org and project context for a repo.
func (s *Store) GetRepoMetadata(ctx context.Context, repoID string) (RepoMetadata, error) {
	const query = `
		SELECT r.id, p.id, p.org_id, r.github_repo, r.default_branch
		FROM repos r
		JOIN projects p ON p.id = r.project_id
		WHERE r.id = $1`

	var metadata RepoMetadata
	if err := s.db.QueryRowContext(ctx, query, repoID).
		Scan(&metadata.RepoID, &metadata.ProjectID, &metadata.OrgID, &metadata.GitHubRepo, &metadata.DefaultBranch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoMetadata{}, fmt.Errorf("repo %s not found: %w", repoID, err)
		}
		return RepoMetadata{}, fmt.Errorf("query repo metadata %s: %w", repoID, err)
	}
	return metadata, nil
}

// UpdateDetectedStack persists the detected stack JSONB to the repos table.
func (s *Store) UpdateDetectedStack(ctx context.Context, repoID string, result analyzer.Result) error {
	payload, err := json.Marshal(map[string]any{
		"runtime":       result.Stack.Runtime,
		"framework":     result.Stack.Framework,
		"database":      result.Stack.Database,
		"containerized": result.Stack.Containerized,
		"has_tests":     result.Stack.HasTests,
		"port":          result.Stack.Port,
		"build_command": result.Stack.BuildCommand,
		"test_command":  result.Stack.TestCommand,
		"confidence":    result.Confidence,
		"fallback_used": result.FallbackUsed,
	})
	if err != nil {
		return fmt.Errorf("marshal detected stack: %w", err)
	}

	commandTag, err := s.db.ExecContext(ctx, `UPDATE repos SET detected_stack = $2 WHERE id = $1`, repoID, payload)
	if err != nil {
		return fmt.Errorf("update detected stack for repo %s: %w", repoID, err)
	}
	rowsAffected, err := commandTag.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected for repo %s: %w", repoID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("repo %s not found for detected_stack update", repoID)
	}
	return nil
}
