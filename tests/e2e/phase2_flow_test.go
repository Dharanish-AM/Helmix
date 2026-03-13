package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestPhase2AnalyzeInfraPipelineFlow(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	jwtPrivateKeyPath := envOrDefault("E2E_JWT_PRIVATE_KEY_PATH", "./certs/jwt-private.pem")

	if !isHealthy(apiBaseURL + "/health") {
		t.Skipf("gateway not reachable at %s", apiBaseURL)
	}

	repositoryURL := createLocalNextJSRepository(t)

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	userID, orgID, _, repoID := seedRepoGraph(t, ctx, db)

	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase2-flow-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase2-flow",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	analyzePayload, err := json.Marshal(map[string]string{
		"repo_url": repositoryURL,
		"repo_id":  repoID,
	})
	if err != nil {
		t.Fatalf("marshal analyze request failed: %v", err)
	}

	analyzeRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/repos/analyze", bytes.NewReader(analyzePayload))
	if err != nil {
		t.Fatalf("build analyze request failed: %v", err)
	}
	analyzeRequest.Header.Set("Content-Type", "application/json")
	analyzeRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	analyzeResponse, err := http.DefaultClient.Do(analyzeRequest)
	if err != nil {
		t.Fatalf("execute analyze request failed: %v", err)
	}
	defer analyzeResponse.Body.Close()

	if analyzeResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected analyze status: got %d want %d", analyzeResponse.StatusCode, http.StatusOK)
	}

	var analyzed struct {
		Result struct {
			Stack struct {
				Runtime   string `json:"runtime"`
				Framework string `json:"framework"`
			} `json:"stack"`
		} `json:"result"`
	}
	if err := json.NewDecoder(analyzeResponse.Body).Decode(&analyzed); err != nil {
		t.Fatalf("decode analyze response failed: %v", err)
	}
	if analyzed.Result.Stack.Runtime != "node" || analyzed.Result.Stack.Framework != "nextjs" {
		t.Fatalf("expected analyzed stack node/nextjs, got %s/%s", analyzed.Result.Stack.Runtime, analyzed.Result.Stack.Framework)
	}

	infraPayload, err := json.Marshal(map[string]any{
		"project_slug": "phase2-flow-next",
		"provider":     "docker",
		"stack": map[string]any{
			"runtime":   analyzed.Result.Stack.Runtime,
			"framework": analyzed.Result.Stack.Framework,
		},
	})
	if err != nil {
		t.Fatalf("marshal infra request failed: %v", err)
	}

	infraRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/infra/generate", bytes.NewReader(infraPayload))
	if err != nil {
		t.Fatalf("build infra request failed: %v", err)
	}
	infraRequest.Header.Set("Content-Type", "application/json")
	infraRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	infraResponse, err := http.DefaultClient.Do(infraRequest)
	if err != nil {
		t.Fatalf("execute infra request failed: %v", err)
	}
	defer infraResponse.Body.Close()

	if infraResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected infra status: got %d want %d", infraResponse.StatusCode, http.StatusOK)
	}

	var infraGenerated struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(infraResponse.Body).Decode(&infraGenerated); err != nil {
		t.Fatalf("decode infra response failed: %v", err)
	}
	if infraGenerated.Template != "docker-nextjs" {
		t.Fatalf("expected infra template docker-nextjs, got %q", infraGenerated.Template)
	}

	pipelinePayload, err := json.Marshal(map[string]any{
		"project_slug": "phase2-flow-next",
		"provider":     "github-actions",
		"stack": map[string]any{
			"runtime":   analyzed.Result.Stack.Runtime,
			"framework": analyzed.Result.Stack.Framework,
		},
	})
	if err != nil {
		t.Fatalf("marshal pipeline request failed: %v", err)
	}

	pipelineRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/pipelines/generate", bytes.NewReader(pipelinePayload))
	if err != nil {
		t.Fatalf("build pipeline request failed: %v", err)
	}
	pipelineRequest.Header.Set("Content-Type", "application/json")
	pipelineRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	pipelineResponse, err := http.DefaultClient.Do(pipelineRequest)
	if err != nil {
		t.Fatalf("execute pipeline request failed: %v", err)
	}
	defer pipelineResponse.Body.Close()

	if pipelineResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected pipeline status: got %d want %d", pipelineResponse.StatusCode, http.StatusOK)
	}

	var pipelineGenerated struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(pipelineResponse.Body).Decode(&pipelineGenerated); err != nil {
		t.Fatalf("decode pipeline response failed: %v", err)
	}
	if pipelineGenerated.Template != "github-actions-nextjs" {
		t.Fatalf("expected pipeline template github-actions-nextjs, got %q", pipelineGenerated.Template)
	}
}

func createLocalNextJSRepository(t *testing.T) string {
	t.Helper()

	baseDir := "/workspace"
	if info, err := os.Stat(baseDir); err != nil || !info.IsDir() {
		baseDir = t.TempDir()
	} else {
		probePath := filepath.Join(baseDir, ".helmix-write-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
		if writeErr := os.WriteFile(probePath, []byte("ok"), 0o644); writeErr != nil {
			baseDir = t.TempDir()
		} else {
			_ = os.Remove(probePath)
		}
	}

	repoPath := filepath.Join(baseDir, ".tmp-phase2-nextjs-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create temp repo dir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repoPath) })

	packageJSON := `{
	  "name": "phase2-nextjs-app",
	  "private": true,
	  "dependencies": {
	    "next": "14.2.5",
	    "react": "18.2.0",
	    "react-dom": "18.2.0"
	  }
	}`
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte(packageJSON), 0o644); err != nil {
		t.Fatalf("write package.json failed: %v", err)
	}

	runGit(t, repoPath, "init", "-b", "main")
	runGit(t, repoPath, "config", "user.email", "phase2@example.com")
	runGit(t, repoPath, "config", "user.name", "Phase2 E2E")
	runGit(t, repoPath, "add", "package.json")
	runGit(t, repoPath, "commit", "-m", "init nextjs test repo")

	return repoPath
}

func runGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repoPath
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(output))
	}
}

func requireJWTPrivateKeyPath(t *testing.T, candidate string) string {
	t.Helper()

	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedCandidate == "" {
		t.Skip("jwt private key path is not configured")
	}
	if _, err := os.Stat(trimmedCandidate); err == nil {
		return trimmedCandidate
	}

	repoRootCandidate, err := filepath.Abs(filepath.Join("..", "..", "certs", "jwt-private.pem"))
	if err == nil {
		if _, statErr := os.Stat(repoRootCandidate); statErr == nil {
			return repoRootCandidate
		}
	}

	t.Skipf("jwt private key not available at %s", trimmedCandidate)
	return ""
}
