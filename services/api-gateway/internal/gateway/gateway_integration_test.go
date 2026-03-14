package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/your-org/helmix/libs/auth"
	"github.com/your-org/helmix/services/api-gateway/internal/config"
)

func TestRequestIDInjected(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if strings.TrimSpace(recorder.Header().Get("X-Request-ID")) == "" {
		t.Fatal("expected gateway to inject X-Request-ID")
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", recorder.Header().Get("X-Content-Type-Options"))
	}
	if recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("unexpected X-Frame-Options: %q", recorder.Header().Get("X-Frame-Options"))
	}
	if recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("unexpected Referrer-Policy: %q", recorder.Header().Get("Referrer-Policy"))
	}
	if recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if recorder.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("expected Strict-Transport-Security header for https-forwarded request")
	}
}

func TestUnauthenticatedRequestRejected(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/health", nil)
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusUnauthorized)
	}

	var envelope errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if envelope.Code != "unauthorized" {
		t.Fatalf("unexpected error code: got %q want %q", envelope.Code, "unauthorized")
	}
	if strings.TrimSpace(envelope.RequestID) == "" {
		t.Fatal("expected error response to include request_id")
	}
}

func TestRateLimitEnforced(t *testing.T) {
	gateway := newTestGateway(t)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase1-rate-limit-user")

	for i := int64(0); i < readRequestsPerMinute; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/projects", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		gateway.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status at request %d: got %d want %d", i+1, recorder.Code, http.StatusOK)
		}
		time.Sleep(2 * time.Millisecond)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status for limited request: got %d want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if strings.TrimSpace(recorder.Header().Get("Retry-After")) == "" {
		t.Fatal("expected Retry-After header for limited request")
	}
}

func TestWriteRateLimitEnforced(t *testing.T) {
	gateway := newTestGateway(t)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase4-write-limit-user")

	bodyPayload := `{"service":"deployment-engine","key":"registry_token","value":"abc"}`
	for i := int64(0); i < writeRequestsPerMinute; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(bodyPayload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		gateway.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status at write request %d: got %d want %d", i+1, recorder.Code, http.StatusOK)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(bodyPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status for limited write request: got %d want %d", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestSecretsBodyTooLargeRejected(t *testing.T) {
	upstreamCalled := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase4-large-body-user")
	largeValue := strings.Repeat("a", 17*1024)
	body := strings.NewReader(`{"service":"deployment-engine","key":"registry_token","value":"` + largeValue + `"}`)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	if upstreamCalled {
		t.Fatal("did not expect upstream call for oversized request body")
	}
}

func newTestGateway(t *testing.T) *Gateway {
	t.Helper()

	return newTestGatewayWithUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
}

func newTestGatewayWithUpstream(t *testing.T, upstreamHandler http.Handler) *Gateway {
	t.Helper()

	redisURL := strings.TrimSpace(os.Getenv("GATEWAY_TEST_REDIS_URL"))
	if redisURL == "" {
		redisURL = "redis://localhost:6379/1"
	}

	privateKeyPath, publicKeyPath := writeTestKeys(t)

	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	cfg := config.Config{
		Port:                     "18080",
		JWTPublicKeyPath:         publicKeyPath,
		RedisURL:                 redisURL,
		DashboardOrigin:          "http://localhost:3000",
		AuthServiceURL:           upstream.URL,
		RepoAnalyzerServiceURL:   upstream.URL,
		InfraGeneratorServiceURL: upstream.URL,
		PipelineServiceURL:       upstream.URL,
		DeploymentServiceURL:     upstream.URL,
		ObservabilityServiceURL:  upstream.URL,
		IncidentAIServiceURL:     upstream.URL,
		WebSocketServiceURL:      upstream.URL,
	}

	gateway, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Skipf("gateway integration dependency unavailable: %v", err)
	}
	t.Cleanup(func() { _ = gateway.Close() })

	_ = privateKeyPath
	return gateway
}

func TestInfraGenerateProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/generate" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/generate")
		}
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request body: %v", err)
		}
		if !strings.Contains(string(requestBody), `"project_slug":"demo-next"`) {
			t.Fatalf("unexpected upstream body: %s", string(requestBody))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"template":"docker-nextjs","files":[]}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase2-infra-user")

	body := strings.NewReader(`{"project_slug":"demo-next","provider":"docker","stack":{"runtime":"node","framework":"nextjs"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/infra/generate", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"template":"docker-nextjs"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestAuthGitHubProxyPath(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/auth/github" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/auth/github")
		}

		w.WriteHeader(http.StatusTemporaryRedirect)
		_, _ = w.Write([]byte("redirect"))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusTemporaryRedirect)
	}
}

func TestCORSPreflightAllowedForDashboardOrigin(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/me", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "authorization,content-type")

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected allow origin header: %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestPipelineGenerateProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/generate" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/generate")
		}
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request body: %v", err)
		}
		if !strings.Contains(string(requestBody), `"project_slug":"demo-next-pipeline"`) {
			t.Fatalf("unexpected upstream body: %s", string(requestBody))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"template":"github-actions-nextjs","files":[]}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase2-pipeline-user")

	body := strings.NewReader(`{"project_slug":"demo-next-pipeline","provider":"github-actions","stack":{"runtime":"node","framework":"nextjs"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/generate", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"template":"github-actions-nextjs"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDeploymentStartProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/deploy" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/deploy")
		}
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request body: %v", err)
		}
		if !strings.Contains(string(requestBody), `"repo_id":"repo-1"`) {
			t.Fatalf("unexpected upstream body: %s", string(requestBody))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"dep-1","status":"deploying"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase2-deploy-user")

	body := strings.NewReader(`{"repo_id":"repo-1","commit_sha":"sha-123","branch":"main","environment":"production","image_tag":"ghcr.io/acme/app:sha-123"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/deploy", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusAccepted)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"deploying"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDeploymentRollbackProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/deployments/dep-1/rollback" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/deployments/dep-1/rollback")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"dep-1","status":"rolled_back","current_live_deployment_id":"dep-0"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase2-rollback-user")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/deployments/dep-1/rollback", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"rolled_back"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDeploymentListProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/deployments" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/deployments")
		}
		if !strings.Contains(r.URL.RawQuery, "project_id=project-1") {
			t.Fatalf("unexpected upstream query: %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"dep-1","status":"live","environment":"production"}]`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase3-deploy-list-user")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/deployments?project_id=project-1&limit=10", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"dep-1"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestObservabilitySnapshotsProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/snapshots" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/snapshots")
		}
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request body: %v", err)
		}
		if !strings.Contains(string(requestBody), `"project_id":"project-1"`) {
			t.Fatalf("unexpected upstream body: %s", string(requestBody))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"snapshot":{"id":"snap-1"},"alerts_fired":[]}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase3-observability-user")
	body := strings.NewReader(`{"project_id":"project-1","cpu_pct":92,"memory_pct":40,"req_per_sec":10,"error_rate_pct":6,"p99_latency_ms":300,"pod_count":2,"ready_pod_count":2}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/observability/snapshots", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestObservabilityAlertsProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/alerts/project-1" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/alerts/project-1")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"alert-1","status":"open"}]`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase3-alerts-user")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/observability/alerts/project-1", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
}

func TestIncidentListProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/projects/project-1" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/projects/project-1")
		}
		if !strings.Contains(r.URL.RawQuery, "limit=5") || !strings.Contains(r.URL.RawQuery, "offset=0") {
			t.Fatalf("unexpected upstream query: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"id":"incident-1","project_id":"project-1","alert_id":"alert-1","created_at":"2026-03-13T12:00:00Z","resolved_at":null,"ai_diagnosis":{"root_cause":"high latency after deploy","confidence":0.92,"reasoning":"p99 breached after rollout","recommended_actions":[{"action":"scale_pods","params":{"replicas":3}}],"auto_execute":false},"ai_actions":[]}],"total":1,"limit":5,"offset":0}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase3-incidents-user")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/projects/project-1?limit=5&offset=0", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}

	var incidentsPage struct {
		Items []struct {
			ID        string `json:"id"`
			ProjectID string `json:"project_id"`
			AlertID   string `json:"alert_id"`
			CreatedAt string `json:"created_at"`
			Diagnosis struct {
				RootCause  string  `json:"root_cause"`
				Confidence float64 `json:"confidence"`
			} `json:"ai_diagnosis"`
		} `json:"items"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &incidentsPage); err != nil {
		t.Fatalf("decode incidents response failed: %v", err)
	}
	if incidentsPage.Total != 1 || incidentsPage.Limit != 5 || incidentsPage.Offset != 0 {
		t.Fatalf("unexpected incidents envelope: %+v", incidentsPage)
	}
	if len(incidentsPage.Items) != 1 || incidentsPage.Items[0].ID != "incident-1" {
		t.Fatalf("unexpected incidents items payload: %+v", incidentsPage.Items)
	}
	if incidentsPage.Items[0].ProjectID != "project-1" || incidentsPage.Items[0].AlertID != "alert-1" {
		t.Fatalf("unexpected incidents identifiers payload: %+v", incidentsPage.Items[0])
	}
	if incidentsPage.Items[0].CreatedAt == "" {
		t.Fatalf("expected created_at in incidents payload: %+v", incidentsPage.Items[0])
	}
	if incidentsPage.Items[0].Diagnosis.RootCause == "" || incidentsPage.Items[0].Diagnosis.Confidence <= 0 {
		t.Fatalf("unexpected diagnosis payload: %+v", incidentsPage.Items[0].Diagnosis)
	}
}

func TestIncidentActionProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/incident-1/actions" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/incident-1/actions")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"incident_id":"incident-1","status":"accepted"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase3-incident-action-user")
	body := strings.NewReader(`{"action":"restart_pods","params":{}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/incident-1/actions", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
}

func TestOrgsCreateProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/orgs" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/orgs")
		}
		if strings.TrimSpace(r.Header.Get("X-Helmix-Org-ID")) == "" {
			t.Fatal("expected X-Helmix-Org-ID header forwarded to upstream")
		}
		if strings.TrimSpace(r.Header.Get("X-Helmix-Role")) == "" {
			t.Fatal("expected X-Helmix-Role header forwarded to upstream")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"org-new","name":"Acme","slug":"acme"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase4-create-org-user")

	body := strings.NewReader(`{"name":"Acme"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orgs", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusCreated)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"org-new"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestOrgsMembersProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/orgs/members" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/orgs/members")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"members":[],"org_id":"phase1-org"}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase4-list-members-user")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/members", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"org_id"`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestOrgsUnauthenticatedRejected(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/members", nil)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSecretsCreateProxyAuthorized(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected upstream method: got %s want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/secrets" {
			t.Fatalf("unexpected upstream path: got %q want %q", r.URL.Path, "/secrets")
		}
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request body: %v", err)
		}
		if !strings.Contains(string(requestBody), `"service":"deployment-engine"`) {
			t.Fatalf("unexpected upstream body: %s", string(requestBody))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"deployment-engine","key":"registry_token","value":"abc","version":1}`))
	})

	gateway := newTestGatewayWithUpstream(t, upstream)
	token := signTestJWT(t, gateway.config.JWTPublicKeyPath, "phase4-secrets-user")

	body := strings.NewReader(`{"service":"deployment-engine","key":"registry_token","value":"abc"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"version":1`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestSecretsUnauthenticatedRejected(t *testing.T) {
	gateway := newTestGateway(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/deployment-engine/registry_token", nil)

	gateway.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func signTestJWT(t *testing.T, publicKeyPath, userID string) string {
	t.Helper()

	privateKeyPath := strings.TrimSuffix(publicKeyPath, "public.pem") + "private.pem"
	token, err := sharedauth.SignUserToken(privateKeyPath, sharedauth.User{
		UserID:         userID,
		OrgID:          "phase1-org",
		Role:           "owner",
		Email:          "phase1@example.com",
		GitHubUsername: "phase1-gh",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}
	return token
}

func writeTestKeys(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa private key failed: %v", err)
	}

	privateKeyPath := t.TempDir() + "/jwt-private.pem"
	publicKeyPath := strings.TrimSuffix(privateKeyPath, "private.pem") + "public.pem"

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

func TestRequestIDRespectedWhenProvided(t *testing.T) {
	gateway := newTestGateway(t)

	const providedRequestID = "phase1-provided-request-id"
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", providedRequestID)
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Header().Get("X-Request-ID") != providedRequestID {
		t.Fatalf("expected gateway to keep provided request id %q, got %q", providedRequestID, recorder.Header().Get("X-Request-ID"))
	}
}

func TestExpiredJWTRejected(t *testing.T) {
	gateway := newTestGateway(t)

	expiredToken := signExpiredTestJWT(t, gateway.config.JWTPublicKeyPath)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/projects", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	gateway.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func signExpiredTestJWT(t *testing.T, publicKeyPath string) string {
	t.Helper()
	privateKeyPath := strings.TrimSuffix(publicKeyPath, "public.pem") + "private.pem"
	token, err := sharedauth.SignUserToken(privateKeyPath, sharedauth.User{
		UserID:         fmt.Sprintf("expired-%d", time.Now().UnixNano()),
		OrgID:          "phase1-org",
		Role:           "owner",
		Email:          "phase1@example.com",
		GitHubUsername: "phase1-gh",
	}, -time.Minute)
	if err != nil {
		t.Fatalf("sign expired jwt failed: %v", err)
	}
	return token
}
