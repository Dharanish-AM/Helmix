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

func TestPhase4VaultSecretsCRUDViaGateway(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	jwtPrivateKeyPath := envOrDefault("E2E_JWT_PRIVATE_KEY_PATH", "./certs/jwt-private.pem")

	if !isHealthy(apiBaseURL + "/health") {
		t.Skipf("gateway not reachable at %s", apiBaseURL)
	}

	identity := sharedauth.User{
		UserID:         fmt.Sprintf("phase4-user-%d", time.Now().UnixNano()),
		OrgID:          fmt.Sprintf("phase4-org-%d", time.Now().UnixNano()),
		Role:           "owner",
		Email:          fmt.Sprintf("phase4-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase4-e2e",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create/update secret
	createPayload, err := json.Marshal(map[string]any{
		"service": "deployment-engine",
		"key":     "registry_token",
		"value":   "phase4-token",
	})
	if err != nil {
		t.Fatalf("marshal create payload failed: %v", err)
	}
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/secrets", bytes.NewReader(createPayload))
	if err != nil {
		t.Fatalf("build create request failed: %v", err)
	}
	createReq.Header.Set("Authorization", "Bearer "+jwtToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("execute create request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create status: got %d want %d", createResp.StatusCode, http.StatusOK)
	}

	// Read secret
	readReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/api/v1/secrets/deployment-engine/registry_token", nil)
	if err != nil {
		t.Fatalf("build read request failed: %v", err)
	}
	readReq.Header.Set("Authorization", "Bearer "+jwtToken)
	readResp, err := http.DefaultClient.Do(readReq)
	if err != nil {
		t.Fatalf("execute read request failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected read status: got %d want %d", readResp.StatusCode, http.StatusOK)
	}
	var readBody struct {
		Service string `json:"service"`
		Key     string `json:"key"`
		Value   any    `json:"value"`
	}
	if err := json.NewDecoder(readResp.Body).Decode(&readBody); err != nil {
		t.Fatalf("decode read response failed: %v", err)
	}
	if readBody.Service != "deployment-engine" || readBody.Key != "registry_token" {
		t.Fatalf("unexpected read response identity: %+v", readBody)
	}
	if value, ok := readBody.Value.(string); !ok || value != "phase4-token" {
		t.Fatalf("unexpected read value: %+v", readBody.Value)
	}

	// Delete secret
	deleteReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiBaseURL+"/api/v1/secrets/deployment-engine/registry_token", nil)
	if err != nil {
		t.Fatalf("build delete request failed: %v", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+jwtToken)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("execute delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected delete status: got %d want %d", deleteResp.StatusCode, http.StatusOK)
	}
}
