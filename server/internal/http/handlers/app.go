package handlers

import (
	"encoding/json"
	"net/http"

	"server/internal/infra"
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
	return &App{
		Config:         cfg,
		Logger:         logger,
		DB:             pool,
		SQL:            runner,
		GeoIPResolver:  geoResolver,
		GoogleVerifier: googleauth.NewVerifier(cfg.GoogleIssuer, cfg.GoogleClientID),
		PromptEnhancer: prompt.NewStaticEnhancer(),
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
