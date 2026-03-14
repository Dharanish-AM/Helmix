package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	sharedauth "github.com/your-org/helmix/libs/auth"

	"github.com/your-org/helmix/services/auth-service/internal/config"
	githubclient "github.com/your-org/helmix/services/auth-service/internal/github"
	"github.com/your-org/helmix/services/auth-service/internal/security"
	"github.com/your-org/helmix/services/auth-service/internal/session"
	"github.com/your-org/helmix/services/auth-service/internal/store"
	vaultclient "github.com/your-org/helmix/services/auth-service/internal/vault"
)

type responseError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type authResponse struct {
	Token        string           `json:"token"`
	RefreshToken string           `json:"refresh_token,omitempty"`
	User         userResponseBody `json:"user"`
}

type userResponseBody struct {
	ID             string    `json:"id"`
	GitHubID       int64     `json:"github_id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	AvatarURL      string    `json:"avatar_url"`
	OrgID          string    `json:"org_id"`
	OrgName        string    `json:"org_name"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	TokenUpdatedAt time.Time `json:"token_updated_at"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type githubRepoResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	UpdatedAt     string `json:"updated_at"`
}

type githubReposResponse struct {
	Items []githubRepoResponse `json:"items"`
}

type triggerRepoAnalysisRequest struct {
	RepoID     string `json:"repo_id"`
	GitHubRepo string `json:"github_repo"`
}

type repoAnalyzeProxyRequest struct {
	RepoURL     string `json:"repo_url"`
	GitHubToken string `json:"github_token"`
	RepoID      string `json:"repo_id"`
}

type createOrgRequest struct {
	Name string `json:"name"`
}

type createOrgResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteResponse struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type acceptInviteRequest struct {
	Token string `json:"token"`
}

type memberResponse struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

type upsertSecretRequest struct {
	Service string `json:"service"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
}

