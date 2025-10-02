package infra

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents application configuration loaded from environment variables.
type Config struct {
	AppEnv           string
	Port             string
	DatabaseURL      string
	JWTSecret        string
	StorageBaseURL   string
	GeoIPDBPath      string
	GoogleClientID   string
	GoogleIssuer     string
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration
	RateLimitPerMin  int
}

// LoadConfig loads configuration from environment variables and applies defaults where needed.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		StorageBaseURL:   getEnv("STORAGE_BASE_URL", "http://localhost:8080/static"),
		GeoIPDBPath:      os.Getenv("GEOIP_DB_PATH"),
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleIssuer:     getEnv("GOOGLE_ISSUER", "https://accounts.google.com"),
		HTTPReadTimeout:  time.Second * time.Duration(getEnvInt("HTTP_READ_TIMEOUT_SECONDS", 15)),
		HTTPWriteTimeout: time.Second * time.Duration(getEnvInt("HTTP_WRITE_TIMEOUT_SECONDS", 30)),
		HTTPIdleTimeout:  time.Second * time.Duration(getEnvInt("HTTP_IDLE_TIMEOUT_SECONDS", 60)),
		RateLimitPerMin:  getEnvInt("RATE_LIMIT_PER_MINUTE", 30),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
