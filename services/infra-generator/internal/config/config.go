package config

import (
	"os"
	"strings"
)

// Config contains the environment-driven infra-generator settings.
type Config struct {
	Port string
}

// Load returns validated infra-generator config values.
func Load() Config {
	return Config{
		Port: getEnv("PORT", "8083"),
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