type secretResponse struct {
	Service string `json:"service"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Version int    `json:"version"`
}

var validSecretSegment = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Server struct {
	config       config.Config
	logger       *slog.Logger
	githubClient *githubclient.Client
	store        *store.Store
	sessions     *session.Store
	vaultClient  vaultclient.SecretClient
	router       chi.Router
}

// New wires the auth-service HTTP router.
func New(cfg config.Config, logger *slog.Logger, githubClient *githubclient.Client, store *store.Store, sessions *session.Store, vaultClient vaultclient.SecretClient) *Server {
	srv := &Server{
		config:       cfg,
		logger:       logger,
		githubClient: githubClient,
		store:        store,
		sessions:     sessions,
		vaultClient:  vaultClient,
	}
	srv.router = srv.buildRouter()
	return srv
}

// Handler returns the HTTP handler for auth-service.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(s.loggingMiddleware)

	router.Get("/health", s.handleHealth)
	router.Get("/auth/github", s.handleGitHubRedirect)
	router.Get("/auth/github/callback", s.handleGitHubCallback)

	router.Group(func(protected chi.Router) {
		protected.Use(sharedauth.JWTMiddleware(s.config.JWTPublicKeyPath))
		protected.Get("/auth/me", s.handleMe)
		protected.Get("/auth/github/repos", s.handleGitHubRepos)
		protected.Post("/auth/repos/analyze", s.handleTriggerRepoAnalysis)

		// Org management — any authenticated user may create an org or accept an invite.
		protected.Post("/orgs", s.handleCreateOrg)
		protected.Post("/orgs/accept-invite", s.handleAcceptInvite)
		protected.Get("/orgs/members", s.handleListMembers)

		// Invite and role management — restricted to owner/admin.
		protected.Group(func(ownerAdmin chi.Router) {
			ownerAdmin.Use(sharedauth.RequireRole("owner", "admin"))
			ownerAdmin.Post("/orgs/invite", s.handleInvite)
			ownerAdmin.Delete("/orgs/members/{user_id}", s.handleRemoveMember)
			ownerAdmin.Post("/secrets", s.handleUpsertSecret)
			ownerAdmin.Get("/secrets/{service}/{key}", s.handleGetSecret)
			ownerAdmin.Delete("/secrets/{service}/{key}", s.handleDeleteSecret)
		})

		// Role update — owner only to prevent privilege escalation.
		protected.Group(func(ownerOnly chi.Router) {
			ownerOnly.Use(sharedauth.RequireRole("owner"))
			ownerOnly.Patch("/orgs/members/{user_id}", s.handleUpdateMemberRole)
		})
	})

	router.Post("/auth/refresh", s.handleRefresh)
	router.Post("/auth/logout", s.handleLogout)

	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "auth-service",
		"version": "0.1.0",
	})
}

func (s *Server) handleGitHubRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "generate_oauth_state", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.config.OAuthStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.CookieSecure,
		MaxAge:   int((10 * time.Minute).Seconds()),
	})

	http.Redirect(w, r, s.githubClient.AuthorizeURL(state), http.StatusTemporaryRedirect)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(s.config.OAuthStateCookieName)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "missing_oauth_state", fmt.Errorf("read oauth state cookie: %w", err))
		return
	}
	if stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		s.writeError(w, http.StatusBadRequest, "invalid_oauth_state", fmt.Errorf("oauth state mismatch"))
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		s.writeError(w, http.StatusBadRequest, "missing_code", fmt.Errorf("oauth callback missing code"))
		return
	}

	ctx := r.Context()
	accessToken, err := s.githubClient.ExchangeCode(ctx, code)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "oauth_exchange_failed", fmt.Errorf("exchange github code: %w", err))
		return
	}

	githubUser, err := s.githubClient.FetchUser(ctx, accessToken)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "github_user_failed", fmt.Errorf("fetch github user: %w", err))
		return
	}

	nonce, ciphertext, err := security.Encrypt(s.config.TokenEncryptionKey, accessToken)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "encrypt_token_failed", fmt.Errorf("encrypt github access token: %w", err))
		return
	}

	userRecord, err := s.store.UpsertUserWithOrg(ctx, store.UpsertUserParams{
		GitHubID:        githubUser.ID,
		Username:        githubUser.Login,
		Email:           githubUser.Email,
		AvatarURL:       githubUser.AvatarURL,
		TokenNonce:      nonce,
		TokenCiphertext: ciphertext,
		TokenUpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "upsert_user_failed", fmt.Errorf("create or update user: %w", err))
		return
	}

	identity := sharedauth.User{
		UserID:         userRecord.ID,
		OrgID:          userRecord.OrgID,
		Role:           userRecord.Role,
		Email:          userRecord.Email,
		GitHubUsername: userRecord.Username,
	}
	jwToken, refreshToken, err := s.issueTokens(ctx, identity)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "issue_tokens_failed", fmt.Errorf("issue tokens: %w", err))
		return
	}

	s.setRefreshCookie(w, refreshToken)
	clearCookie(w, s.config.OAuthStateCookieName, s.config.CookieSecure)

	redirectURL, err := url.Parse(s.config.DashboardURL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "invalid_dashboard_url", fmt.Errorf("parse dashboard url: %w", err))
		return
	}
	redirectURL.Path = "/dashboard"
	query := redirectURL.Query()
	query.Set("token", jwToken)
	redirectURL.RawQuery = query.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		refreshToken = readRefreshCookie(r, s.config.RefreshCookieName)
	}
	if refreshToken == "" {
		s.writeError(w, http.StatusBadRequest, "missing_refresh_token", fmt.Errorf("refresh token is required"))
		return
	}

	identity, rotatedRefreshToken, err := s.sessions.Rotate(r.Context(), refreshToken)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid_refresh_token", fmt.Errorf("rotate refresh token: %w", err))
		return
	}

	jwtToken, err := sharedauth.SignUserToken(s.config.JWTPrivateKeyPath, identity, s.config.JWTTTL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "sign_jwt_failed", fmt.Errorf("sign jwt: %w", err))
		return
	}

	userRecord, err := s.store.GetUserByID(r.Context(), identity.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeError(w, http.StatusUnauthorized, "invalid_user", fmt.Errorf("refresh token user no longer exists: %w", err))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "load_user_failed", fmt.Errorf("load user after refresh: %w", err))
		return
	}

	s.setRefreshCookie(w, rotatedRefreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        jwtToken,
		RefreshToken: rotatedRefreshToken,
		User:         toUserResponse(userRecord),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	userRecord, err := s.store.GetUserByID(r.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeError(w, http.StatusUnauthorized, "invalid_user", fmt.Errorf("authenticated user no longer exists: %w", err))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "load_user_failed", fmt.Errorf("load current user: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]userResponseBody{
		"user": toUserResponse(userRecord),
	})
}

func (s *Server) handleGitHubRepos(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	tokenRecord, err := s.store.GetGitHubTokenByUserID(r.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeError(w, http.StatusUnauthorized, "invalid_user", fmt.Errorf("authenticated user no longer exists: %w", err))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "load_github_token_failed", fmt.Errorf("load encrypted github token: %w", err))
		return
	}
	if len(tokenRecord.Nonce) == 0 || len(tokenRecord.Ciphertext) == 0 {
		s.writeError(w, http.StatusUnauthorized, "github_token_missing", fmt.Errorf("github token not available for user"))
		return
	}

	accessToken, err := security.Decrypt(s.config.TokenEncryptionKey, tokenRecord.Nonce, tokenRecord.Ciphertext)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "decrypt_token_failed", fmt.Errorf("decrypt github token: %w", err))
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			s.writeError(w, http.StatusBadRequest, "invalid_limit", fmt.Errorf("limit must be a positive integer"))
			return
		}
		if parsedLimit > 200 {
			parsedLimit = 200
		}
		limit = parsedLimit
	}

	repos, err := s.githubClient.ListRepositories(r.Context(), accessToken, limit)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "github_repos_failed", fmt.Errorf("list github repositories: %w", err))
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	items := make([]githubRepoResponse, 0, len(repos))
	for _, repo := range repos {
		if search != "" {
			name := strings.ToLower(repo.Name)
			fullName := strings.ToLower(repo.FullName)
			if !strings.Contains(name, search) && !strings.Contains(fullName, search) {
				continue
			}
		}
		items = append(items, githubRepoResponse{
			ID:            repo.ID,
			Name:          repo.Name,
			FullName:      repo.FullName,
			Private:       repo.Private,
			DefaultBranch: repo.DefaultBranch,
			UpdatedAt:     repo.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, githubReposResponse{Items: items})
}

func (s *Server) handleTriggerRepoAnalysis(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	var request triggerRepoAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode analyze request: %w", err))
		return
	}
	repoID := strings.TrimSpace(request.RepoID)
	githubRepo := strings.TrimSpace(strings.ToLower(request.GitHubRepo))
	if repoID == "" || githubRepo == "" || !strings.Contains(githubRepo, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("repo_id and github_repo (owner/name) are required"))
		return
	}

	tokenRecord, err := s.store.GetGitHubTokenByUserID(r.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeError(w, http.StatusUnauthorized, "invalid_user", fmt.Errorf("authenticated user no longer exists: %w", err))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "load_github_token_failed", fmt.Errorf("load encrypted github token: %w", err))
		return
	}

	accessToken, err := security.Decrypt(s.config.TokenEncryptionKey, tokenRecord.Nonce, tokenRecord.Ciphertext)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "decrypt_token_failed", fmt.Errorf("decrypt github token: %w", err))
		return
	}

	body, err := json.Marshal(repoAnalyzeProxyRequest{
		RepoURL:     "https://github.com/" + githubRepo + ".git",
		GitHubToken: accessToken,
		RepoID:      repoID,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "marshal_request_failed", fmt.Errorf("marshal repo analyze request: %w", err))
		return
	}

	upstreamRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(s.config.RepoAnalyzerURL, "/")+"/analyze", bytes.NewReader(body))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "build_request_failed", fmt.Errorf("create repo-analyzer request: %w", err))
		return
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	upstreamRequest.Header.Set("X-Helmix-Org-ID", user.OrgID)

	response, err := (&http.Client{Timeout: s.config.HTTPClientTimeout}).Do(upstreamRequest)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "analysis_failed", fmt.Errorf("call repo-analyzer: %w", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		s.writeError(w, http.StatusBadGateway, "analysis_failed", fmt.Errorf("repo-analyzer returned status %d", response.StatusCode))
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		s.writeError(w, http.StatusBadGateway, "analysis_failed", fmt.Errorf("decode repo-analyzer response: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		refreshToken = readRefreshCookie(r, s.config.RefreshCookieName)
	}
	if err := s.sessions.Delete(r.Context(), refreshToken); err != nil {
		s.writeError(w, http.StatusInternalServerError, "logout_failed", fmt.Errorf("delete refresh token: %w", err))
		return
	}
	clearCookie(w, s.config.RefreshCookieName, s.config.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) issueTokens(ctx context.Context, user sharedauth.User) (string, string, error) {
	jwtToken, err := sharedauth.SignUserToken(s.config.JWTPrivateKeyPath, user, s.config.JWTTTL)
	if err != nil {
		return "", "", fmt.Errorf("sign jwt: %w", err)
	}
	refreshToken, err := s.sessions.Create(ctx, user)
	if err != nil {
		return "", "", fmt.Errorf("create refresh token: %w", err)
	}
	return jwtToken, refreshToken, nil
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.config.RefreshCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.CookieSecure,
		MaxAge:   int(s.config.RefreshTTL.Seconds()),
	})
}

func readRefreshCookie(r *http.Request, cookieName string) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(writer, r)
		s.logger.Info("auth request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", writer.statusCode),
			slog.Duration("latency", time.Since(startedAt)),
		)
	})
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, code string, err error) {
	s.logger.Error("auth request failed", slog.String("code", code), slog.String("error", err.Error()))
	writeJSON(w, statusCode, responseError{Error: http.StatusText(statusCode), Code: code})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func toUserResponse(userRecord store.UserRecord) userResponseBody {
	return userResponseBody{
		ID:             userRecord.ID,
		GitHubID:       userRecord.GitHubID,
		Username:       userRecord.Username,
		Email:          userRecord.Email,
		AvatarURL:      userRecord.AvatarURL,
		OrgID:          userRecord.OrgID,
		OrgName:        userRecord.OrgName,
		Role:           userRecord.Role,
		CreatedAt:      userRecord.CreatedAt,
		TokenUpdatedAt: userRecord.TokenUpdatedAt,
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func ShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// ── Org management handlers ──────────────────────────────────────────────────

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	var req createOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode create org request: %w", err))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("name is required"))
		return
	}

	org, err := s.store.CreateOrg(r.Context(), user.UserID, name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "create_org_failed", fmt.Errorf("create org: %w", err))
		return
	}

	writeJSON(w, http.StatusCreated, createOrgResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		OwnerID:   org.OwnerID,
		CreatedAt: org.CreatedAt,
	})
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	var req inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode invite request: %w", err))
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("email is required"))
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "developer"
	}
	validRoles := map[string]bool{"owner": true, "admin": true, "developer": true, "viewer": true}
	if !validRoles[role] {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("role must be one of: owner, admin, developer, viewer"))
		return
	}

	invite, err := s.store.CreateInvite(r.Context(), user.OrgID, email, role, user.UserID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "invite_failed", fmt.Errorf("create invite: %w", err))
		return
	}

	writeJSON(w, http.StatusCreated, inviteResponse{
		ID:        invite.ID,
		OrgID:     invite.OrgID,
		Email:     invite.Email,
		Role:      invite.Role,
		Token:     invite.Token,
		ExpiresAt: invite.ExpiresAt,
	})
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode accept invite request: %w", err))
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("token is required"))
		return
	}

	orgID, err := s.store.AcceptInvite(r.Context(), token, user.UserID)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "expired") || strings.Contains(errMsg, "already accepted") {
			s.writeError(w, http.StatusBadRequest, "invalid_invite", err)
			return
		}
		s.writeError(w, http.StatusInternalServerError, "accept_invite_failed", fmt.Errorf("accept invite: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"org_id": orgID, "status": "joined"})
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	members, err := s.store.GetOrgMembers(r.Context(), user.OrgID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "list_members_failed", fmt.Errorf("list org members: %w", err))
		return
	}

	items := make([]memberResponse, 0, len(members))
	for _, m := range members {
		items = append(items, memberResponse{
			UserID:    m.UserID,
			Username:  m.Username,
			Email:     m.Email,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"members": items, "org_id": user.OrgID})
}

func (s *Server) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	targetUserID := chi.URLParam(r, "user_id")
	if strings.TrimSpace(targetUserID) == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("user_id path parameter is required"))
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode update role request: %w", err))
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	// Owner cannot be assigned through this endpoint to prevent uncontrolled privilege escalation.
	validRoles := map[string]bool{"admin": true, "developer": true, "viewer": true}
	if !validRoles[role] {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("role must be one of: admin, developer, viewer"))
		return
	}

	if err := s.store.UpdateMemberRole(r.Context(), user.OrgID, targetUserID, role); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, "member_not_found", err)
			return
		}
		s.writeError(w, http.StatusInternalServerError, "update_role_failed", fmt.Errorf("update member role: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": targetUserID, "role": role, "status": "updated"})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	user := sharedauth.UserFromContext(r.Context())
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "missing_user", fmt.Errorf("authenticated user missing from context"))
		return
	}

	targetUserID := chi.URLParam(r, "user_id")
	if strings.TrimSpace(targetUserID) == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("user_id path parameter is required"))
		return
	}

	if err := s.store.RemoveMember(r.Context(), user.OrgID, targetUserID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, "member_not_found", err)
			return
		}
		s.writeError(w, http.StatusInternalServerError, "remove_member_failed", fmt.Errorf("remove member: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": targetUserID, "status": "removed"})
}

func (s *Server) handleUpsertSecret(w http.ResponseWriter, r *http.Request) {
	if s.vaultClient == nil {
		s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", fmt.Errorf("vault client is not configured"))
		return
	}

	var req upsertSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("decode upsert secret request: %w", err))
		return
	}
	if strings.TrimSpace(req.Service) == "" || strings.TrimSpace(req.Key) == "" || req.Value == nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service, key, and value are required"))
		return
	}
	if !validSecretSegment.MatchString(strings.TrimSpace(req.Service)) || !validSecretSegment.MatchString(strings.TrimSpace(req.Key)) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service and key may only contain letters, numbers, dashes, and underscores"))
		return
	}
	if valueString, ok := req.Value.(string); ok && len(valueString) > 4096 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("value exceeds max length of 4096 characters"))
		return
	}

	record, err := s.vaultClient.UpsertSecret(r.Context(), req.Service, req.Key, req.Value)
	if err != nil {
		if errors.Is(err, vaultclient.ErrUnavailable) {
			s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", err)
			return
		}
		s.writeError(w, http.StatusBadGateway, "vault_write_failed", err)
		return
	}

	writeJSON(w, http.StatusOK, secretResponse{
		Service: record.Service,
		Key:     record.Key,
		Value:   record.Value,
		Version: record.Version,
	})
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	if s.vaultClient == nil {
		s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", fmt.Errorf("vault client is not configured"))
		return
	}

	service := strings.TrimSpace(chi.URLParam(r, "service"))
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if service == "" || key == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service and key path parameters are required"))
		return
	}
	if !validSecretSegment.MatchString(service) || !validSecretSegment.MatchString(key) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service and key may only contain letters, numbers, dashes, and underscores"))
		return
	}

	record, err := s.vaultClient.GetSecret(r.Context(), service, key)
	if err != nil {
		if errors.Is(err, vaultclient.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, "secret_not_found", err)
			return
		}
		if errors.Is(err, vaultclient.ErrUnavailable) {
			s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", err)
			return
		}
		s.writeError(w, http.StatusBadGateway, "vault_read_failed", err)
		return
	}

	writeJSON(w, http.StatusOK, secretResponse{
		Service: record.Service,
		Key:     record.Key,
		Value:   record.Value,
		Version: record.Version,
	})
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	if s.vaultClient == nil {
		s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", fmt.Errorf("vault client is not configured"))
		return
	}

	service := strings.TrimSpace(chi.URLParam(r, "service"))
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if service == "" || key == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service and key path parameters are required"))
		return
	}
	if !validSecretSegment.MatchString(service) || !validSecretSegment.MatchString(key) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Errorf("service and key may only contain letters, numbers, dashes, and underscores"))
		return
	}

	if err := s.vaultClient.DeleteSecret(r.Context(), service, key); err != nil {
		if errors.Is(err, vaultclient.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, "secret_not_found", err)
			return
		}
		if errors.Is(err, vaultclient.ErrUnavailable) {
			s.writeError(w, http.StatusServiceUnavailable, "vault_unavailable", err)
			return
		}
		s.writeError(w, http.StatusBadGateway, "vault_delete_failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"service": service, "key": key, "status": "deleted"})
}
