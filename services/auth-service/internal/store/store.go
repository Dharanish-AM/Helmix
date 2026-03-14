package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

// GitHubTokenRecord holds encrypted token material for a user.
type GitHubTokenRecord struct {
	Nonce      []byte
	Ciphertext []byte
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
	defer func() {
		_ = tx.Rollback()
	}()

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
		JOIN org_members om ON om.user_id = u.id
		JOIN organizations o ON o.id = om.org_id
		WHERE u.id = $1
		ORDER BY CASE WHEN o.owner_id = u.id THEN 0 ELSE 1 END, o.created_at ASC
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

// GetGitHubTokenByUserID returns encrypted GitHub token material for a user.
func (s *Store) GetGitHubTokenByUserID(ctx context.Context, userID string) (GitHubTokenRecord, error) {
	const query = `
		SELECT github_token_nonce, github_token_ciphertext
		FROM users
		WHERE id = $1`

	var tokenRecord GitHubTokenRecord
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&tokenRecord.Nonce, &tokenRecord.Ciphertext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GitHubTokenRecord{}, fmt.Errorf("load github token for user %s: %w", userID, err)
		}
		return GitHubTokenRecord{}, fmt.Errorf("query github token for user %s: %w", userID, err)
	}

	return tokenRecord, nil
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

// OrgRecord is the org projection returned by CreateOrg.
type OrgRecord struct {
	ID        string
	Name      string
	Slug      string
	OwnerID   string
	CreatedAt time.Time
}

// MemberRecord is the member projection returned by GetOrgMembers.
type MemberRecord struct {
	UserID    string
	Username  string
	Email     string
	AvatarURL string
	Role      string
}

// InviteRecord holds the invite details returned by CreateInvite.
type InviteRecord struct {
	ID        string
	OrgID     string
	Email     string
	Role      string
	Token     string
	ExpiresAt time.Time
}

// CreateOrg creates a new organization owned by the given user.
func (s *Store) CreateOrg(ctx context.Context, userID, name string) (OrgRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OrgRecord{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	base := sanitizeSlug(name)
	var orgID, slug string
	var createdAt time.Time
	for attempt := 0; attempt < 5; attempt++ {
		slug = base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", base, attempt+1)
		}
		err = tx.QueryRowContext(ctx,
			`INSERT INTO organizations (name, slug, owner_id) VALUES ($1, $2, $3) RETURNING id, created_at`,
			name, slug, userID,
		).Scan(&orgID, &createdAt)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "organizations_slug_key") {
			continue
		}
		return OrgRecord{}, fmt.Errorf("insert organization: %w", err)
	}
	if orgID == "" {
		return OrgRecord{}, errors.New("could not generate unique organization slug")
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, 'owner')`,
		orgID, userID,
	); err != nil {
		return OrgRecord{}, fmt.Errorf("insert org membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return OrgRecord{}, fmt.Errorf("commit transaction: %w", err)
	}
	return OrgRecord{ID: orgID, Name: name, Slug: slug, OwnerID: userID, CreatedAt: createdAt}, nil
}

// GetOrgMembers returns all members of the given organization ordered by role precedence.
func (s *Store) GetOrgMembers(ctx context.Context, orgID string) ([]MemberRecord, error) {
	const query = `
		SELECT u.id, u.username, u.email, COALESCE(u.avatar_url, ''), om.role
		FROM org_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY CASE om.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'developer' THEN 2 ELSE 3 END, u.username`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query org members: %w", err)
	}
	defer rows.Close()

	var members []MemberRecord
	for rows.Next() {
		var m MemberRecord
		if err := rows.Scan(&m.UserID, &m.Username, &m.Email, &m.AvatarURL, &m.Role); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// CreateInvite stores a new time-limited org invite and returns the invite record.
func (s *Store) CreateInvite(ctx context.Context, orgID, email, role, invitedBy string) (InviteRecord, error) {
	token, err := generateInviteToken()
	if err != nil {
		return InviteRecord{}, fmt.Errorf("generate invite token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(72 * time.Hour)

	var id string
	if err := s.db.QueryRowContext(ctx,
		`INSERT INTO org_invites (org_id, email, role, token, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, email, role, token, invitedBy, expiresAt,
	).Scan(&id); err != nil {
		return InviteRecord{}, fmt.Errorf("insert invite: %w", err)
	}

	return InviteRecord{ID: id, OrgID: orgID, Email: email, Role: role, Token: token, ExpiresAt: expiresAt}, nil
}

// AcceptInvite marks an invite accepted and adds the user to the org; returns the org ID.
func (s *Store) AcceptInvite(ctx context.Context, token, userID string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var inviteID, orgID, role string
	var accepted bool
	var expiresAt time.Time
	if err := tx.QueryRowContext(ctx,
		`SELECT id, org_id, role, accepted, expires_at FROM org_invites WHERE token = $1`,
		token,
	).Scan(&inviteID, &orgID, &role, &accepted, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invite not found")
		}
		return "", fmt.Errorf("lookup invite: %w", err)
	}
	if accepted {
		return "", errors.New("invite already accepted")
	}
	if time.Now().UTC().After(expiresAt) {
		return "", errors.New("invite expired")
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID, role,
	); err != nil {
		return "", fmt.Errorf("insert org membership: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE org_invites SET accepted = true WHERE id = $1`, inviteID,
	); err != nil {
		return "", fmt.Errorf("mark invite accepted: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}
	return orgID, nil
}

// UpdateMemberRole changes the role of a member within an org.
func (s *Store) UpdateMemberRole(ctx context.Context, orgID, targetUserID, newRole string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE org_members SET role = $1 WHERE org_id = $2 AND user_id = $3`,
		newRole, orgID, targetUserID,
	)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return errors.New("member not found in org")
	}
	return nil
}

// RemoveMember removes a user from an org.
func (s *Store) RemoveMember(ctx context.Context, orgID, targetUserID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`,
		orgID, targetUserID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return errors.New("member not found in org")
	}
	return nil
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
