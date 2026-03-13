package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	sharedauth "github.com/your-org/helmix/libs/auth"

	"github.com/your-org/helmix/services/auth-service/internal/config"
	githubclient "github.com/your-org/helmix/services/auth-service/internal/github"
	"github.com/your-org/helmix/services/auth-service/internal/security"
	"github.com/your-org/helmix/services/auth-service/internal/session"
	"github.com/your-org/helmix/services/auth-service/internal/store"
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

type Server struct {
	config       config.Config
	logger       *slog.Logger
	githubClient *githubclient.Client
	store        *store.Store
	sessions     *session.Store
	router       chi.Router
}

// New wires the auth-service HTTP router.
func New(cfg config.Config, logger *slog.Logger, githubClient *githubclient.Client, store *store.Store, sessions *session.Store) *Server {
	srv := &Server{
		config:       cfg,
		logger:       logger,
		githubClient: githubClient,
		store:        store,
		sessions:     sessions,
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
		s.writeError(w, http.StatusInternalServerError, "load_user_failed", fmt.Errorf("load current user: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]userResponseBody{
		"user": toUserResponse(userRecord),
	})
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
