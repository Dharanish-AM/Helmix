package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/your-org/helmix/libs/auth"
	"github.com/your-org/helmix/services/auth-service/internal/config"
	githubclient "github.com/your-org/helmix/services/auth-service/internal/github"
	"github.com/your-org/helmix/services/auth-service/internal/store"
	vaultclient "github.com/your-org/helmix/services/auth-service/internal/vault"
)

type fakeAuditStore struct {
	userRecord store.UserRecord
	audits     []store.AuditLogEntry
	createOrgFn func(context.Context, string, string) (store.OrgRecord, error)
}

func (f *fakeAuditStore) UpsertUserWithOrg(context.Context, store.UpsertUserParams) (store.UserRecord, error) {
	return store.UserRecord{}, errors.New("unexpected call")
}

func (f *fakeAuditStore) GetUserByID(context.Context, string) (store.UserRecord, error) {
	if f.userRecord.ID == "" {
		return store.UserRecord{}, errors.New("user not found")
	}
	return f.userRecord, nil
}

func (f *fakeAuditStore) GetGitHubTokenByUserID(context.Context, string) (store.GitHubTokenRecord, error) {
	return store.GitHubTokenRecord{}, errors.New("unexpected call")
}

func (f *fakeAuditStore) CreateAuditLog(_ context.Context, entry store.AuditLogEntry) error {
	f.audits = append(f.audits, entry)
	return nil
}

func (f *fakeAuditStore) CreateOrg(ctx context.Context, userID, name string) (store.OrgRecord, error) {
	if f.createOrgFn != nil {
		return f.createOrgFn(ctx, userID, name)
	}
	return store.OrgRecord{}, errors.New("unexpected call")
}

func (f *fakeAuditStore) GetOrgMembers(context.Context, string) ([]store.MemberRecord, error) {
	return nil, errors.New("unexpected call")
}

func (f *fakeAuditStore) CreateInvite(context.Context, string, string, string, string) (store.InviteRecord, error) {
	return store.InviteRecord{}, errors.New("unexpected call")
}

func (f *fakeAuditStore) AcceptInvite(context.Context, string, string) (string, error) {
	return "", errors.New("unexpected call")
}

func (f *fakeAuditStore) UpdateMemberRole(context.Context, string, string, string) error {
	return errors.New("unexpected call")
}

func (f *fakeAuditStore) RemoveMember(context.Context, string, string) error {
	return errors.New("unexpected call")
}

type fakeAuditSessions struct {
	rotateUser   sharedauth.User
	rotateToken  string
	rotateErr    error
	deletedToken string
	rotatedFrom  string
}

func (f *fakeAuditSessions) Create(context.Context, sharedauth.User) (string, error) {
	return "", errors.New("unexpected call")
}

func (f *fakeAuditSessions) Rotate(_ context.Context, currentToken string) (sharedauth.User, string, error) {
	f.rotatedFrom = currentToken
	return f.rotateUser, f.rotateToken, f.rotateErr
}

func (f *fakeAuditSessions) Delete(_ context.Context, token string) error {
	f.deletedToken = token
	return nil
}

type fakeAuditGitHubClient struct{}

func (fakeAuditGitHubClient) AuthorizeURL(state string) string { return state }

func (fakeAuditGitHubClient) ExchangeCode(context.Context, string) (string, error) {
	return "", errors.New("unexpected call")
}

func (fakeAuditGitHubClient) FetchUser(context.Context, string) (githubclient.User, error) {
	return githubclient.User{}, errors.New("unexpected call")
}

func (fakeAuditGitHubClient) ListRepositories(context.Context, string, int) ([]githubclient.Repository, error) {
	return nil, errors.New("unexpected call")
}

