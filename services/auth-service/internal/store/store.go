package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// UserRecord is the auth-service projection returned by DB operations.
type UserRecord struct {
	ID             string    `json:"id"`
	GitHubID       int64     `json:"github_id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	AvatarURL      string    `json:"avatar_url"`
	OrgID          string    `json:"org_id"`
	OrgName        string    `json:"org_name"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	TokenUpdatedAt time.Time `json:"token_updated_at"`
}

// UpsertUserParams contains all values needed to update a user after OAuth.
type UpsertUserParams struct {
	GitHubID        int64
	Username        string
	Email           string
	AvatarURL       string
	TokenNonce      []byte
	TokenCiphertext []byte
	TokenUpdatedAt  time.Time
}

// Store wraps PostgreSQL access for auth-service.
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

// Close closes the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// UpsertUserWithOrg creates or updates a user, ensures a default org, and returns membership data.
func (s *Store) UpsertUserWithOrg(ctx context.Context, params UpsertUserParams) (UserRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UserRecord{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var userRecord UserRecord
	query := `
		INSERT INTO users (github_id, username, email, avatar_url, github_token_nonce, github_token_ciphertext, github_token_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (github_id)
		DO UPDATE SET
			username = EXCLUDED.username,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			github_token_nonce = EXCLUDED.github_token_nonce,
			github_token_ciphertext = EXCLUDED.github_token_ciphertext,
			github_token_updated_at = EXCLUDED.github_token_updated_at
		RETURNING id, github_id, username, email, avatar_url, created_at, COALESCE(github_token_updated_at, now())`
	if err := tx.QueryRowContext(ctx, query, params.GitHubID, params.Username, params.Email, params.AvatarURL, params.TokenNonce, params.TokenCiphertext, params.TokenUpdatedAt).
		Scan(&userRecord.ID, &userRecord.GitHubID, &userRecord.Username, &userRecord.Email, &userRecord.AvatarURL, &userRecord.CreatedAt, &userRecord.TokenUpdatedAt); err != nil {
		return UserRecord{}, fmt.Errorf("upsert user: %w", err)
	}

	orgID, orgName, role, err := ensureDefaultOrg(ctx, tx, userRecord.ID, userRecord.Username)
	if err != nil {
		return UserRecord{}, fmt.Errorf("ensure default org: %w", err)
	}
	userRecord.OrgID = orgID
	userRecord.OrgName = orgName
	userRecord.Role = role

	if err := tx.Commit(); err != nil {
		return UserRecord{}, fmt.Errorf("commit transaction: %w", err)
	}

	return userRecord, nil
}

// GetUserByID loads the user plus current org membership.
func (s *Store) GetUserByID(ctx context.Context, userID string) (UserRecord, error) {
	const query = `
		SELECT u.id, u.github_id, u.username, u.email, COALESCE(u.avatar_url, ''), u.created_at,
		       COALESCE(u.github_token_updated_at, u.created_at),
		       o.id, o.name, om.role
		FROM users u
		JOIN organizations o ON o.owner_id = u.id
		JOIN org_members om ON om.org_id = o.id AND om.user_id = u.id
		WHERE u.id = $1
		LIMIT 1`

	var userRecord UserRecord
	if err := s.db.QueryRowContext(ctx, query, userID).
		Scan(
			&userRecord.ID,
			&userRecord.GitHubID,
			&userRecord.Username,
			&userRecord.Email,
			&userRecord.AvatarURL,
			&userRecord.CreatedAt,
			&userRecord.TokenUpdatedAt,
			&userRecord.OrgID,
			&userRecord.OrgName,
			&userRecord.Role,
		); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserRecord{}, fmt.Errorf("load user %s: %w", userID, err)
		}
		return UserRecord{}, fmt.Errorf("query user %s: %w", userID, err)
	}

	return userRecord, nil
}

func ensureDefaultOrg(ctx context.Context, tx *sql.Tx, userID, username string) (string, string, string, error) {
	const existingQuery = `
		SELECT o.id, o.name, om.role
		FROM organizations o
		JOIN org_members om ON om.org_id = o.id AND om.user_id = $1
		WHERE o.owner_id = $1
		LIMIT 1`

	var orgID string
	var orgName string
	var role string
	if err := tx.QueryRowContext(ctx, existingQuery, userID).Scan(&orgID, &orgName, &role); err == nil {
		return orgID, orgName, role, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", "", "", fmt.Errorf("query existing org: %w", err)
	}

	orgName = strings.TrimSpace(username) + " Workspace"
	baseSlug := sanitizeSlug(username)
	for attempt := 0; attempt < 5; attempt++ {
		slug := baseSlug
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", baseSlug, attempt+1)
		}

		insertQuery := `INSERT INTO organizations (name, slug, owner_id) VALUES ($1, $2, $3) RETURNING id`
		err := tx.QueryRowContext(ctx, insertQuery, orgName, slug, userID).Scan(&orgID)
		if err != nil {
			if strings.Contains(err.Error(), "organizations_slug_key") {
				continue
			}
			return "", "", "", fmt.Errorf("insert organization: %w", err)
		}

		role = "owner"
		membershipQuery := `INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`
		if _, err := tx.ExecContext(ctx, membershipQuery, orgID, userID, role); err != nil {
			return "", "", "", fmt.Errorf("insert org membership: %w", err)
		}

		return orgID, orgName, role, nil
	}

	return "", "", "", errors.New("could not generate unique organization slug")
}

func sanitizeSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = slugSanitizer.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "helmix-workspace"
	}
	return slug
}
