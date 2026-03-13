package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains all environment-driven auth-service settings.
type Config struct {
	Port                 string
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubRedirectURL    string
	GitHubOAuthBaseURL   string
	GitHubAPIBaseURL     string
	JWTPrivateKeyPath    string
	JWTPublicKeyPath     string
	DatabaseURL          string
	RedisURL             string
	DashboardURL         string
	RepoAnalyzerURL      string
	TokenEncryptionKey   []byte
	JWTTTL               time.Duration
	RefreshTTL           time.Duration
	RefreshCookieName    string
	CookieSecure         bool
	OAuthStateCookieName string
	HTTPClientTimeout    time.Duration
}

// Load validates auth-service configuration from environment variables.
func Load() (Config, error) {
	config := Config{
		Port:                 getEnv("PORT", "8081"),
		GitHubClientID:       strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		GitHubClientSecret:   strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		GitHubRedirectURL:    strings.TrimSpace(os.Getenv("GITHUB_REDIRECT_URL")),
		GitHubOAuthBaseURL:   getEnv("GITHUB_OAUTH_BASE_URL", "https://github.com/login/oauth"),
		GitHubAPIBaseURL:     getEnv("GITHUB_API_BASE_URL", "https://api.github.com"),
		JWTPrivateKeyPath:    strings.TrimSpace(os.Getenv("JWT_PRIVATE_KEY_PATH")),
		JWTPublicKeyPath:     strings.TrimSpace(os.Getenv("JWT_PUBLIC_KEY_PATH")),
		DatabaseURL:          strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisURL:             strings.TrimSpace(os.Getenv("REDIS_URL")),
		DashboardURL:         getEnv("DASHBOARD_URL", "http://localhost:3000"),
		RepoAnalyzerURL:      getEnv("REPO_ANALYZER_URL", "http://repo-analyzer:8082"),
		JWTTTL:               mustParseDuration(getEnv("JWT_TTL", "24h")),
		RefreshTTL:           mustParseDuration(getEnv("REFRESH_TOKEN_TTL", "720h")),
		RefreshCookieName:    getEnv("REFRESH_TOKEN_COOKIE_NAME", "helmix_refresh_token"),
		OAuthStateCookieName: getEnv("OAUTH_STATE_COOKIE_NAME", "helmix_oauth_state"),
		HTTPClientTimeout:    mustParseDuration(getEnv("HTTP_CLIENT_TIMEOUT", "10s")),
	}

	if strings.TrimSpace(os.Getenv("COOKIE_SECURE")) != "" {
		cookieSecure, err := strconv.ParseBool(os.Getenv("COOKIE_SECURE"))
		if err != nil {
			return Config{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
		}
		config.CookieSecure = cookieSecure
	} else {
		config.CookieSecure = strings.HasPrefix(config.DashboardURL, "https://")
	}

	encryptionKey, err := parseEncryptionKey(strings.TrimSpace(os.Getenv("GITHUB_TOKEN_ENCRYPTION_KEY")))
	if err != nil {
		return Config{}, fmt.Errorf("parse GITHUB_TOKEN_ENCRYPTION_KEY: %w", err)
	}
	config.TokenEncryptionKey = encryptionKey

	if err := validate(config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func mustParseDuration(raw string) time.Duration {
	duration, err := time.ParseDuration(raw)
	if err != nil {
		panic(err)
	}
	return duration
}

func parseEncryptionKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("value is required")
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		if len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, errors.New("must be 32 raw bytes or base64-encoded 32 bytes")
}

func validate(config Config) error {
	required := map[string]string{
		"GITHUB_CLIENT_ID":          config.GitHubClientID,
		"GITHUB_CLIENT_SECRET":      config.GitHubClientSecret,
		"GITHUB_REDIRECT_URL":       config.GitHubRedirectURL,
		"JWT_PRIVATE_KEY_PATH":      config.JWTPrivateKeyPath,
		"JWT_PUBLIC_KEY_PATH":       config.JWTPublicKeyPath,
		"DATABASE_URL":              config.DatabaseURL,
		"REDIS_URL":                 config.RedisURL,
		"REFRESH_TOKEN_COOKIE_NAME": config.RefreshCookieName,
		"OAUTH_STATE_COOKIE_NAME":   config.OAuthStateCookieName,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	for _, rawURL := range []string{config.GitHubRedirectURL, config.GitHubOAuthBaseURL, config.GitHubAPIBaseURL, config.DashboardURL, config.RepoAnalyzerURL, config.DatabaseURL, config.RedisURL} {
		if _, err := url.Parse(rawURL); err != nil {
			return fmt.Errorf("parse url %q: %w", rawURL, err)
		}
	}

	return nil
}
