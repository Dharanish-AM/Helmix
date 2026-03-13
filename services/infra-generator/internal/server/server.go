package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/your-org/helmix/services/infra-generator/internal/generator"
)

// Server exposes infra generation endpoints.
type Server struct {
	logger *slog.Logger
	router chi.Router
}

// New constructs an infra-generator HTTP server.
func New(logger *slog.Logger) *Server {
	server := &Server{logger: logger}
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
	router.Post("/generate", s.handleGenerate)
	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "infra-generator",
		"version": "0.2.0",
	})
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var request generator.Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode request: %w", err))
		return
	}

	response, err := generator.Generate(request)
	if err != nil {
		switch {
		case errors.Is(err, generator.ErrInvalidRequest):
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err)
		case errors.Is(err, generator.ErrUnsupportedStack):
			writeJSONError(w, http.StatusUnprocessableEntity, "unsupported_stack", err)
		default:
			writeJSONError(w, http.StatusInternalServerError, "generation_failed", err)
		}
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("infra-generator request",
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
		"error": http.StatusText(statusCode),
		"code":  code,
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
