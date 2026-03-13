package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignAndParseUserToken(t *testing.T) {
	privateKeyPath, publicKeyPath := writeTestKeys(t)

	inputUser := User{
		UserID:         "user-1",
		OrgID:          "org-1",
		Role:           "owner",
		Email:          "owner@example.com",
		GitHubUsername: "octocat",
	}

	token, err := SignUserToken(privateKeyPath, inputUser, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	parsedUser, err := ParseUserToken(publicKeyPath, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if parsedUser.UserID != inputUser.UserID {
		t.Fatalf("expected user id %q, got %q", inputUser.UserID, parsedUser.UserID)
	}
	if parsedUser.OrgID != inputUser.OrgID {
		t.Fatalf("expected org id %q, got %q", inputUser.OrgID, parsedUser.OrgID)
	}
	if parsedUser.Role != inputUser.Role {
		t.Fatalf("expected role %q, got %q", inputUser.Role, parsedUser.Role)
	}
}

func TestJWTMiddlewareInjectsUser(t *testing.T) {
	privateKeyPath, publicKeyPath := writeTestKeys(t)
	token, err := SignUserToken(privateKeyPath, User{
		UserID:         "user-2",
		OrgID:          "org-2",
		Role:           "admin",
		Email:          "admin@example.com",
		GitHubUsername: "hubber",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler := JWTMiddleware(publicKeyPath)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			t.Fatal("expected user in context")
		}
		if user.UserID != "user-2" {
			t.Fatalf("expected user id user-2, got %q", user.UserID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
}

func TestRequireRoleRejectsUnexpectedRole(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request = request.WithContext(ContextWithUser(request.Context(), &User{UserID: "user-3", Role: "viewer"}))
	response := httptest.NewRecorder()

	handler := RequireRole("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	for _, role := range []string{"owner", "admin", "developer"} {
		role := role
		t.Run(role, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request = request.WithContext(ContextWithUser(request.Context(), &User{UserID: "user-4", Role: role}))
			response := httptest.NewRecorder()

			handler := RequireRole("owner", "admin", "developer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("role %q: expected status %d, got %d", role, http.StatusNoContent, response.Code)
			}
		})
	}
}

func TestRequireRoleRejectsUnauthenticated(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	response := httptest.NewRecorder()

	handler := RequireRole("owner")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func writeTestKeys(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "jwt-private.pem")
	publicKeyPath := filepath.Join(tempDir, "jwt-public.pem")

	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(privateKeyPath, privatePEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})
	if err := os.WriteFile(publicKeyPath, publicPEM, 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	return privateKeyPath, publicKeyPath
}
