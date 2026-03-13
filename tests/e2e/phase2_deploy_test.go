package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestPhase2GatewayDeploymentRollbackAuthorized(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	jwtPrivateKeyPath := requireJWTPrivateKeyPath(t, envOrDefault("E2E_JWT_PRIVATE_KEY_PATH", "./certs/jwt-private.pem"))

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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	userID, orgID, _, repoID := seedRepoGraph(t, ctx, db)
	previousDeploymentID := seedLiveDeployment(t, ctx, db, repoID, "production", "phase2-prev-sha", "ghcr.io/helmix/demo:prev")

	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase2-deploy-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase2-deploy",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	deployResponse := startDeploymentRequest(t, ctx, apiBaseURL, jwtToken, map[string]any{
		"repo_id":             repoID,
		"commit_sha":          "phase2-deploy-sha",
		"branch":              "main",
		"environment":         "production",
		"image_tag":           "ghcr.io/helmix/demo:phase2-deploy-sha",
		"ready_after_seconds": 1,
	})

	if deployResponse.Status != "deploying" {
		t.Fatalf("unexpected initial deployment status: got %q want %q", deployResponse.Status, "deploying")
	}

	liveDeployment := waitForDeploymentState(t, ctx, apiBaseURL, jwtToken, deployResponse.ID, func(response deploymentStatusResponse) bool {
		return response.Status == "live" && response.CurrentLiveDeploymentID == response.ID && response.PreviousDeploymentID == previousDeploymentID
	})

	rollbackRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/deployments/deployments/"+liveDeployment.ID+"/rollback", nil)
	if err != nil {
		t.Fatalf("build rollback request failed: %v", err)
	}
	rollbackRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	rollbackResponse, err := http.DefaultClient.Do(rollbackRequest)
	if err != nil {
		t.Fatalf("execute rollback request failed: %v", err)
	}
	defer rollbackResponse.Body.Close()

	if rollbackResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected rollback status: got %d want %d", rollbackResponse.StatusCode, http.StatusOK)
	}

	var rolledBack deploymentStatusResponse
	if err := json.NewDecoder(rollbackResponse.Body).Decode(&rolledBack); err != nil {
		t.Fatalf("decode rollback response failed: %v", err)
	}
	if rolledBack.Status != "rolled_back" {
		t.Fatalf("unexpected rolled back status: got %q want %q", rolledBack.Status, "rolled_back")
	}
	if rolledBack.CurrentLiveDeploymentID != previousDeploymentID {
		t.Fatalf("unexpected active deployment after rollback: got %q want %q", rolledBack.CurrentLiveDeploymentID, previousDeploymentID)
	}

	assertDeploymentStatusInDB(t, ctx, db, liveDeployment.ID, "rolled_back")
	assertDeploymentStatusInDB(t, ctx, db, previousDeploymentID, "live")
}

type deploymentStatusResponse struct {
	ID                      string `json:"id"`
	Status                  string `json:"status"`
	CurrentLiveDeploymentID string `json:"current_live_deployment_id"`
	PreviousDeploymentID    string `json:"previous_deployment_id"`
}

func startDeploymentRequest(t *testing.T, ctx context.Context, apiBaseURL, jwtToken string, payload map[string]any) deploymentStatusResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal deployment request failed: %v", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/deployments/deploy", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build deployment request failed: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute deployment request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected deployment status: got %d want %d", response.StatusCode, http.StatusAccepted)
	}

	var parsed deploymentStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode deployment response failed: %v", err)
	}
	if parsed.ID == "" {
		t.Fatal("expected deployment id in response")
	}
	return parsed
}

func waitForDeploymentState(t *testing.T, ctx context.Context, apiBaseURL, jwtToken, deploymentID string, predicate func(deploymentStatusResponse) bool) deploymentStatusResponse {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/api/v1/deployments/deployments/"+deploymentID, nil)
		if err != nil {
			t.Fatalf("build status request failed: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+jwtToken)

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("execute status request failed: %v", err)
		}

		var parsed deploymentStatusResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&parsed)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("unexpected deployment poll status: got %d want %d", response.StatusCode, http.StatusOK)
		}
		if decodeErr != nil {
			t.Fatalf("decode deployment poll response failed: %v", decodeErr)
		}
		if predicate(parsed) {
			return parsed
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("deployment %s did not reach expected state before timeout", deploymentID)
	return deploymentStatusResponse{}
}

func seedLiveDeployment(t *testing.T, ctx context.Context, db *sql.DB, repoID, environment, commitSHA, imageTag string) string {
	t.Helper()

	var deploymentID string
	deployedAt := time.Now().Add(-5 * time.Minute).UTC()
	if err := db.QueryRowContext(ctx, `
		INSERT INTO deployments (repo_id, commit_sha, branch, status, environment, image_tag, deployed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`, repoID, commitSHA, "main", "live", environment, imageTag, deployedAt).Scan(&deploymentID); err != nil {
		t.Fatalf("insert live deployment failed: %v", err)
	}
	return deploymentID
}

func assertDeploymentStatusInDB(t *testing.T, ctx context.Context, db *sql.DB, deploymentID, expectedStatus string) {
	t.Helper()

	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM deployments WHERE id = $1`, deploymentID).Scan(&status); err != nil {
		t.Fatalf("query deployment status failed: %v", err)
	}
	if status != expectedStatus {
		t.Fatalf("unexpected database status for %s: got %q want %q", deploymentID, status, expectedStatus)
	}
}