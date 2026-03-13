package config

import (
	"fmt"
	"os"
	"strings"
)

// Config contains environment-driven observability settings.
type Config struct {
	Port        string
	MetricsPort string
	DatabaseURL string
	NATSURL     string
}

// Load returns validated observability config values.
func Load() (Config, error) {
	config := Config{
		Port:        getEnv("PORT", "8086"),
		MetricsPort: getEnv("METRICS_PORT", "9090"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		NATSURL:     getEnv("NATS_URL", "nats://localhost:4222"),
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}