// Package config loads application configuration from environment variables.
package config

import (
	"os"
	"time"
)

type Config struct {
	DatabaseURL       string
	APIPort           string
	CORSAllowedOrigin string
	SessionCookieName string
	SessionTTL        time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:       envOr("DATABASE_URL", "postgres://biomed:biomed@localhost:5432/biomed_cmms?sslmode=disable"),
		APIPort:           envOr("API_PORT", "8080"),
		CORSAllowedOrigin: envOr("CORS_ALLOWED_ORIGIN", "http://localhost:3000"),
		SessionCookieName: envOr("SESSION_COOKIE_NAME", "session"),
		SessionTTL:        durationOr("SESSION_TTL", 12*time.Hour),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