func TestLogoutWritesAuditEntry(t *testing.T) {
	auditStore := &fakeAuditStore{}
	sessions := &fakeAuditSessions{}
	srv := newAuditTestServer(t, auditStore, sessions)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(`{"refresh_token":"token-123"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "req-logout-1")
	request.RemoteAddr = "127.0.0.1:4567"

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if sessions.deletedToken != "token-123" {
		t.Fatalf("expected deleted refresh token to be captured, got %q", sessions.deletedToken)
	}
	if len(auditStore.audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.audits))
	}
	entry := auditStore.audits[0]
	if entry.EventType != "auth.logout.succeeded" {
		t.Fatalf("unexpected audit event type: %q", entry.EventType)
	}
	if entry.RequestID != "req-logout-1" {
		t.Fatalf("unexpected request id: %q", entry.RequestID)
	}
	if entry.IPAddress != "127.0.0.1" {
		t.Fatalf("unexpected ip address: %q", entry.IPAddress)
	}
}

func TestRefreshWritesSuccessAuditEntry(t *testing.T) {
	identity := sharedauth.User{
		UserID:         "user-1",
		OrgID:          "org-1",
		Role:           "owner",
		Email:          "owner@example.com",
		GitHubUsername: "owner-gh",
	}
	auditStore := &fakeAuditStore{userRecord: store.UserRecord{
		ID:             identity.UserID,
		GitHubID:       1,
		Username:       identity.GitHubUsername,
		Email:          identity.Email,
		OrgID:          identity.OrgID,
		OrgName:        "Org",
		Role:           identity.Role,
		CreatedAt:      time.Now().UTC(),
		TokenUpdatedAt: time.Now().UTC(),
	}}
	sessions := &fakeAuditSessions{rotateUser: identity, rotateToken: "refresh-rotated"}
	srv := newAuditTestServer(t, auditStore, sessions)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(`{"refresh_token":"refresh-old"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "req-refresh-1")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if sessions.rotatedFrom != "refresh-old" {
		t.Fatalf("unexpected rotated token source: %q", sessions.rotatedFrom)
	}
	if len(auditStore.audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.audits))
	}
	entry := auditStore.audits[0]
	if entry.EventType != "auth.refresh.succeeded" {
		t.Fatalf("unexpected audit event type: %q", entry.EventType)
	}
	if entry.ActorUserID != identity.UserID || entry.ActorOrgID != identity.OrgID {
		t.Fatalf("unexpected actor identity: %+v", entry)
	}

	var payload authResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode refresh response failed: %v", err)
	}
	if payload.RefreshToken != "refresh-rotated" {
		t.Fatalf("unexpected rotated refresh token in payload: %q", payload.RefreshToken)
	}
}

func TestCreateOrgWritesSuccessAuditEntry(t *testing.T) {
	auditStore := &fakeAuditStore{
		createOrgFn: func(_ context.Context, userID, name string) (store.OrgRecord, error) {
			if userID != "user-1" {
				t.Fatalf("unexpected user id: %q", userID)
			}
			if name != "Platform Team" {
				t.Fatalf("unexpected org name: %q", name)
			}
			return store.OrgRecord{
				ID:        "org-2",
				Name:      name,
				Slug:      "platform-team",
				OwnerID:   userID,
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	srv := newAuditTestServerWithVault(t, auditStore, &fakeAuditSessions{}, fakeVaultClient{})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orgs", bytes.NewBufferString(`{"name":"Platform Team"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "req-org-create-1")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got %d want %d body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if len(auditStore.audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.audits))
	}
	entry := auditStore.audits[0]
	if entry.EventType != "org.create.succeeded" {
		t.Fatalf("unexpected audit event type: %q", entry.EventType)
	}
	if entry.ActorUserID != "user-1" || entry.ActorOrgID != "org-1" {
		t.Fatalf("unexpected actor identity: %+v", entry)
	}
	if entry.RequestID != "req-org-create-1" {
		t.Fatalf("unexpected request id: %q", entry.RequestID)
	}
	if entry.Metadata["org_id"] != "org-2" {
		t.Fatalf("unexpected metadata: %+v", entry.Metadata)
	}
}

func TestGetSecretWritesFailureAuditEntry(t *testing.T) {
	auditStore := &fakeAuditStore{}
	vault := fakeVaultClient{
		getFn: func(_ context.Context, service, key string) (vaultclient.SecretRecord, error) {
			if service != "deployment-engine" || key != "missing_token" {
				t.Fatalf("unexpected secret path %s/%s", service, key)
			}
			return vaultclient.SecretRecord{}, vaultclient.ErrNotFound
		},
	}
	srv := newAuditTestServerWithVault(t, auditStore, &fakeAuditSessions{}, vault)
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/secrets/deployment-engine/missing_token", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Request-ID", "req-secret-get-1")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want %d body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if len(auditStore.audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.audits))
	}
	entry := auditStore.audits[0]
	if entry.EventType != "secret.get.failed" {
		t.Fatalf("unexpected audit event type: %q", entry.EventType)
	}
	if entry.Metadata["code"] != "secret_not_found" {
		t.Fatalf("unexpected metadata: %+v", entry.Metadata)
	}
	if entry.ActorUserID != "user-1" || entry.ActorOrgID != "org-1" {
		t.Fatalf("unexpected actor identity: %+v", entry)
	}
}

func newAuditTestServer(t *testing.T, auditStore authStore, sessions sessionStore) *Server {
	t.Helper()
	return newAuditTestServerWithVault(t, auditStore, sessions, fakeVaultClient{})
}

func newAuditTestServerWithVault(t *testing.T, auditStore authStore, sessions sessionStore, vault vaultclient.SecretClient) *Server {
	t.Helper()

	privateKeyPath, publicKeyPath := writeServerTestKeys(t)
	return New(config.Config{
		JWTPublicKeyPath:  publicKeyPath,
		JWTPrivateKeyPath: privateKeyPath,
		RefreshCookieName: "helmix_refresh_token",
		RefreshTTL:        24 * time.Hour,
		JWTTTL:            time.Hour,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), fakeAuditGitHubClient{}, auditStore, sessions, vault)
}