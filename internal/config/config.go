package config

import (
	"os"
	"time"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTExpiration time.Duration
}

// Load loads the configuration from the environment, applying sensible defaults.
// This allows Prompt 1 to run seamlessly without database or JWT variables configured.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")

	jwtSecret := os.Getenv("JWT_SECRET")

	var jwtExp time.Duration
	if expStr := os.Getenv("JWT_EXPIRATION"); expStr != "" {
		if d, err := time.ParseDuration(expStr); err == nil {
			jwtExp = d
		} else {
			jwtExp = 24 * time.Hour
		}
	} else {
		jwtExp = 24 * time.Hour
	}

	return &Config{
		Port:          port,
		DatabaseURL:   databaseURL,
		JWTSecret:     jwtSecret,
		JWTExpiration: jwtExp,
	}
}
