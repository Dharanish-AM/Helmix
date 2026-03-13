package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

func TestPhase3IncidentFlow(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	userID, orgID, projectID, _ := seedRepoGraph(t, ctx, db)
	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase3-incident-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase3-incident",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	incidentCreatedSub, err := nc.SubscribeSync("incident.created")
	if err != nil {
		t.Fatalf("subscribe incident.created failed: %v", err)
	}
	t.Cleanup(func() { _ = incidentCreatedSub.Unsubscribe() })

	autohealSub, err := nc.SubscribeSync("autoheal.triggered")
	if err != nil {
		t.Fatalf("subscribe autoheal.triggered failed: %v", err)
	}
	t.Cleanup(func() { _ = autohealSub.Unsubscribe() })

	baseTime := time.Now().Add(-90 * time.Second).UTC()
	for index := 0; index < 4; index++ {
		capturedAt := baseTime.Add(time.Duration(index) * 30 * time.Second)
		payload, err := json.Marshal(map[string]any{
			"project_id":      projectID,
			"captured_at":     capturedAt.Format(time.RFC3339Nano),
			"cpu_pct":         40,
			"memory_pct":      45,
			"req_per_sec":     10,
			"error_rate_pct":  8.5,
			"p99_latency_ms":  400,
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

	incidentMessage, err := incidentCreatedSub.NextMsg(30 * time.Second)
	if err != nil {
		t.Fatalf("wait for incident.created event failed: %v", err)
	}

	var incidentEvent struct {
		IncidentID string `json:"incident_id"`
		ProjectID  string `json:"project_id"`
		Type       string `json:"type"`
	}
	if err := json.Unmarshal(incidentMessage.Data, &incidentEvent); err != nil {
		t.Fatalf("decode incident.created event failed: %v", err)
	}
	if incidentEvent.Type != "incident.created" || incidentEvent.ProjectID != projectID || incidentEvent.IncidentID == "" {
		t.Fatalf("unexpected incident event payload: %+v", incidentEvent)
	}

	var diagnosisRaw []byte
	if err := db.QueryRowContext(ctx, `SELECT ai_diagnosis FROM incidents WHERE id = $1`, incidentEvent.IncidentID).Scan(&diagnosisRaw); err != nil {
		t.Fatalf("query incident diagnosis failed: %v", err)
	}
	var diagnosis struct {
		RootCause string  `json:"root_cause"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(diagnosisRaw, &diagnosis); err != nil {
		t.Fatalf("decode incident diagnosis failed: %v", err)
	}
	if diagnosis.RootCause == "" || diagnosis.Confidence <= 0 {
		t.Fatalf("unexpected diagnosis payload: %+v", diagnosis)
	}

	actionPayload, err := json.Marshal(map[string]any{"action": "restart_pods", "params": map[string]any{}})
	if err != nil {
		t.Fatalf("marshal action request failed: %v", err)
	}
	actionRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/incidents/"+incidentEvent.IncidentID+"/actions", bytes.NewReader(actionPayload))
	if err != nil {
		t.Fatalf("build action request failed: %v", err)
	}
	actionRequest.Header.Set("Content-Type", "application/json")
	actionRequest.Header.Set("Authorization", "Bearer "+jwtToken)
	actionResponse, err := http.DefaultClient.Do(actionRequest)
	if err != nil {
		t.Fatalf("execute action request failed: %v", err)
	}
	defer actionResponse.Body.Close()
	if actionResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected action status: got %d want %d", actionResponse.StatusCode, http.StatusOK)
	}

	autohealMessage, err := autohealSub.NextMsg(20 * time.Second)
	if err != nil {
		t.Fatalf("wait for autoheal.triggered event failed: %v", err)
	}

	var autohealEvent struct {
		Type       string `json:"type"`
		IncidentID string `json:"incident_id"`
		Action     string `json:"action"`
	}
	if err := json.Unmarshal(autohealMessage.Data, &autohealEvent); err != nil {
		t.Fatalf("decode autoheal event failed: %v", err)
	}
	if autohealEvent.Type != "autoheal.triggered" || autohealEvent.IncidentID != incidentEvent.IncidentID || autohealEvent.Action != "restart_pods" {
		t.Fatalf("unexpected autoheal event payload: %+v", autohealEvent)
	}
}

func TestPhase3IncidentGatewayRoutes(t *testing.T) {
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

	userID, orgID, projectID, _ := seedRepoGraph(t, ctx, db)
	identity := sharedauth.User{
		UserID:         userID,
		OrgID:          orgID,
		Role:           "owner",
		Email:          fmt.Sprintf("phase3-incident-routes-%d@example.com", time.Now().UnixNano()),
		GitHubUsername: "phase3-incident-routes",
	}
	jwtToken, err := sharedauth.SignUserToken(jwtPrivateKeyPath, identity, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	var alertID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO alerts (project_id, severity, title, description, status)
		VALUES ($1, 'critical', 'phase3 test alert', 'route-level verification', 'open')
		RETURNING id`, projectID).Scan(&alertID); err != nil {
		t.Fatalf("insert alert failed: %v", err)
	}

	var incidentID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO incidents (alert_id, project_id, ai_diagnosis, ai_actions)
		VALUES ($1, $2, $3::jsonb, '[]'::jsonb)
		RETURNING id`, alertID, projectID, `{"root_cause":"route test","confidence":0.9,"reasoning":"seeded","recommended_actions":[{"action":"restart_pods","params":{}}],"auto_execute":false}`).Scan(&incidentID); err != nil {
		t.Fatalf("insert incident failed: %v", err)
	}

	listRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/api/v1/incidents/projects/"+projectID+"?limit=5&offset=0", nil)
	if err != nil {
		t.Fatalf("build incidents list request failed: %v", err)
	}
	listRequest.Header.Set("Authorization", "Bearer "+jwtToken)
	listResponse, err := http.DefaultClient.Do(listRequest)
	if err != nil {
		t.Fatalf("execute incidents list request failed: %v", err)
	}
	defer listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected incidents list status: got %d want %d", listResponse.StatusCode, http.StatusOK)
	}

	bodyBytes, err := io.ReadAll(listResponse.Body)
	if err != nil {
		t.Fatalf("read incidents list response failed: %v", err)
	}

	var incidentsPage struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(bodyBytes, &incidentsPage); err != nil {
		var legacyIncidents []json.RawMessage
		if legacyErr := json.Unmarshal(bodyBytes, &legacyIncidents); legacyErr == nil {
			t.Fatalf("incidents list returned legacy array response; paginated envelope is now required (items,total,limit,offset)")
		}
		t.Fatalf("decode incidents list response failed: %v", err)
	}
	if incidentsPage.Limit != 5 {
		t.Fatalf("expected limit=5 in incidents response envelope, got %d", incidentsPage.Limit)
	}
	if incidentsPage.Offset != 0 {
		t.Fatalf("expected offset=0 in incidents response envelope, got %d", incidentsPage.Offset)
	}
	if incidentsPage.Total < 1 {
		t.Fatalf("expected total >= 1 in paginated response, got %d", incidentsPage.Total)
	}
	if len(incidentsPage.Items) == 0 {
		t.Fatal("expected at least one incident in paginated response items")
	}
	foundIncident := false
	for _, incident := range incidentsPage.Items {
		if incident.ID == incidentID {
			foundIncident = true
			break
		}
	}
	if !foundIncident {
		t.Fatalf("expected seeded incident %q in list response", incidentID)
	}

	actionPayload, err := json.Marshal(map[string]any{"action": "restart_pods", "params": map[string]any{}})
	if err != nil {
		t.Fatalf("marshal action request failed: %v", err)
	}
	actionRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/v1/incidents/"+incidentID+"/actions", bytes.NewReader(actionPayload))
	if err != nil {
		t.Fatalf("build action request failed: %v", err)
	}
	actionRequest.Header.Set("Content-Type", "application/json")
	actionRequest.Header.Set("Authorization", "Bearer "+jwtToken)
	actionResponse, err := http.DefaultClient.Do(actionRequest)
	if err != nil {
		t.Fatalf("execute action request failed: %v", err)
	}
	defer actionResponse.Body.Close()
	if actionResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected action status: got %d want %d", actionResponse.StatusCode, http.StatusOK)
	}

	var actionResult struct {
		IncidentID string `json:"incident_id"`
		Action     string `json:"action"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(actionResponse.Body).Decode(&actionResult); err != nil {
		t.Fatalf("decode action response failed: %v", err)
	}
	if actionResult.IncidentID != incidentID || actionResult.Action != "restart_pods" {
		t.Fatalf("unexpected action response payload: %+v", actionResult)
	}
}