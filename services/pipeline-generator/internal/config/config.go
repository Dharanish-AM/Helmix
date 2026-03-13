package config

import (
	"os"
	"strings"
)

// Config contains the environment-driven pipeline-generator settings.
type Config struct {
	Port string
}

// Load returns validated pipeline-generator config values.
func Load() Config {
	return Config{
		Port: getEnv("PORT", "8084"),
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}