package infra

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config represents application configuration loaded from environment variables.
type Config struct {
	AppEnv               string
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	StorageBaseURL       string
	StoragePath          string
	GeoIPDBPath          string
	GoogleClientID       string
	GoogleIssuer         string
	PromptProvider       string
	QwenAPIKey           string
	QwenModel            string
	QwenBaseURL          string
	QwenDefaultSize      string
	GeminiAPIKey         string
	GeminiModel          string
	GeminiBaseURL        string
	OpenAIAPIKey         string
	OpenAIModel          string
	OpenAIBaseURL        string
	OpenAIOrg            string
	ImageSourceAllowlist []string
	HTTPReadTimeout      time.Duration
	HTTPWriteTimeout     time.Duration
	HTTPIdleTimeout      time.Duration
	RateLimitPerMin      int
}

// LoadConfig loads configuration from environment variables and applies defaults where needed.
func LoadConfig() (*Config, error) {
	port := getEnv("PORT", "8080")
	storageBaseDefault := fmt.Sprintf("http://localhost:%s/static", port)

	allowlistHosts := make(map[string]struct{})
	if rawAllowlist := strings.TrimSpace(os.Getenv("IMAGE_SOURCE_HOST_ALLOWLIST")); rawAllowlist != "" {
		for _, host := range strings.Split(rawAllowlist, ",") {
			normalized := strings.ToLower(strings.TrimSpace(host))
			if normalized != "" {
				allowlistHosts[normalized] = struct{}{}
			}
		}
	}

	cfg := &Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		Port:             port,
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		StorageBaseURL:   getEnv("STORAGE_BASE_URL", storageBaseDefault),
		StoragePath:      getEnv("STORAGE_PATH", "./storage"),
		GeoIPDBPath:      os.Getenv("GEOIP_DB_PATH"),
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleIssuer:     getEnv("GOOGLE_ISSUER", "https://accounts.google.com"),
		PromptProvider:   getEnv("PROMPT_PROVIDER", "gemini"),
		QwenAPIKey:       os.Getenv("QWEN_API_KEY"),
		QwenModel:        getEnv("QWEN_MODEL", "qwen-image-plus"),
		QwenBaseURL:      getEnv("QWEN_BASE_URL", "https://dashscope-intl.aliyuncs.com/api/v1"),
		QwenDefaultSize:  getEnv("QWEN_DEFAULT_SIZE", "1328*1328"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		GeminiModel:      getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		GeminiBaseURL:    getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:      getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIOrg:        os.Getenv("OPENAI_ORG"),
		HTTPReadTimeout:  time.Second * time.Duration(getEnvInt("HTTP_READ_TIMEOUT_SECONDS", 15)),
		HTTPWriteTimeout: time.Second * time.Duration(getEnvInt("HTTP_WRITE_TIMEOUT_SECONDS", 30)),
		HTTPIdleTimeout:  time.Second * time.Duration(getEnvInt("HTTP_IDLE_TIMEOUT_SECONDS", 60)),
		RateLimitPerMin:  getEnvInt("RATE_LIMIT_PER_MINUTE", 30),
	}

	if parsedBase, err := url.Parse(cfg.StorageBaseURL); err == nil && parsedBase != nil {
		if host := strings.ToLower(strings.TrimSpace(parsedBase.Hostname())); host != "" {
			allowlistHosts[host] = struct{}{}
		}
	}

	if len(allowlistHosts) > 0 {
		cfg.ImageSourceAllowlist = make([]string, 0, len(allowlistHosts))
		for host := range allowlistHosts {
			cfg.ImageSourceAllowlist = append(cfg.ImageSourceAllowlist, host)
		}
		sort.Strings(cfg.ImageSourceAllowlist)
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
