package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	sharedauth "github.com/your-org/helmix/libs/auth"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/your-org/helmix/services/api-gateway/internal/config"
)

const requestsPerMinute = 100

type contextKey string

const requestIDContextKey contextKey = "gateway-request-id"

type errorEnvelope struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// Gateway exposes the Phase 1 API gateway middleware stack and routes.
type Gateway struct {
	config      config.Config
	logger      *slog.Logger
	redisClient *redis.Client
	tracer      trace.Tracer
	router      chi.Router
}

// New constructs an API gateway instance.
func New(cfg config.Config, logger *slog.Logger) (*Gateway, error) {
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	redisClient := redis.NewClient(redisOptions)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	gateway := &Gateway{
		config:      cfg,
		logger:      logger,
		redisClient: redisClient,
		tracer:      otel.Tracer("helmix/api-gateway"),
	}
	gateway.router = gateway.buildRouter()
	return gateway, nil
}

// Handler returns the gateway HTTP handler.
func (g *Gateway) Handler() http.Handler {
	return g.router
}

// Close closes any gateway resources.
func (g *Gateway) Close() error {
	if g == nil || g.redisClient == nil {
		return nil
	}
	if err := g.redisClient.Close(); err != nil {
		return fmt.Errorf("close redis client: %w", err)
	}
	return nil
}

func (g *Gateway) buildRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(g.requestIDMiddleware)
	router.Use(g.corsMiddleware)
	router.Use(g.loggingMiddleware)
	router.Use(g.otelMiddleware)
	router.Use(g.authMiddleware)
	router.Use(g.rateLimitMiddleware)

	router.Get("/health", g.handleHealth)
	// Auth service routes keep /auth in the upstream path (prefix stripped: /api/v1).
	router.Mount("/api/v1/auth", g.proxyPrefix("/api/v1", g.config.AuthServiceURL))
	// Org management routes proxy to the auth-service (prefix stripped: /api/v1).
	router.Mount("/api/v1/orgs", g.proxyPrefix("/api/v1", g.config.AuthServiceURL))
	router.Mount("/api/v1/repos", g.proxyPrefix("/api/v1/repos", g.config.RepoAnalyzerServiceURL))
	router.Mount("/api/v1/infra", g.proxyPrefix("/api/v1/infra", g.config.InfraGeneratorServiceURL))
	router.Mount("/api/v1/pipelines", g.proxyPrefix("/api/v1/pipelines", g.config.PipelineServiceURL))
	router.Mount("/api/v1/deployments", g.proxyPrefix("/api/v1/deployments", g.config.DeploymentServiceURL))
	router.Mount("/api/v1/observability", g.proxyPrefix("/api/v1/observability", g.config.ObservabilityServiceURL))
	router.Mount("/api/v1/incidents", g.proxyPrefix("/api/v1/incidents", g.config.IncidentAIServiceURL))
	router.Mount("/ws", g.proxyPrefix("/ws", g.config.WebSocketServiceURL))

	return router
}

func (g *Gateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "api-gateway",
		"version": "0.1.0",
	})
}

func (g *Gateway) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			generatedRequestID, err := randomHex(12)
			if err != nil {
				writeGatewayError(w, http.StatusInternalServerError, "request_id_generation_failed", requestIDFromContext(r.Context()))
				return
			}
			requestID = generatedRequestID
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowedOrigin := strings.TrimSpace(g.config.DashboardOrigin)
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		g.logger.Info("gateway request",
			slog.String("request_id", requestIDFromContext(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.statusCode),
			slog.Duration("latency", time.Since(startedAt)),
		)
	})
}

