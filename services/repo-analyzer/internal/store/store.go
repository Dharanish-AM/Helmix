package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/your-org/helmix/services/repo-analyzer/internal/analyzer"
)

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// RepoMetadata contains repo-to-project/org linkage required for event publication.
type RepoMetadata struct {
	RepoID        string
	ProjectID     string
	OrgID         string
	GitHubRepo    string
	DefaultBranch string
}

// ConnectedRepo is the projection used by dashboard repository picker flows.
type ConnectedRepo struct {
	RepoID        string         `json:"repo_id"`
	ProjectID     string         `json:"project_id"`
	ProjectName   string         `json:"project_name"`
	GitHubRepo    string         `json:"github_repo"`
	DefaultBranch string         `json:"default_branch"`
	DetectedStack map[string]any `json:"detected_stack,omitempty"`
	ConnectedAt   time.Time      `json:"connected_at"`
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

// ListConnectedRepos returns repositories linked to the org, optionally filtered by query.
func (s *Store) ListConnectedRepos(ctx context.Context, orgID, query string, limit int) ([]ConnectedRepo, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, errors.New("org id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	searchPattern := "%" + strings.TrimSpace(strings.ToLower(query)) + "%"
	const sqlQuery = `
		SELECT r.id, p.id, p.name, r.github_repo, r.default_branch, COALESCE(r.detected_stack, '{}'::jsonb), r.connected_at
		FROM repos r
		JOIN projects p ON p.id = r.project_id
		WHERE p.org_id = $1
		  AND ($2 = '%%' OR LOWER(r.github_repo) LIKE $2)
		ORDER BY r.connected_at DESC
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, sqlQuery, orgID, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("list connected repos: %w", err)
	}
	defer rows.Close()

	repos := make([]ConnectedRepo, 0, limit)
	for rows.Next() {
		var item ConnectedRepo
		var detectedStackRaw []byte
		if err := rows.Scan(
			&item.RepoID,
			&item.ProjectID,
			&item.ProjectName,
			&item.GitHubRepo,
			&item.DefaultBranch,
			&detectedStackRaw,
			&item.ConnectedAt,
		); err != nil {
			return nil, fmt.Errorf("scan connected repo: %w", err)
		}
		if len(detectedStackRaw) > 0 {
			_ = json.Unmarshal(detectedStackRaw, &item.DetectedStack)
		}
		repos = append(repos, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connected repos: %w", err)
	}

	return repos, nil
}

// CreateConnectedRepo creates or reuses project/repo records for a GitHub repository.
func (s *Store) CreateConnectedRepo(ctx context.Context, orgID, githubRepo, defaultBranch string) (ConnectedRepo, error) {
	repoPath := strings.TrimSpace(strings.ToLower(githubRepo))
	if repoPath == "" || !strings.Contains(repoPath, "/") {
		return ConnectedRepo{}, errors.New("github_repo must be in owner/name format")
	}
	if strings.TrimSpace(orgID) == "" {
		return ConnectedRepo{}, errors.New("org id is required")
	}
	branch := strings.TrimSpace(defaultBranch)
	if branch == "" {
		branch = "main"
	}

	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return ConnectedRepo{}, errors.New("github_repo must be in owner/name format")
	}
	repoName := parts[len(parts)-1]
	projectSlug := sanitizeSlug(repoName)
	projectName := strings.ToUpper(repoName[:1]) + repoName[1:] + " Project"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConnectedRepo{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var projectID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO projects (org_id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id, slug)
		DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, orgID, projectName, projectSlug).Scan(&projectID); err != nil {
		return ConnectedRepo{}, fmt.Errorf("upsert project: %w", err)
	}

	var existing ConnectedRepo
	var existingDetectedRaw []byte
	err = tx.QueryRowContext(ctx, `
		SELECT r.id, p.id, p.name, r.github_repo, r.default_branch, COALESCE(r.detected_stack, '{}'::jsonb), r.connected_at
		FROM repos r
		JOIN projects p ON p.id = r.project_id
		WHERE r.project_id = $1 AND r.github_repo = $2
		LIMIT 1`, projectID, repoPath).Scan(
		&existing.RepoID,
		&existing.ProjectID,
		&existing.ProjectName,
		&existing.GitHubRepo,
		&existing.DefaultBranch,
		&existingDetectedRaw,
		&existing.ConnectedAt,
	)
	if err == nil {
		if len(existingDetectedRaw) > 0 {
			_ = json.Unmarshal(existingDetectedRaw, &existing.DetectedStack)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return ConnectedRepo{}, fmt.Errorf("commit existing repo transaction: %w", commitErr)
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ConnectedRepo{}, fmt.Errorf("query existing repo: %w", err)
	}

	var created ConnectedRepo
	var createdDetectedRaw []byte
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO repos (project_id, github_repo, default_branch)
		VALUES ($1, $2, $3)
		RETURNING id, project_id, $4, github_repo, default_branch, COALESCE(detected_stack, '{}'::jsonb), connected_at`,
		projectID, repoPath, branch, projectName,
	).Scan(
		&created.RepoID,
		&created.ProjectID,
		&created.ProjectName,
		&created.GitHubRepo,
		&created.DefaultBranch,
		&createdDetectedRaw,
		&created.ConnectedAt,
	); err != nil {
		return ConnectedRepo{}, fmt.Errorf("insert repo: %w", err)
	}
	if len(createdDetectedRaw) > 0 {
		_ = json.Unmarshal(createdDetectedRaw, &created.DetectedStack)
	}

	if err := tx.Commit(); err != nil {
		return ConnectedRepo{}, fmt.Errorf("commit create repo transaction: %w", err)
	}
	return created, nil
}

func sanitizeSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = slugSanitizer.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "project"
	}
	return slug
}
