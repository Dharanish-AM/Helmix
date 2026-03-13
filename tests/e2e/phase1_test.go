package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

type analyzeResponse struct {
	RepoID string `json:"repo_id"`
	Result struct {
		Stack struct {
			Runtime   string `json:"runtime"`
			Framework string `json:"framework"`
		} `json:"stack"`
	} `json:"result"`
}

func TestPhase1AnalyzeViaGatewayPublishesEvent(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	natsURL := envOrDefault("E2E_NATS_URL", "nats://localhost:4222")
	jwtPrivateKeyPath := envOrDefault("E2E_JWT_PRIVATE_KEY_PATH", "./certs/jwt-private.pem")
	repositoryURL := envOrDefault("E2E_REPO_URL", "https://github.com/gin-gonic/examples.git")

	if !isHealthy(apiBaseURL + "/health") {
		t.Skipf("gateway not reachable at %s", apiBaseURL)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Skipf("nats unavailable: %v", err)
	}
	t.Cleanup(nc.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	userID, orgID, projectID, repoID := seedRepoGraph(t, ctx, db)

	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase1-e2e-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase1-e2e",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	subscription, err := nc.SubscribeSync("repo.analyzed")
	if err != nil {
		t.Fatalf("subscribe repo.analyzed failed: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })

	requestBody := map[string]string{
		"repo_url": repositoryURL,
		"repo_id":  repoID,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/repos/analyze", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build analyze request failed: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute analyze request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected analyze status: got %d want %d", response.StatusCode, http.StatusOK)
	}

	var parsedResponse analyzeResponse
	if err := json.NewDecoder(response.Body).Decode(&parsedResponse); err != nil {
		t.Fatalf("decode analyze response failed: %v", err)
	}
	if parsedResponse.RepoID != repoID {
		t.Fatalf("unexpected repo id in response: got %q want %q", parsedResponse.RepoID, repoID)
	}
	if strings.TrimSpace(parsedResponse.Result.Stack.Runtime) == "" {
		t.Fatalf("expected detected runtime in response, got empty (framework=%q)", parsedResponse.Result.Stack.Framework)
	}

	message, err := subscription.NextMsg(30 * time.Second)
	if err != nil {
		t.Fatalf("wait for repo.analyzed event failed: %v", err)
	}

	var event struct {
		Type   string `json:"type"`
		RepoID string `json:"repo_id"`
	}
	if err := json.Unmarshal(message.Data, &event); err != nil {
		t.Fatalf("decode repo.analyzed event failed: %v", err)
	}
	if event.Type != "repo.analyzed" {
		t.Fatalf("unexpected event type: got %q want %q", event.Type, "repo.analyzed")
	}
	if event.RepoID != repoID {
		t.Fatalf("unexpected event repo_id: got %q want %q", event.RepoID, repoID)
	}

	var detectedStackRaw []byte
	if err := db.QueryRowContext(ctx, "SELECT detected_stack FROM repos WHERE id = $1", repoID).Scan(&detectedStackRaw); err != nil {
		t.Fatalf("query detected_stack failed: %v", err)
	}
	if len(bytes.TrimSpace(detectedStackRaw)) == 0 || string(bytes.TrimSpace(detectedStackRaw)) == "null" {
		t.Fatal("expected repos.detected_stack to be persisted")
	}

	_, _, _ = orgID, projectID, userID
}

func seedRepoGraph(t *testing.T, ctx context.Context, db *sql.DB) (userID string, orgID string, projectID string, repoID string) {
	t.Helper()

	randomSuffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	username := "phase1e2e-" + randomSuffix
	email := "phase1e2e-" + randomSuffix + "@example.com"
	orgSlug := "phase1-org-" + randomSuffix
	projectSlug := "phase1-proj-" + randomSuffix
	githubRepo := "phase1/e2e-" + randomSuffix

	if err := db.QueryRowContext(ctx, `
		INSERT INTO users (github_id, username, email)
		VALUES ($1, $2, $3)
		RETURNING id`, time.Now().UnixNano(), username, email).Scan(&userID); err != nil {
		t.Fatalf("insert user failed: %v", err)
	}

	if err := db.QueryRowContext(ctx, `
		INSERT INTO organizations (name, slug, owner_id)
		VALUES ($1, $2, $3)
		RETURNING id`, "Phase1 E2E Org", orgSlug, userID).Scan(&orgID); err != nil {
		t.Fatalf("insert organization failed: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO org_members (org_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`, orgID, userID, "owner"); err != nil {
		t.Fatalf("insert org member failed: %v", err)
	}

	if err := db.QueryRowContext(ctx, `
		INSERT INTO projects (org_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id`, orgID, "Phase1 E2E Project", projectSlug).Scan(&projectID); err != nil {
		t.Fatalf("insert project failed: %v", err)
	}

	if err := db.QueryRowContext(ctx, `
		INSERT INTO repos (project_id, github_repo, default_branch)
		VALUES ($1, $2, $3)
		RETURNING id`, projectID, githubRepo, "main").Scan(&repoID); err != nil {
		t.Fatalf("insert repo failed: %v", err)
	}

	return userID, orgID, projectID, repoID
}

func isHealthy(url string) bool {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
