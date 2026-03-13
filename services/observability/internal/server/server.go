package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/your-org/helmix/services/observability/internal/observability"
	"github.com/your-org/helmix/services/observability/internal/store"
)

type Service interface {
	IngestSnapshot(ctx context.Context, request observability.SnapshotRequest) (observability.SnapshotResponse, error)
	ListSnapshots(ctx context.Context, projectID string) ([]store.MetricSnapshot, error)
	CurrentSnapshot(ctx context.Context, projectID string) (store.MetricSnapshot, error)
	OpenAlerts(ctx context.Context, projectID string) ([]store.Alert, error)
}

type Server struct {
	logger  *slog.Logger
	service Service
	router  chi.Router
}

func New(logger *slog.Logger, service Service) *Server {
	server := &Server{logger: logger, service: service}
	server.router = server.buildRouter()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(s.loggingMiddleware)
	router.Get("/health", s.handleHealth)
	router.Post("/snapshots", s.handleIngestSnapshot)
	router.Get("/metrics/{project_id}", s.handleListSnapshots)
	router.Get("/metrics/{project_id}/current", s.handleCurrentSnapshot)
	router.Get("/alerts/{project_id}", s.handleOpenAlerts)
	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "observability",
		"version": "0.2.0",
	})
}

func (s *Server) handleIngestSnapshot(w http.ResponseWriter, r *http.Request) {
	var request observability.SnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode request: %w", err))
		return
	}

	response, err := s.service.IngestSnapshot(r.Context(), request)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.ListSnapshots(r.Context(), chi.URLParam(r, "project_id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCurrentSnapshot(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.CurrentSnapshot(r.Context(), chi.URLParam(r, "project_id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOpenAlerts(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.OpenAlerts(r.Context(), chi.URLParam(r, "project_id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "not_found", err)
		return
	}
	writeJSONError(w, http.StatusBadRequest, "invalid_request", err)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("observability request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.statusCode),
			slog.Duration("latency", time.Since(startedAt)),
		)
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, statusCode int, code string, err error) {
	writeJSON(w, statusCode, map[string]string{
		"error":  http.StatusText(statusCode),
		"code":   code,
		"detail": err.Error(),
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}