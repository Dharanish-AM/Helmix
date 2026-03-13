package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/your-org/helmix/services/deployment-engine/internal/deploy"
)

// Engine defines the deployment service contract required by the HTTP layer.
type Engine interface {
	StartDeployment(ctx context.Context, orgID string, request deploy.StartRequest) (deploy.DeploymentResponse, error)
	GetDeployment(ctx context.Context, deploymentID string) (deploy.DeploymentResponse, error)
	ListDeploymentsByProject(ctx context.Context, projectID string, limit int) ([]deploy.DeploymentResponse, error)
	RollbackDeployment(ctx context.Context, orgID, deploymentID string) (deploy.DeploymentResponse, error)
}

// Server exposes deployment-engine endpoints.
type Server struct {
	logger *slog.Logger
	engine Engine
	router chi.Router
}

// New constructs a deployment-engine HTTP server.
func New(logger *slog.Logger, engine Engine) *Server {
	server := &Server{logger: logger, engine: engine}
	server.router = server.buildRouter()
	return server
}

// Handler returns the server HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(s.loggingMiddleware)
	router.Get("/health", s.handleHealth)
	router.Post("/deploy", s.handleDeploy)
	router.Get("/deployments", s.handleListDeployments)
	router.Get("/deployments/{id}", s.handleGetDeployment)
	router.Post("/deployments/{id}/rollback", s.handleRollback)
	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "deployment-engine",
		"version": "0.2.0",
	})
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var request deploy.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode request: %w", err))
		return
	}

	response, err := s.engine.StartDeployment(r.Context(), r.Header.Get("X-Helmix-Org-ID"), request)
	if err != nil {
		s.writeEngineError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, response)
}

func (s *Server) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	response, err := s.engine.GetDeployment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.writeEngineError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsedLimit
		}
	}

	response, err := s.engine.ListDeploymentsByProject(r.Context(), projectID, limit)
	if err != nil {
		s.writeEngineError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	response, err := s.engine.RollbackDeployment(r.Context(), r.Header.Get("X-Helmix-Org-ID"), chi.URLParam(r, "id"))
	if err != nil {
		s.writeEngineError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) writeEngineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, deploy.ErrInvalidRequest):
		writeJSONError(w, http.StatusBadRequest, "invalid_request", err)
	case errors.Is(err, deploy.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "not_found", err)
	case errors.Is(err, deploy.ErrRollbackUnavailable):
		writeJSONError(w, http.StatusConflict, "rollback_unavailable", err)
	default:
		writeJSONError(w, http.StatusInternalServerError, "deployment_failed", err)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("deployment-engine request",
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