func (g *Gateway) otelMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := g.tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.skipAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			writeGatewayError(w, http.StatusUnauthorized, "unauthorized", requestIDFromContext(r.Context()))
			return
		}

		rawToken := strings.TrimSpace(authorization[len("Bearer "):])
		user, err := sharedauth.ParseUserToken(g.config.JWTPublicKeyPath, rawToken)
		if err != nil {
			writeGatewayError(w, http.StatusUnauthorized, "unauthorized", requestIDFromContext(r.Context()))
			return
		}

		ctx := sharedauth.ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.skipAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		user := sharedauth.UserFromContext(r.Context())
		if user == nil {
			writeGatewayError(w, http.StatusUnauthorized, "unauthorized", requestIDFromContext(r.Context()))
			return
		}

		allowed, retryAfter, err := g.allowRequest(r.Context(), user.UserID)
		if err != nil {
			g.logger.Error("rate limit failure", slog.String("error", err.Error()), slog.String("request_id", requestIDFromContext(r.Context())))
			writeGatewayError(w, http.StatusServiceUnavailable, "service_unavailable", requestIDFromContext(r.Context()))
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeGatewayError(w, http.StatusTooManyRequests, "rate_limited", requestIDFromContext(r.Context()))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) allowRequest(ctx context.Context, userID string) (bool, int, error) {
	now := time.Now().UnixMilli()
	windowStart := now - int64(time.Minute/time.Millisecond)
	key := "rate-limit:" + userID
	pipeline := g.redisClient.TxPipeline()
	pipeline.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart, 10))
	pipeline.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: strconv.FormatInt(now, 10)})
	countCommand := pipeline.ZCard(ctx, key)
	pipeline.Expire(ctx, key, 2*time.Minute)
	if _, err := pipeline.Exec(ctx); err != nil {
		return false, 0, fmt.Errorf("exec rate limit pipeline: %w", err)
	}
	if countCommand.Val() > requestsPerMinute {
		return false, int(time.Minute.Seconds()), nil
	}
	return true, 0, nil
}

func (g *Gateway) proxyPrefix(prefix, target string) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil {
		panic(fmt.Errorf("parse proxy target %s: %w", target, err))
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		trimmedPath := strings.TrimPrefix(req.URL.Path, prefix)
		if trimmedPath == "" {
			trimmedPath = "/"
		}
		req.URL.Path = singleJoiningSlash(targetURL.Path, trimmedPath)
		req.Host = targetURL.Host
		req.Header.Set("X-Request-ID", requestIDFromContext(req.Context()))
		if user := sharedauth.UserFromContext(req.Context()); user != nil {
			req.Header.Set("X-Helmix-User-ID", user.UserID)
			req.Header.Set("X-Helmix-Org-ID", user.OrgID)
			req.Header.Set("X-Helmix-Role", user.Role)
		}
		if req.URL.RawPath != "" {
			req.URL.RawPath = singleJoiningSlash(targetURL.Path, strings.TrimPrefix(req.URL.RawPath, prefix))
		}
		if strings.TrimSpace(req.URL.Path) == "" {
			req.URL.Path = "/"
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		g.logger.Error("proxy upstream unavailable",
			slog.String("target", target),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
			slog.String("request_id", requestIDFromContext(r.Context())),
		)
		writeGatewayError(w, http.StatusServiceUnavailable, "service_unavailable", requestIDFromContext(r.Context()))
	}
	return proxy
}

func (g *Gateway) skipAuth(path string) bool {
	return path == "/health" || strings.HasPrefix(path, "/api/v1/auth/") || path == "/api/v1/auth"
}

func writeGatewayError(w http.ResponseWriter, statusCode int, code, requestID string) {
	message := http.StatusText(statusCode)
	if code == "unauthorized" {
		message = "authentication required"
	}
	if code == "rate_limited" {
		message = "rate limit exceeded"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error:     message,
		Code:      code,
		RequestID: requestID,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	return requestID
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func singleJoiningSlash(left, right string) string {
	leftHasSlash := strings.HasSuffix(left, "/")
	rightHasSlash := strings.HasPrefix(right, "/")
	switch {
	case leftHasSlash && rightHasSlash:
		return left + right[1:]
	case !leftHasSlash && !rightHasSlash:
		return left + "/" + right
	default:
		return left + right
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
