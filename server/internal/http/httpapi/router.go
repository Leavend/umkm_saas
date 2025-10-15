package httpapi

import (
	"net/http"
	"strings"
	"time"

	"server/internal/http/handlers"
	"server/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func NewRouter(app *handlers.App) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(app.Logger))

	var geoLookup middleware.CountryLookup
	if app.GeoIPResolver != nil {
		geoLookup = app.GeoIPResolver.CountryCode
	}
	r.Use(middleware.I18N("en", geoLookup))
	r.Use(middleware.CORS([]string{"http://localhost:3000", "https://script.google.com"}))
	r.Use(middleware.RateLimit(app.Config.RateLimitPerMin, time.Minute))

	if base := strings.TrimSpace(app.Config.StoragePath); base != "" {
		fs := http.StripPrefix("/static/", http.FileServer(http.Dir(base)))
		r.Handle("/static/*", fs)
	}

	r.Route("/v1", func(r chi.Router) {
		r.Get("/healthz", app.Health)
		r.Get("/openapi.json", app.OpenAPIJSON)
		r.Get("/docs", app.OpenAPIDocs)

		r.Post("/auth/google/verify", app.AuthGoogleVerify)
		r.With(middleware.AuthJWT(app.JWTSecret)).Get("/me", app.Me)

		r.With(middleware.AuthJWT(app.JWTSecret)).Route("/prompts", func(r chi.Router) {
			r.Post("/enhance", app.PromptEnhance)
			r.Post("/random", app.PromptRandom)
			r.Post("/clear", app.PromptClear)
		})

		r.With(middleware.AuthJWT(app.JWTSecret)).Route("/images", func(r chi.Router) {
			r.Post("/uploads", app.ImagesUpload)
			r.Post("/generate", app.ImagesGenerate)
			r.Get("/jobs/{id}", app.ImageJob)
			r.Get("/{job_id}/download", app.ImageDownload)
			r.Get("/{job_id}/download.zip", app.ImageDownloadZip)
		})

		r.With(middleware.AuthJWT(app.JWTSecret)).Route("/ideas", func(r chi.Router) {
			r.Post("/from-image", app.IdeasFromImage)
		})

		r.With(middleware.AuthJWT(app.JWTSecret)).Route("/videos", func(r chi.Router) {
			r.Post("/generate", app.VideosGenerate)
			r.Get("/{job_id}/status", app.VideoStatus)
			r.Get("/{job_id}/assets", app.VideoAssets)
		})

		r.With(middleware.AuthJWT(app.JWTSecret)).Route("/assets", func(r chi.Router) {
			r.Get("/", app.ListAssets)
			r.Get("/{id}/download", app.DownloadAsset)
		})

		r.Get("/stats/summary", app.StatsSummary)
		r.Post("/donations", app.DonationsCreate)
		r.Get("/donations/testimonials", app.DonationsTestimonials)
	})

	return r
}
