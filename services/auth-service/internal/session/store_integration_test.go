package session

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestRefreshTokenCreateRotateDelete(t *testing.T) {
	redisURL := strings.TrimSpace(os.Getenv("AUTH_TEST_REDIS_URL"))
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	store, err := New(redisURL, 30*time.Minute)
	if err != nil {
		t.Skipf("redis unavailable for integration test: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	identity := sharedauth.User{
		UserID:         "integration-user",
		OrgID:          "integration-org",
		Role:           "owner",
		Email:          "integration@example.com",
		GitHubUsername: "integration-gh",
	}

	firstToken, err := store.Create(ctx, identity)
	if err != nil {
		t.Fatalf("create refresh token failed: %v", err)
	}
	if firstToken == "" {
		t.Fatal("expected non-empty refresh token")
	}

	fetchedUser, err := store.Get(ctx, firstToken)
	if err != nil {
		t.Fatalf("get refresh token failed: %v", err)
	}
	if fetchedUser.UserID != identity.UserID {
		t.Fatalf("unexpected token user: got %q want %q", fetchedUser.UserID, identity.UserID)
	}

	rotatedUser, rotatedToken, err := store.Rotate(ctx, firstToken)
	if err != nil {
		t.Fatalf("rotate refresh token failed: %v", err)
	}
	if rotatedToken == "" || rotatedToken == firstToken {
		t.Fatalf("expected rotated token to be new, got %q", rotatedToken)
	}
	if rotatedUser.UserID != identity.UserID {
		t.Fatalf("unexpected rotated identity: got %q want %q", rotatedUser.UserID, identity.UserID)
	}

	if _, err := store.Get(ctx, firstToken); err == nil {
		t.Fatal("expected old refresh token lookup to fail after rotation")
	}

	if err := store.Delete(ctx, rotatedToken); err != nil {
		t.Fatalf("delete rotated token failed: %v", err)
	}
	if _, err := store.Get(ctx, rotatedToken); err == nil {
		t.Fatal("expected deleted refresh token lookup to fail")
	}
}
