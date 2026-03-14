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
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			service TEXT NOT NULL,
			event_type TEXT NOT NULL,
			actor_user_id UUID REFERENCES users(id),
			actor_org_id UUID REFERENCES organizations(id),
			request_id TEXT,
			ip_address TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
	if _, err := db.Exec("TRUNCATE TABLE audit_logs, org_members, organizations, users RESTART IDENTITY CASCADE"); err != nil {
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

func TestCreateAuditLogPersistsEntry(t *testing.T) {
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

	user, err := store.UpsertUserWithOrg(ctx, UpsertUserParams{
		GitHubID:        900001,
		Username:        "audit-user",
		Email:           "audit-user@example.com",
		AvatarURL:       "https://example.com/audit.png",
		TokenNonce:      []byte("nonce-audit"),
		TokenCiphertext: []byte("ciphertext-audit"),
		TokenUpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	err = store.CreateAuditLog(ctx, AuditLogEntry{
		Service:     "auth-service",
		EventType:   "auth.logout.succeeded",
		ActorUserID: user.ID,
		ActorOrgID:  user.OrgID,
		RequestID:   "req-audit-1",
		IPAddress:   "127.0.0.1",
		Metadata: map[string]any{
			"path":   "/auth/logout",
			"status": "logged_out",
		},
	})
	if err != nil {
		t.Fatalf("create audit log failed: %v", err)
	}

	assertCount(t, store.db, "SELECT COUNT(*) FROM audit_logs", 1)

	var serviceName string
	var eventType string
	var requestID string
	if err := store.db.QueryRow(`SELECT service, event_type, request_id FROM audit_logs LIMIT 1`).Scan(&serviceName, &eventType, &requestID); err != nil {
		t.Fatalf("load audit log row failed: %v", err)
	}
	if serviceName != "auth-service" || eventType != "auth.logout.succeeded" || requestID != "req-audit-1" {
		t.Fatalf("unexpected audit row: service=%q event=%q request_id=%q", serviceName, eventType, requestID)
	}
}
