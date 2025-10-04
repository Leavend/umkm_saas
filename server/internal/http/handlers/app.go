package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"server/internal/infra"
	"server/internal/infra/credentials"
	"server/internal/infra/geoip"
	googleauth "server/internal/infra/google"
	"server/internal/middleware"
	"server/internal/providers/image"
	"server/internal/providers/prompt"
	"server/internal/providers/video"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type App struct {
	Config         *infra.Config
	Logger         zerolog.Logger
	DB             *pgxpool.Pool
	SQL            infra.SQLExecutor
	GeoIPResolver  geoip.CountryResolver
	GoogleVerifier *googleauth.Verifier
	PromptEnhancer prompt.Enhancer
	ImageProviders map[string]image.Generator
	VideoProviders map[string]video.Generator
	JWTSecret      string
}

func NewApp(cfg *infra.Config, pool *pgxpool.Pool, logger zerolog.Logger) *App {
	runner := infra.NewSQLRunner(pool, logger)
	geoResolver, err := geoip.NewResolver(cfg.GeoIPDBPath)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to initialize geoip resolver")
	}
	credentialStore := credentials.NewStore(runner)
	staticEnhancer := prompt.NewStaticEnhancer()
	var promptProvider prompt.Enhancer = staticEnhancer

	loadKey := func(envValue string, getter func(context.Context) (string, error), setter func(context.Context, string) error, provider string) string {
		ctxLoad, cancelLoad := context.WithTimeout(context.Background(), 2*time.Second)
		keyFromDB, err := getter(ctxLoad)
		cancelLoad()
		if err != nil {
			logger.Warn().Err(err).Str("provider", provider).Msg("failed to load api key from database")
		}
		envValue = strings.TrimSpace(envValue)
		if envValue != "" {
			if keyFromDB == "" && setter != nil {
				ctxPersist, cancelPersist := context.WithTimeout(context.Background(), 2*time.Second)
				if err := setter(ctxPersist, envValue); err != nil {
					logger.Warn().Err(err).Str("provider", provider).Msg("failed to persist api key to database")
				}
				cancelPersist()
			}
			return envValue
		}
		return strings.TrimSpace(keyFromDB)
	}

	providerChoice := strings.TrimSpace(strings.ToLower(cfg.PromptProvider))
	switch providerChoice {
	case credentials.ProviderOpenAI:
		openaiKey := loadKey(cfg.OpenAIAPIKey, credentialStore.OpenAIAPIKey, credentialStore.SetOpenAIAPIKey, credentials.ProviderOpenAI)
		if openaiKey == "" {
			logger.Warn().Str("provider", credentials.ProviderOpenAI).Msg("openai api key missing; prompt enhancer will use static provider")
			break
		}
		enhancer, err := prompt.NewOpenAIEnhancer(prompt.OpenAIOptions{
			APIKey:       openaiKey,
			Model:        cfg.OpenAIModel,
			BaseURL:      cfg.OpenAIBaseURL,
			Organization: cfg.OpenAIOrg,
			HTTPClient:   &http.Client{Timeout: 15 * time.Second},
			Fallback:     staticEnhancer,
			OnFallback: func(reason string, err error) {
				evt := logger.Warn().Str("provider", credentials.ProviderOpenAI).Str("reason", reason)
				if err != nil {
					evt = evt.Err(err)
				}
				evt.Msg("openai enhancer fallback")
			},
		})
		if err != nil {
			logger.Warn().Err(err).Str("provider", credentials.ProviderOpenAI).Msg("failed to initialize openai enhancer, falling back to static prompts")
		} else {
			promptProvider = enhancer
		}
	case credentials.ProviderGemini, "":
		geminiKey := loadKey(cfg.GeminiAPIKey, credentialStore.GeminiAPIKey, credentialStore.SetGeminiAPIKey, credentials.ProviderGemini)
		if geminiKey == "" {
			logger.Warn().Str("provider", credentials.ProviderGemini).Msg("gemini api key missing; prompt enhancer will use static provider")
			break
		}
		enhancer, err := prompt.NewGeminiEnhancer(prompt.GeminiOptions{
			APIKey:     geminiKey,
			Model:      cfg.GeminiModel,
			BaseURL:    cfg.GeminiBaseURL,
			HTTPClient: &http.Client{Timeout: 15 * time.Second},
			Fallback:   staticEnhancer,
			OnFallback: func(reason string, err error) {
				evt := logger.Warn().Str("provider", credentials.ProviderGemini).Str("reason", reason)
				if err != nil {
					evt = evt.Err(err)
				}
				evt.Msg("gemini enhancer fallback")
			},
		})
		if err != nil {
			logger.Warn().Err(err).Str("provider", credentials.ProviderGemini).Msg("failed to initialize gemini enhancer, falling back to static prompts")
		} else {
			promptProvider = enhancer
		}
	case "static":
		logger.Info().Msg("prompt provider configured as static; dynamic prompts disabled")
	default:
		logger.Warn().Str("provider", providerChoice).Msg("unknown prompt provider; using static prompts")
	}

	return &App{
		Config:         cfg,
		Logger:         logger,
		DB:             pool,
		SQL:            runner,
		GeoIPResolver:  geoResolver,
		GoogleVerifier: googleauth.NewVerifier(cfg.GoogleIssuer, cfg.GoogleClientID),
		PromptEnhancer: promptProvider,
		ImageProviders: map[string]image.Generator{
			"gemini":     image.NewNanoBanana(),
			"nanobanana": image.NewNanoBanana(),
		},
		VideoProviders: map[string]video.Generator{
			"veo2": video.NewVEO(),
			"veo3": video.NewVEO(),
		},
		JWTSecret: cfg.JWTSecret,
	}
}

func (a *App) json(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) error(w http.ResponseWriter, status int, code, message string) {
	a.json(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func (a *App) currentUserID(r *http.Request) string {
	return middleware.UserIDFromContext(r.Context())
}
