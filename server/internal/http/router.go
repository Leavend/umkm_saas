package httpapi

import (
	stdhttp "net/http"

	"server/internal/http/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(app *handlers.App) stdhttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Logger)

	// Health
	r.Get("/v1/healthz", app.Health)

	r.Route("/me", func(r chi.Router) { r.Get("/", handlers.MeHandler) })

	r.Route("/integrations/google", func(r chi.Router) {
		r.Get("/status", handlers.GoogleStatusHandler)
	})

	r.Route("/requests", func(r chi.Router) {
		r.Post("/", handlers.EnqueueRequestHandler)
		r.Get("/{id}", handlers.GetRequestStatusHandler)
	})

	r.Route("/assets", func(r chi.Router) {
		r.Get("/", handlers.ListAssetsHandler)
		r.Get("/{id}/download", handlers.DownloadAssetHandler)
	})

	r.Get("/metrics/dashboard-24h", handlers.Dashboard24hHandler)

	return r
}
