package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestPhase2GatewayInfraGenerateAuthorized(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	jwtPrivateKeyPath := envOrDefault("E2E_JWT_PRIVATE_KEY_PATH", "./certs/jwt-private.pem")

	if !isHealthy(apiBaseURL + "/health") {
		t.Skipf("gateway not reachable at %s", apiBaseURL)
	}

	identity := sharedauth.User{
		UserID:         fmt.Sprintf("phase2-user-%d", time.Now().UnixNano()),
		OrgID:          fmt.Sprintf("phase2-org-%d", time.Now().UnixNano()),
		Role:           "owner",
		Email:          fmt.Sprintf("phase2-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase2-e2e",
	}

	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_slug": "phase2-demo-next",
		"provider":     "docker",
		"stack": map[string]any{
			"runtime":   "node",
			"framework": "nextjs",
		},
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/infra/generate", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", response.StatusCode, http.StatusOK)
	}

	var parsed struct {
		Template string `json:"template"`
		Files    []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if parsed.Template != "docker-nextjs" {
		t.Fatalf("unexpected template: got %q want %q", parsed.Template, "docker-nextjs")
	}
	if len(parsed.Files) == 0 {
		t.Fatal("expected generated files in response")
	}
}
