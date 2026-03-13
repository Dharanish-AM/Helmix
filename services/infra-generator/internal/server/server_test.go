package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGenerateNextJSSucceeds(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := map[string]any{
		"project_slug": "demo-next",
		"provider":     "docker",
		"stack": map[string]any{
			"runtime":   "node",
			"framework": "nextjs",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Template string `json:"template"`
		Files    []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Template != "docker-nextjs" {
		t.Fatalf("expected template %q, got %q", "docker-nextjs", response.Template)
	}
	if len(response.Files) == 0 {
		t.Fatal("expected generated files in response")
	}
}

func TestGenerateUnsupportedStackReturns422(t *testing.T) {
	srv := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := map[string]any{
		"project_slug": "demo-go",
		"provider":     "docker",
		"stack": map[string]any{
			"runtime":   "go",
			"framework": "gin",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}
}
