package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/your-org/helmix/libs/auth"
	"github.com/your-org/helmix/services/auth-service/internal/config"
	vaultclient "github.com/your-org/helmix/services/auth-service/internal/vault"
)

type fakeVaultClient struct {
	upsertFn func(ctx context.Context, service, key string, value any) (vaultclient.SecretRecord, error)
	getFn    func(ctx context.Context, service, key string) (vaultclient.SecretRecord, error)
	deleteFn func(ctx context.Context, service, key string) error
}

func (f fakeVaultClient) UpsertSecret(ctx context.Context, service, key string, value any) (vaultclient.SecretRecord, error) {
	if f.upsertFn == nil {
		return vaultclient.SecretRecord{}, nil
	}
	return f.upsertFn(ctx, service, key, value)
}

func (f fakeVaultClient) GetSecret(ctx context.Context, service, key string) (vaultclient.SecretRecord, error) {
	if f.getFn == nil {
		return vaultclient.SecretRecord{}, nil
	}
	return f.getFn(ctx, service, key)
}

func (f fakeVaultClient) DeleteSecret(ctx context.Context, service, key string) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, service, key)
}

func TestSecretsUpsertOwnerSuccess(t *testing.T) {
	hit := false
	srv := newSecretsTestServer(t, fakeVaultClient{
		upsertFn: func(_ context.Context, service, key string, value any) (vaultclient.SecretRecord, error) {
			hit = true
			if service != "deployment-engine" || key != "registry_token" {
				t.Fatalf("unexpected secret path %s/%s", service, key)
			}
			return vaultclient.SecretRecord{Service: service, Key: key, Value: value, Version: 3}, nil
		},
	})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	body := bytes.NewBufferString(`{"service":"deployment-engine","key":"registry_token","value":"abc"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/secrets", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !hit {
		t.Fatal("expected vault client upsert to be called")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload["service"] != "deployment-engine" {
		t.Fatalf("unexpected service in response: %+v", payload)
	}
}

func TestSecretsUpsertPermissionDenied(t *testing.T) {
	hit := false
	srv := newSecretsTestServer(t, fakeVaultClient{
		upsertFn: func(_ context.Context, service, key string, value any) (vaultclient.SecretRecord, error) {
			hit = true
			return vaultclient.SecretRecord{Service: service, Key: key, Value: value, Version: 1}, nil
		},
	})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "developer")

	body := bytes.NewBufferString(`{"service":"deployment-engine","key":"registry_token","value":"abc"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/secrets", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusForbidden)
	}
	if hit {
		t.Fatal("did not expect vault client upsert call for unauthorized role")
	}
}

func TestSecretsGetVaultUnavailable(t *testing.T) {
	srv := newSecretsTestServer(t, fakeVaultClient{
		getFn: func(_ context.Context, service, key string) (vaultclient.SecretRecord, error) {
			return vaultclient.SecretRecord{}, vaultclient.ErrUnavailable
		},
	})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/secrets/deployment-engine/registry_token", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload["code"] != "vault_unavailable" {
		t.Fatalf("unexpected response code: %+v", payload)
	}
}

func TestSecretsUpsertRejectsInvalidPathCharacters(t *testing.T) {
	hit := false
	srv := newSecretsTestServer(t, fakeVaultClient{
		upsertFn: func(_ context.Context, service, key string, value any) (vaultclient.SecretRecord, error) {
			hit = true
			return vaultclient.SecretRecord{Service: service, Key: key, Value: value, Version: 1}, nil
		},
	})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	body := bytes.NewBufferString(`{"service":"deployment/engine","key":"registry_token","value":"abc"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/secrets", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusBadRequest)
	}
	if hit {
		t.Fatal("did not expect vault client upsert call for invalid secret path")
	}
}

func TestSecretsUpsertRejectsOversizedStringValue(t *testing.T) {
	hit := false
	srv := newSecretsTestServer(t, fakeVaultClient{
		upsertFn: func(_ context.Context, service, key string, value any) (vaultclient.SecretRecord, error) {
			hit = true
			return vaultclient.SecretRecord{Service: service, Key: key, Value: value, Version: 1}, nil
		},
	})
	token := signServerTestJWT(t, srv.config.JWTPrivateKeyPath, "owner")

	oversized := strings.Repeat("a", 4097)
	body := bytes.NewBufferString(`{"service":"deployment-engine","key":"registry_token","value":"` + oversized + `"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/secrets", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusBadRequest)
	}
	if hit {
		t.Fatal("did not expect vault client upsert call for oversized value")
	}
}

func newSecretsTestServer(t *testing.T, vault vaultclient.SecretClient) *Server {
	t.Helper()

	privateKeyPath, publicKeyPath := writeServerTestKeys(t)
	cfg := config.Config{
		JWTPublicKeyPath:  publicKeyPath,
		JWTPrivateKeyPath: privateKeyPath,
	}

	return New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)), nil, nil, nil, vault)
}

func signServerTestJWT(t *testing.T, privateKeyPath, role string) string {
	t.Helper()

	token, err := sharedauth.SignUserToken(privateKeyPath, sharedauth.User{
		UserID:         "user-1",
		OrgID:          "org-1",
		Role:           role,
		Email:          "owner@example.com",
		GitHubUsername: "owner-gh",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}
	return token
}

func writeServerTestKeys(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa private key failed: %v", err)
	}

	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "jwt-private.pem")
	publicKeyPath := filepath.Join(tempDir, "jwt-public.pem")

	privateFile, err := os.Create(privateKeyPath)
	if err != nil {
		t.Fatalf("create private key file failed: %v", err)
	}
	defer privateFile.Close()

	if err := pem.Encode(privateFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		t.Fatalf("write private key failed: %v", err)
	}

	publicPKIX, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key failed: %v", err)
	}

	publicFile, err := os.Create(publicKeyPath)
	if err != nil {
		t.Fatalf("create public key file failed: %v", err)
	}
	defer publicFile.Close()

	if err := pem.Encode(publicFile, &pem.Block{Type: "PUBLIC KEY", Bytes: publicPKIX}); err != nil {
		t.Fatalf("write public key failed: %v", err)
	}

	return privateKeyPath, publicKeyPath
}
