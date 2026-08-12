// Package config loads application configuration from environment variables.
package config

import "os"

type Config struct {
	DatabaseURL string
	APIPort     string
}

func Load() Config {
	return Config{
		DatabaseURL: envOr("DATABASE_URL", "postgres://biomed:biomed@localhost:5432/biomed_cmms?sslmode=disable"),
		APIPort:     envOr("API_PORT", "8080"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
