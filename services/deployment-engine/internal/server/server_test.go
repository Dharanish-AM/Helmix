package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/your-org/helmix/services/deployment-engine/internal/deploy"
)

func TestHealthEndpoint(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeEngine{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDeployEndpointReturnsAccepted(t *testing.T) {
	engine := &fakeEngine{
		startResponse: deploy.DeploymentResponse{ID: "dep-1", Status: "deploying"},
	}
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), engine)

	payload := map[string]any{
		"repo_id":          "repo-1",
		"commit_sha":       "sha-123",
		"branch":           "main",
		"scan_results":     map[string]any{"critical": 1, "high": 0},
		"accept_risk":      true,
		"environment":      "production",
		"image_tag":        "ghcr.io/acme/app:sha-123",
		"simulate_failure": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Helmix-Org-ID", "org-1")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
	if engine.lastOrgID != "org-1" {
		t.Fatalf("unexpected org id: got %s want %s", engine.lastOrgID, "org-1")
	}
	if !engine.lastStartRequest.AcceptRisk {
		t.Fatal("expected accept_risk=true to be passed to engine")
	}
	if engine.lastStartRequest.ScanResults["critical"] != float64(1) {
		t.Fatalf("unexpected scan_results payload: %+v", engine.lastStartRequest.ScanResults)
	}
}

func TestGetDeploymentNotFoundReturns404(t *testing.T) {
	engine := &fakeEngine{getErr: deploy.ErrNotFound}
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/deployments/dep-missing", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestListDeploymentsByProjectReturnsOK(t *testing.T) {
	engine := &fakeEngine{
		listResponse: []deploy.DeploymentResponse{
			{ID: "dep-1", Status: "live", RepoID: "repo-1", CreatedAt: time.Now().UTC()},
		},
	}
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/deployments?project_id=project-1&limit=5", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if engine.lastProjectID != "project-1" {
		t.Fatalf("unexpected project id: got %s want %s", engine.lastProjectID, "project-1")
	}
	if engine.lastLimit != 5 {
		t.Fatalf("unexpected limit: got %d want %d", engine.lastLimit, 5)
	}
}

func TestRollbackEndpointReturnsConflict(t *testing.T) {
	engine := &fakeEngine{rollbackErr: deploy.ErrRollbackUnavailable}
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deployments/dep-1/rollback", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

type fakeEngine struct {
	startResponse    deploy.DeploymentResponse
	getResponse      deploy.DeploymentResponse
	listResponse     []deploy.DeploymentResponse
	rollbackResponse deploy.DeploymentResponse
	startErr         error
	getErr           error
	listErr          error
	rollbackErr      error
	lastOrgID        string
	lastStartRequest deploy.StartRequest
	lastProjectID    string
	lastLimit        int
}

func (f *fakeEngine) StartDeployment(_ context.Context, orgID string, request deploy.StartRequest) (deploy.DeploymentResponse, error) {
	f.lastOrgID = orgID
	f.lastStartRequest = request
	return f.startResponse, f.startErr
}

func (f *fakeEngine) GetDeployment(_ context.Context, _ string) (deploy.DeploymentResponse, error) {
	if f.getErr != nil {
		return deploy.DeploymentResponse{}, f.getErr
	}
	if f.getResponse.ID == "" {
		return deploy.DeploymentResponse{ID: "dep-1", Status: "live", CreatedAt: time.Now().UTC()}, nil
	}
	return f.getResponse, nil
}

func (f *fakeEngine) ListDeploymentsByProject(_ context.Context, projectID string, limit int) ([]deploy.DeploymentResponse, error) {
	f.lastProjectID = projectID
	f.lastLimit = limit
	if f.listErr != nil {
		return nil, f.listErr
	}
	if len(f.listResponse) == 0 {
		return []deploy.DeploymentResponse{}, nil
	}
	return f.listResponse, nil
}

func (f *fakeEngine) RollbackDeployment(_ context.Context, orgID, _ string) (deploy.DeploymentResponse, error) {
	f.lastOrgID = orgID
	if f.rollbackErr != nil {
		return deploy.DeploymentResponse{}, f.rollbackErr
	}
	if f.rollbackResponse.ID == "" {
		return deploy.DeploymentResponse{ID: "dep-1", Status: "rolled_back", CreatedAt: time.Now().UTC()}, nil
	}
	return f.rollbackResponse, nil
}

var _ Engine = (*fakeEngine)(nil)

var _ = errors.New
