// Package config loads application configuration from environment variables.
// It reads a .env file (if present) and exposes a typed Config struct
// that the rest of the application consumes.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration values for the server.
type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	JWTExpiresHours int
}

// Load reads configuration from the environment, optionally loading a .env file.
// Returns an error if any required variable is missing.
func Load() (*Config, error) {
	// Load .env file if it exists; ignore error because in production
	// the variables are injected directly via the environment.
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	dbURL := getEnvRequired("DATABASE_URL")
	jwtSecret := getEnvRequired("JWT_SECRET")
	expiresHours, err := strconv.Atoi(getEnv("JWT_EXPIRES_HOURS", "72"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRES_HOURS: %w", err)
	}

	return &Config{
		Port:            port,
		DatabaseURL:     dbURL,
		JWTSecret:       jwtSecret,
		JWTExpiresHours: expiresHours,
	}, nil
}

// getEnv returns the environment variable value or a fallback default.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvRequired returns the environment variable value or panics.
// Use this for settings that have no sensible default (e.g. secrets, DB URLs).
func getEnvRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}
