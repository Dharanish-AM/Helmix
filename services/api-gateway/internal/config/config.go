package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config contains environment-driven gateway settings.
type Config struct {
	Port                     string
	JWTPublicKeyPath         string
	RedisURL                 string
	AuthServiceURL           string
	RepoAnalyzerServiceURL   string
	InfraGeneratorServiceURL string
	PipelineServiceURL       string
	DeploymentServiceURL     string
	ObservabilityServiceURL  string
	IncidentAIServiceURL     string
	WebSocketServiceURL      string
}

// Load validates the API gateway configuration.
func Load() (Config, error) {
	config := Config{
		Port:                     getEnv("PORT", "8080"),
		JWTPublicKeyPath:         strings.TrimSpace(os.Getenv("JWT_PUBLIC_KEY_PATH")),
		RedisURL:                 strings.TrimSpace(os.Getenv("REDIS_URL")),
		AuthServiceURL:           getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
		RepoAnalyzerServiceURL:   getEnv("REPO_ANALYZER_SERVICE_URL", "http://localhost:8082"),
		InfraGeneratorServiceURL: getEnv("INFRA_GENERATOR_SERVICE_URL", "http://localhost:8083"),
		PipelineServiceURL:       getEnv("PIPELINE_GENERATOR_SERVICE_URL", "http://localhost:8084"),
		DeploymentServiceURL:     getEnv("DEPLOYMENT_ENGINE_SERVICE_URL", "http://localhost:8085"),
		ObservabilityServiceURL:  getEnv("OBSERVABILITY_SERVICE_URL", "http://localhost:8086"),
		IncidentAIServiceURL:     getEnv("INCIDENT_AI_SERVICE_URL", "http://localhost:8087"),
	}
	config.WebSocketServiceURL = getEnv("WS_UPSTREAM_URL", config.ObservabilityServiceURL)

	required := map[string]string{
		"JWT_PUBLIC_KEY_PATH": config.JWTPublicKeyPath,
		"REDIS_URL":           config.RedisURL,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}

	for _, rawURL := range []string{
		config.AuthServiceURL,
		config.RepoAnalyzerServiceURL,
		config.InfraGeneratorServiceURL,
		config.PipelineServiceURL,
		config.DeploymentServiceURL,
		config.ObservabilityServiceURL,
		config.IncidentAIServiceURL,
		config.WebSocketServiceURL,
		config.RedisURL,
	} {
		if _, err := url.Parse(rawURL); err != nil {
			return Config{}, fmt.Errorf("parse url %q: %w", rawURL, err)
		}
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
