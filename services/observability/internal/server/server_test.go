package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/your-org/helmix/services/observability/internal/observability"
	"github.com/your-org/helmix/services/observability/internal/store"
)

func TestHealthEndpoint(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeService{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestIngestSnapshotAccepted(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeService{
		ingestResponse: observability.SnapshotResponse{Snapshot: store.MetricSnapshot{ID: "snap-1"}},
	})
	body, _ := json.Marshal(map[string]any{"project_id": "project-1", "cpu_pct": 91, "memory_pct": 40, "req_per_sec": 12, "error_rate_pct": 6, "p99_latency_ms": 300, "pod_count": 2, "ready_pod_count": 2})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestCurrentSnapshotNotFound(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeService{currentErr: store.ErrNotFound})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics/project-1/current", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

type fakeService struct {
	ingestResponse observability.SnapshotResponse
	ingestErr      error
	listResponse   []store.MetricSnapshot
	listErr        error
	currentResp    store.MetricSnapshot
	currentErr     error
	alertsResp     []store.Alert
	alertsErr      error
	telemetryResp  store.TelemetrySource
	telemetryErr   error
}

func (f *fakeService) IngestSnapshot(_ context.Context, _ observability.SnapshotRequest) (observability.SnapshotResponse, error) {
	return f.ingestResponse, f.ingestErr
}

func (f *fakeService) ListSnapshots(_ context.Context, _ string) ([]store.MetricSnapshot, error) {
	return f.listResponse, f.listErr
}

func (f *fakeService) CurrentSnapshot(_ context.Context, _ string) (store.MetricSnapshot, error) {
	if f.currentErr != nil {
		return store.MetricSnapshot{}, f.currentErr
	}
	if f.currentResp.ID == "" {
		return store.MetricSnapshot{ID: "snap-1", CapturedAt: time.Now().UTC()}, nil
	}
	return f.currentResp, nil
}

func (f *fakeService) OpenAlerts(_ context.Context, _ string) ([]store.Alert, error) {
	return f.alertsResp, f.alertsErr
}

func (f *fakeService) GetTelemetrySource(_ context.Context, _ string) (store.TelemetrySource, error) {
	return f.telemetryResp, f.telemetryErr
}

func (f *fakeService) UpsertTelemetrySource(_ context.Context, params store.UpsertTelemetrySourceParams) (store.TelemetrySource, error) {
	if f.telemetryErr != nil {
		return store.TelemetrySource{}, f.telemetryErr
	}
	return store.TelemetrySource{
		ProjectID:             params.ProjectID,
		SourceType:            params.SourceType,
		MetricsURL:            params.MetricsURL,
		ScrapeIntervalSeconds: params.ScrapeIntervalSeconds,
		Enabled:               params.Enabled,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}, nil
}

func (f *fakeService) ScrapeTelemetrySource(_ context.Context, _ string) (observability.SnapshotResponse, error) {
	return f.ingestResponse, f.ingestErr
}

func TestGetTelemetrySourceReturnsOK(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeService{
		telemetryResp: store.TelemetrySource{
			ProjectID:             "project-1",
			SourceType:            "helmix-json",
			MetricsURL:            "https://example.com/metrics",
			ScrapeIntervalSeconds: 30,
			Enabled:               true,
			CreatedAt:             time.Now().UTC(),
			UpdatedAt:             time.Now().UTC(),
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sources/project-1", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestUpsertTelemetrySourceReturnsOK(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeService{})
	body, _ := json.Marshal(map[string]any{
		"source_type":             "prometheus",
		"metrics_url":             "https://example.com/metrics",
		"scrape_interval_seconds": 30,
		"enabled":                 true,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sources/project-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}
