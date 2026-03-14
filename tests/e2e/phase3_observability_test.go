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
	"github.com/nats-io/nats.go"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestPhase3ObservabilityAlertFlow(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	natsURL := envOrDefault("E2E_NATS_URL", "nats://localhost:4222")
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

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Skipf("nats unavailable: %v", err)
	}
	t.Cleanup(nc.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	userID, orgID, projectID, _ := seedRepoGraph(t, ctx, db)
	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase3-observability-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase3-observability",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	subscription, err := nc.SubscribeSync("alert.fired")
	if err != nil {
		t.Fatalf("subscribe alert.fired failed: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })

	baseTime := time.Now().Add(-90 * time.Second).UTC()
	for index := 0; index < 4; index++ {
		capturedAt := baseTime.Add(time.Duration(index) * 30 * time.Second)
		payload, err := json.Marshal(map[string]any{
			"project_id":      projectID,
			"captured_at":     capturedAt.Format(time.RFC3339Nano),
			"cpu_pct":         45,
			"memory_pct":      55,
			"req_per_sec":     12,
			"error_rate_pct":  7.5,
			"p99_latency_ms":  250,
			"pod_count":       2,
			"ready_pod_count": 2,
		})
		if err != nil {
			t.Fatalf("marshal snapshot request failed: %v", err)
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/observability/snapshots", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("build snapshot request failed: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+jwtToken)

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("execute snapshot request failed: %v", err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			t.Fatalf("unexpected snapshot status: got %d want %d", response.StatusCode, http.StatusAccepted)
		}
	}

	message, err := subscription.NextMsg(20 * time.Second)
	if err != nil {
		t.Fatalf("wait for alert.fired event failed: %v", err)
	}

	var event struct {
		Type      string  `json:"type"`
		ProjectID string  `json:"project_id"`
		Severity  string  `json:"severity"`
		Metric    string  `json:"metric"`
		Value     float64 `json:"value"`
	}
	if err := json.Unmarshal(message.Data, &event); err != nil {
		t.Fatalf("decode alert.fired event failed: %v", err)
	}
	if event.Type != "alert.fired" {
		t.Fatalf("unexpected event type: got %q want %q", event.Type, "alert.fired")
	}
	if event.ProjectID != projectID {
		t.Fatalf("unexpected event project id: got %q want %q", event.ProjectID, projectID)
	}
	if event.Severity != "critical" {
		t.Fatalf("unexpected event severity: got %q want %q", event.Severity, "critical")
	}

	alertsRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/api/v1/observability/alerts/"+projectID, nil)
	if err != nil {
		t.Fatalf("build alerts request failed: %v", err)
	}
	alertsRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	alertsResponse, err := http.DefaultClient.Do(alertsRequest)
	if err != nil {
		t.Fatalf("execute alerts request failed: %v", err)
	}
	defer alertsResponse.Body.Close()
	if alertsResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected alerts status: got %d want %d", alertsResponse.StatusCode, http.StatusOK)
	}

	var alerts []struct {
		ProjectID string  `json:"project_id"`
		Rule      string  `json:"rule"`
		Severity  string  `json:"severity"`
		Status    string  `json:"status"`
		Metric    string  `json:"metric"`
		Value     float64 `json:"value"`
		Threshold float64 `json:"threshold"`
		Title     string  `json:"title"`
	}
	if err := json.NewDecoder(alertsResponse.Body).Decode(&alerts); err != nil {
		t.Fatalf("decode alerts response failed: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 open alert, got %d", len(alerts))
	}
	if alerts[0].ProjectID != projectID || alerts[0].Severity != "critical" || alerts[0].Status != "open" {
		t.Fatalf("unexpected alert payload: %+v", alerts[0])
	}
	if alerts[0].Rule == "" || alerts[0].Metric == "" || alerts[0].Value <= 0 || alerts[0].Threshold <= 0 {
		t.Fatalf("expected real alert detail fields, got %+v", alerts[0])
	}

	currentRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/api/v1/observability/metrics/"+projectID+"/current", nil)
	if err != nil {
		t.Fatalf("build current metrics request failed: %v", err)
	}
	currentRequest.Header.Set("Authorization", "Bearer "+jwtToken)

	currentResponse, err := http.DefaultClient.Do(currentRequest)
	if err != nil {
		t.Fatalf("execute current metrics request failed: %v", err)
	}
	defer currentResponse.Body.Close()
	if currentResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected current metrics status: got %d want %d", currentResponse.StatusCode, http.StatusOK)
	}

	var current struct {
		ProjectID    string  `json:"project_id"`
		ErrorRatePct float64 `json:"error_rate_pct"`
	}
	if err := json.NewDecoder(currentResponse.Body).Decode(&current); err != nil {
		t.Fatalf("decode current metrics response failed: %v", err)
	}
	if current.ProjectID != projectID || current.ErrorRatePct <= 5 {
		t.Fatalf("unexpected current metrics payload: %+v", current)
	}
}

