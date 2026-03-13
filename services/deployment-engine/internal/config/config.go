package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains environment-driven deployment-engine settings.
type Config struct {
	Port              string
	DatabaseURL       string
	NATSURL           string
	DeployTimeout     time.Duration
	DefaultReadyDelay time.Duration
}

// Load returns validated deployment-engine config values.
func Load() (Config, error) {
	deployTimeoutSeconds, err := getPositiveIntEnv("DEPLOY_TIMEOUT_SECONDS", 300)
	if err != nil {
		return Config{}, err
	}
	defaultReadySeconds, err := getPositiveIntEnv("DEPLOY_DEFAULT_READY_SECONDS", 5)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		Port:              getEnv("PORT", "8085"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		NATSURL:           getEnv("NATS_URL", "nats://localhost:4222"),
		DeployTimeout:     time.Duration(deployTimeoutSeconds) * time.Second,
		DefaultReadyDelay: time.Duration(defaultReadySeconds) * time.Second,
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

func getPositiveIntEnv(key string, fallback int) (int, error) {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback, nil
	}

	parsedValue, err := strconv.Atoi(rawValue)
	if err != nil || parsedValue <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return parsedValue, nil
}