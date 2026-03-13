package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/your-org/helmix/services/repo-analyzer/internal/analyzer"
	"github.com/your-org/helmix/services/repo-analyzer/internal/classifier"
	"github.com/your-org/helmix/services/repo-analyzer/internal/config"
	"github.com/your-org/helmix/services/repo-analyzer/internal/gitclone"
	"github.com/your-org/helmix/services/repo-analyzer/internal/store"
)

type analyzeRequest struct {
	RepoURL     string `json:"repo_url"`
	GitHubToken string `json:"github_token"`
	RepoID      string `json:"repo_id"`
}

type analyzeResponse struct {
	RepoID string          `json:"repo_id"`
	Result analyzer.Result `json:"result"`
}

type connectRepoRequest struct {
	GitHubRepo    string `json:"github_repo"`
	DefaultBranch string `json:"default_branch"`
}

type connectedReposResponse struct {
	Items []store.ConnectedRepo `json:"items"`
}

type Server struct {
	config     config.Config
	logger     *slog.Logger
	store      *store.Store
	natsClient *nats.Conn
	classifier *classifier.Client
	router     chi.Router
}

// New constructs the repo-analyzer HTTP server.
func New(cfg config.Config, logger *slog.Logger, store *store.Store, natsClient *nats.Conn) *Server {
	server := &Server{
		config:     cfg,
		logger:     logger,
		store:      store,
		natsClient: natsClient,
		classifier: classifier.New(cfg.IncidentAIClassifyURL, cfg.HTTPClientTimeout),
	}
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
	router.Get("/projects", s.handleListProjects)
	router.Post("/projects", s.handleConnectRepo)
	router.Post("/analyze", s.handleAnalyze)
	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "repo-analyzer", "version": "0.1.0"})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var request analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode analyze request: %w", err))
		return
	}
	if request.RepoURL == "" || request.RepoID == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("repo_url and repo_id are required"))
		return
	}

	metadata, err := s.store.GetRepoMetadata(r.Context(), request.RepoID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "repo_not_found", fmt.Errorf("load repo metadata: %w", err))
		return
	}

	clonePath, err := gitclone.Clone(r.Context(), s.config.GitBinary, s.config.CloneBaseDir, request.RepoURL, request.GitHubToken, request.RepoID)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "clone_failed", fmt.Errorf("clone repository: %w", err))
		return
	}
	defer os.RemoveAll(clonePath)

	result, err := analyzer.DetectRepository(clonePath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "detect_failed", fmt.Errorf("detect repository stack: %w", err))
		return
	}

	if result.Confidence < 0.70 {
		enrichedResult, enrichErr := s.classifier.Enrich(r.Context(), result)
		if enrichErr != nil {
			s.logger.Warn("fallback classification failed", slog.String("repo_id", request.RepoID), slog.String("error", enrichErr.Error()))
		} else {
			result = enrichedResult
		}
	}

	if err := s.store.UpdateDetectedStack(r.Context(), request.RepoID, result); err != nil {
		s.writeError(w, http.StatusInternalServerError, "persist_failed", fmt.Errorf("persist detected stack: %w", err))
		return
	}

	event := eventsdk.RepoAnalyzedEvent{
		BaseEvent: eventsdk.BaseEvent{
			ID:        randomID(),
			Type:      string(eventsdk.RepoAnalyzed),
			OrgID:     metadata.OrgID,
			ProjectID: metadata.ProjectID,
			CreatedAt: time.Now().UTC(),
		},
		RepoID: request.RepoID,
		Stack:  result.Stack,
	}
	if err := eventsdk.Publish(s.natsClient, event); err != nil {
		s.writeError(w, http.StatusInternalServerError, "publish_failed", fmt.Errorf("publish repo.analyzed event: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, analyzeResponse{RepoID: request.RepoID, Result: result})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(r.Header.Get("X-Helmix-Org-ID"))
	if orgID == "" {
		s.writeError(w, http.StatusUnauthorized, "missing_org_context", fmt.Errorf("missing X-Helmix-Org-ID header"))
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_limit", fmt.Errorf("limit must be an integer: %w", err))
			return
		}
		limit = parsedLimit
	}

	items, err := s.store.ListConnectedRepos(r.Context(), orgID, r.URL.Query().Get("q"), limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "list_repos_failed", fmt.Errorf("list connected repos: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, connectedReposResponse{Items: items})
}

func (s *Server) handleConnectRepo(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(r.Header.Get("X-Helmix-Org-ID"))
	if orgID == "" {
		s.writeError(w, http.StatusUnauthorized, "missing_org_context", fmt.Errorf("missing X-Helmix-Org-ID header"))
		return
	}

	var request connectRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode connect repo request: %w", err))
		return
	}
	if strings.TrimSpace(request.GitHubRepo) == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("github_repo is required"))
		return
	}

	connectedRepo, err := s.store.CreateConnectedRepo(r.Context(), orgID, request.GitHubRepo, request.DefaultBranch)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "connect_repo_failed", fmt.Errorf("create connected repo: %w", err))
		return
	}

	event := map[string]any{
		"id":         randomID(),
		"type":       string(eventsdk.RepoConnected),
		"org_id":     orgID,
		"project_id": connectedRepo.ProjectID,
		"repo_id":    connectedRepo.RepoID,
		"github_repo": connectedRepo.GitHubRepo,
		"created_at": time.Now().UTC(),
	}
	if err := eventsdk.Publish(s.natsClient, event); err != nil {
		s.writeError(w, http.StatusInternalServerError, "publish_failed", fmt.Errorf("publish repo.connected event: %w", err))
		return
	}

	writeJSON(w, http.StatusCreated, connectedRepo)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("repo analyzer request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.statusCode),
			slog.Duration("latency", time.Since(startedAt)),
		)
	})
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, code string, err error) {
	s.logger.Error("repo analyzer request failed", slog.String("code", code), slog.String("error", err.Error()))
	writeJSON(w, statusCode, map[string]string{"error": http.StatusText(statusCode), "code": code})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func randomID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
