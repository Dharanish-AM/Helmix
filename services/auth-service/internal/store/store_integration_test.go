package store

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestUpsertUserWithOrgCreatesAndUpdatesWithoutDuplicates(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AUTH_TEST_DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable"
	}

	store, err := Open(databaseURL)
	if err != nil {
		t.Skipf("postgres unavailable for integration test: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	ensureTestSchema(t, store.db)
	resetAuthTables(t, store.db)

	first, err := store.UpsertUserWithOrg(ctx, UpsertUserParams{
		GitHubID:        424242,
		Username:        "phase1-user",
		Email:           "phase1-user@example.com",
		AvatarURL:       "https://example.com/a.png",
		TokenNonce:      []byte("nonce-a"),
		TokenCiphertext: []byte("ciphertext-a"),
		TokenUpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if first.ID == "" || first.OrgID == "" {
		t.Fatalf("expected user and org IDs to be populated, got user=%q org=%q", first.ID, first.OrgID)
	}

	second, err := store.UpsertUserWithOrg(ctx, UpsertUserParams{
		GitHubID:        424242,
		Username:        "phase1-user-updated",
		Email:           "phase1-user-updated@example.com",
		AvatarURL:       "https://example.com/b.png",
		TokenNonce:      []byte("nonce-b"),
		TokenCiphertext: []byte("ciphertext-b"),
		TokenUpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected same user ID on update, got first=%q second=%q", first.ID, second.ID)
	}
	if second.OrgID != first.OrgID {
		t.Fatalf("expected same org ID on update, got first=%q second=%q", first.OrgID, second.OrgID)
	}

	reloaded, err := store.GetUserByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("get user by id failed: %v", err)
	}
	if reloaded.Username != "phase1-user-updated" {
		t.Fatalf("expected updated username, got %q", reloaded.Username)
	}

	assertCount(t, store.db, "SELECT COUNT(*) FROM users", 1)
	assertCount(t, store.db, "SELECT COUNT(*) FROM organizations", 1)
	assertCount(t, store.db, "SELECT COUNT(*) FROM org_members", 1)
}

func ensureTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		"CREATE EXTENSION IF NOT EXISTS pgcrypto",
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			github_id BIGINT UNIQUE NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			avatar_url TEXT,
			created_at TIMESTAMPTZ DEFAULT now(),
			github_token_nonce BYTEA,
			github_token_ciphertext BYTEA,
			github_token_updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS organizations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			owner_id UUID REFERENCES users(id),
			created_at TIMESTAMPTZ DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS org_members (
			org_id UUID REFERENCES organizations(id),
			user_id UUID REFERENCES users(id),
			role TEXT NOT NULL,
			PRIMARY KEY (org_id, user_id)
		)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("ensure schema failed for %q: %v", statement, err)
		}
	}
}

func resetAuthTables(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("TRUNCATE TABLE org_members, organizations, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate auth tables failed: %v", err)
	}
}

func assertCount(t *testing.T, db *sql.DB, query string, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("query count failed for %q: %v", query, err)
	}
	if count != expected {
		t.Fatalf("unexpected row count for %q: got %d want %d", query, count, expected)
	}
}
