package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config contains the environment-driven repo-analyzer settings.
type Config struct {
	Port                  string
	DatabaseURL           string
	NATSURL               string
	IncidentAIClassifyURL string
	CloneBaseDir          string
	GitBinary             string
	HTTPClientTimeout     time.Duration
}

// Load validates the repo-analyzer configuration.
func Load() (Config, error) {
	config := Config{
		Port:                  getEnv("PORT", "8082"),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		NATSURL:               strings.TrimSpace(os.Getenv("NATS_URL")),
		IncidentAIClassifyURL: getEnv("INCIDENT_AI_CLASSIFY_URL", "http://localhost:8087/classify"),
		CloneBaseDir:          getEnv("REPO_CLONE_BASE_DIR", os.TempDir()),
		GitBinary:             getEnv("GIT_BINARY", "git"),
		HTTPClientTimeout:     10 * time.Second,
	}

	required := map[string]string{
		"DATABASE_URL": config.DatabaseURL,
		"NATS_URL":     config.NATSURL,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}

	for _, rawURL := range []string{config.DatabaseURL, config.NATSURL, config.IncidentAIClassifyURL} {
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