// TestPhase3ObservabilityLatencyAlert verifies that 6 consecutive snapshots with
// P99 latency above the 2000 ms threshold produce an alert.fired event with
// rule="p99-latency-high" and severity="warning".
func TestPhase3ObservabilityLatencyAlert(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	natsURL := envOrDefault("E2E_NATS_URL", "nats://localhost:4222")
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

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Skipf("nats unavailable: %v", err)
	}
	t.Cleanup(nc.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	userID, orgID, projectID, _ := seedRepoGraph(t, ctx, db)
	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase3-latency-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase3-latency",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	subscription, err := nc.SubscribeSync("alert.fired")
	if err != nil {
		t.Fatalf("subscribe alert.fired failed: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })

	// Post 6 snapshots — exactly the consecutive window required by the latency rule.
	baseTime := time.Now().Add(-3 * time.Minute).UTC()
	for index := 0; index < 6; index++ {
		capturedAt := baseTime.Add(time.Duration(index) * 30 * time.Second)
		payload, err := json.Marshal(map[string]any{
			"project_id":      projectID,
			"captured_at":     capturedAt.Format(time.RFC3339Nano),
			"cpu_pct":         35,
			"memory_pct":      45,
			"req_per_sec":     20,
			"error_rate_pct":  1.0,
			"p99_latency_ms":  2800,
			"pod_count":       2,
			"ready_pod_count": 2,
		})
		if err != nil {
			t.Fatalf("marshal latency snapshot failed: %v", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/observability/snapshots", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("build latency snapshot request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwtToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post latency snapshot failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("unexpected snapshot status for latency: got %d want 202", resp.StatusCode)
		}
	}

	msg, err := subscription.NextMsg(20 * time.Second)
	if err != nil {
		t.Fatalf("wait for alert.fired (latency) failed: %v", err)
	}
	var event struct {
		Type      string `json:"type"`
		ProjectID string `json:"project_id"`
		Severity  string `json:"severity"`
		Metric    string `json:"metric"`
		Rule      string `json:"rule"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		t.Fatalf("decode alert.fired (latency) event failed: %v", err)
	}
	if event.Type != "alert.fired" {
		t.Fatalf("unexpected event type: got %q want alert.fired", event.Type)
	}
	if event.ProjectID != projectID {
		t.Fatalf("unexpected project_id: got %q want %q", event.ProjectID, projectID)
	}
	if event.Metric != "p99_latency_ms" {
		t.Fatalf("unexpected metric: got %q want p99_latency_ms", event.Metric)
	}
	if event.Severity != "warning" {
		t.Fatalf("unexpected severity: got %q want warning", event.Severity)
	}
}

// TestPhase3ObservabilityZeroPodAlert verifies that a single snapshot with
// ready_pod_count=0 immediately fires an alert.fired event with
// rule="ready-pods-zero" and severity="critical".
func TestPhase3ObservabilityZeroPodAlert(t *testing.T) {
	apiBaseURL := envOrDefault("E2E_API_BASE_URL", "http://localhost:8080")
	databaseURL := envOrDefault("E2E_DATABASE_URL", "postgres://helmix:helmix@localhost:5432/helmix?sslmode=disable")
	natsURL := envOrDefault("E2E_NATS_URL", "nats://localhost:4222")
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

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Skipf("nats unavailable: %v", err)
	}
	t.Cleanup(nc.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	userID, orgID, projectID, _ := seedRepoGraph(t, ctx, db)
	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase3-zeropod-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase3-zeropod",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	subscription, err := nc.SubscribeSync("alert.fired")
	if err != nil {
		t.Fatalf("subscribe alert.fired failed: %v", err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })

	// A single snapshot with zero ready pods is all that is required.
	payload, err := json.Marshal(map[string]any{
		"project_id":      projectID,
		"captured_at":     time.Now().UTC().Format(time.RFC3339Nano),
		"cpu_pct":         55,
		"memory_pct":      60,
		"req_per_sec":     0,
		"error_rate_pct":  0,
		"p99_latency_ms":  0,
		"pod_count":       3,
		"ready_pod_count": 0,
	})
	if err != nil {
		t.Fatalf("marshal zero-pod snapshot failed: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/observability/snapshots", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build zero-pod request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post zero-pod snapshot failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected snapshot status for zero-pod: got %d want 202", resp.StatusCode)
	}

	msg, err := subscription.NextMsg(20 * time.Second)
	if err != nil {
		t.Fatalf("wait for alert.fired (zero-pod) failed: %v", err)
	}
	var event struct {
		Type      string `json:"type"`
		ProjectID string `json:"project_id"`
		Severity  string `json:"severity"`
		Metric    string `json:"metric"`
		Rule      string `json:"rule"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		t.Fatalf("decode alert.fired (zero-pod) event failed: %v", err)
	}
	if event.Type != "alert.fired" {
		t.Fatalf("unexpected event type: got %q want alert.fired", event.Type)
	}
	if event.ProjectID != projectID {
		t.Fatalf("unexpected project_id: got %q want %q", event.ProjectID, projectID)
	}
	if event.Metric != "ready_pod_count" {
		t.Fatalf("unexpected metric: got %q want ready_pod_count", event.Metric)
	}
	if event.Severity != "critical" {
		t.Fatalf("unexpected severity: got %q want critical", event.Severity)
	}
}
