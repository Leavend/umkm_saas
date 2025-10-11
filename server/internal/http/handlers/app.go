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
	"server/internal/providers/genai"
	"server/internal/providers/image"
	"server/internal/providers/prompt"
	"server/internal/providers/qwen"
	"server/internal/providers/video"
	"server/internal/storage"

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
	FileStore      *storage.FileStore
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

	// Preload provider credentials so we can wire graceful fallbacks (Gemini -> OpenAI -> Static).
	openaiKey := loadKey(cfg.OpenAIAPIKey, credentialStore.OpenAIAPIKey, credentialStore.SetOpenAIAPIKey, credentials.ProviderOpenAI)
	var openaiEnhancer prompt.Enhancer
	if openaiKey != "" {
		enhancer, err := prompt.NewOpenAIEnhancer(prompt.OpenAIOptions{
			APIKey:       openaiKey,
			Model:        cfg.OpenAIModel,
			BaseURL:      cfg.OpenAIBaseURL,
			Organization: cfg.OpenAIOrg,
			HTTPClient:   &http.Client{Timeout: 15 * time.Second},
			Fallback:     staticEnhancer,
			OnFallback: func(reason string, err error) {
				evt := logger.Info().Str("provider", credentials.ProviderOpenAI).Str("reason", reason)
				if err != nil {
					evt = evt.Err(err)
				}
				evt.Msg("openai enhancer fallback")
			},
			OnWarning: func(reason, detail string) {
				logger.Warn().
					Str("provider", credentials.ProviderOpenAI).
					Str("reason", reason).
					Str("detail", detail).
					Msg("openai enhancer normalization")
			},
		})
		if err != nil {
			logger.Warn().Err(err).Str("provider", credentials.ProviderOpenAI).Msg("failed to initialize openai enhancer, falling back to static prompts")
		} else {
			openaiEnhancer = enhancer
		}
	}

	qwenKey := loadKey(cfg.QwenAPIKey, credentialStore.QwenAPIKey, credentialStore.SetQwenAPIKey, credentials.ProviderQwen)
	geminiKey := loadKey(cfg.GeminiAPIKey, credentialStore.GeminiAPIKey, credentialStore.SetGeminiAPIKey, credentials.ProviderGemini)
	var geminiEnhancer prompt.Enhancer
	if geminiKey != "" {
		geminiFallback := prompt.Enhancer(staticEnhancer)
		if openaiEnhancer != nil {
			geminiFallback = openaiEnhancer
		}
		enhancer, err := prompt.NewGeminiEnhancer(prompt.GeminiOptions{
			APIKey:     geminiKey,
			Model:      cfg.GeminiModel,
			BaseURL:    cfg.GeminiBaseURL,
			HTTPClient: &http.Client{Timeout: 15 * time.Second},
			Fallback:   geminiFallback,
			OnFallback: func(reason string, err error) {
				evt := logger.Info().Str("provider", credentials.ProviderGemini).Str("reason", reason)
				if err != nil {
					evt = evt.Err(err)
				}
				evt.Msg("gemini enhancer fallback")
			},
		})
		if err != nil {
			logger.Warn().Err(err).Str("provider", credentials.ProviderGemini).Msg("failed to initialize gemini enhancer, falling back to static prompts")
		} else {
			geminiEnhancer = enhancer
		}
	}

	switch providerChoice {
	case credentials.ProviderOpenAI:
		switch {
		case openaiEnhancer != nil:
			promptProvider = openaiEnhancer
		case geminiEnhancer != nil:
			logger.Warn().Str("provider", credentials.ProviderOpenAI).Msg("openai api key unavailable; using gemini enhancer instead")
			promptProvider = geminiEnhancer
		default:
			logger.Warn().Str("provider", credentials.ProviderOpenAI).Msg("openai api key missing; prompt enhancer will use static provider")
		}
	case credentials.ProviderGemini, "":
		switch {
		case geminiEnhancer != nil:
			promptProvider = geminiEnhancer
		case openaiEnhancer != nil:
			logger.Warn().Str("provider", credentials.ProviderGemini).Msg("gemini api key unavailable; using openai enhancer instead")
			promptProvider = openaiEnhancer
		default:
			logger.Warn().Str("provider", credentials.ProviderGemini).Msg("gemini api key missing; prompt enhancer will use static provider")
		}
	case "static":
		logger.Info().Msg("prompt provider configured as static; dynamic prompts disabled")
	default:
		logger.Warn().Str("provider", providerChoice).Msg("unknown prompt provider; using static prompts")
	}

	geminiClient, err := genai.NewClient(genai.Options{
		APIKey:     geminiKey,
		BaseURL:    cfg.GeminiBaseURL,
		Model:      cfg.GeminiModel,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Logger:     &logger,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure gemini client")
	}

	if geminiKey == "" {
		logger.Warn().Str("model", geminiClient.Model()).Msg("gemini api key missing; synthetic media assets will be generated")
	}

	qwenClient, err := qwen.NewClient(qwen.Options{
		APIKey:         qwenKey,
		BaseURL:        cfg.QwenBaseURL,
		Model:          cfg.QwenModel,
		DefaultSize:    cfg.QwenDefaultSize,
		PromptExtend:   true,
		Watermark:      false,
		HTTPClient:     &http.Client{Timeout: 45 * time.Second},
		Logger:         &logger,
		RequestTimeout: 45 * time.Second,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure qwen client")
	}
	if !qwenClient.HasCredentials() {
		logger.Warn().Str("model", qwenClient.Model()).Msg("qwen api key missing; worker will fall back to synthetic assets")
	}

	geminiImage := image.NewGeminiGenerator(geminiClient)
	geminiVideo := video.NewGeminiGenerator(geminiClient)
	qwenImage := image.NewQwenGenerator(qwenClient, geminiImage)

	fileStore, err := storage.NewFileStore(cfg.StoragePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure storage root")
	}

	imageProviders := map[string]image.Generator{
		"qwen":                              qwenImage,
		"qwen-image":                        qwenImage,
		"qwen-image-plus":                   qwenImage,
		strings.ToLower(qwenClient.Model()): qwenImage,
		"gemini":                            geminiImage,
		"gemini-1.5-flash":                  geminiImage,
		"gemini-2.0-flash":                  geminiImage,
		"gemini-2.5-flash":                  geminiImage,
	}

	return &App{
		Config:         cfg,
		Logger:         logger,
		DB:             pool,
		SQL:            runner,
		GeoIPResolver:  geoResolver,
		GoogleVerifier: googleauth.NewVerifier(cfg.GoogleIssuer, cfg.GoogleClientID),
		PromptEnhancer: promptProvider,
		ImageProviders: imageProviders,
		VideoProviders: map[string]video.Generator{
			"gemini":           geminiVideo,
			"gemini-1.5-flash": geminiVideo,
			"gemini-2.0-flash": geminiVideo,
			"gemini-2.5-flash": geminiVideo,
		},
		JWTSecret: cfg.JWTSecret,
		FileStore: fileStore,
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

func (a *App) assetURL(storageKey string) string {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return ""
	}
	lower := strings.ToLower(storageKey)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return storageKey
	}
	base := strings.TrimRight(a.Config.StorageBaseURL, "/")
	key := strings.TrimLeft(storageKey, "/")
	return base + "/" + key
}

func (a *App) currentUserID(r *http.Request) string {
	return middleware.UserIDFromContext(r.Context())
}
