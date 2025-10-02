package http

import (
	"database/sql"
	"net/http"

	"server/internal/http/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(db *sql.DB) http.Handler {
	h := handlers.NewApp(db)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Logger)

	r.Get("/health", h.HealthHandler)

	r.Route("/me", func(r chi.Router) { r.Get("/", h.MeHandler) })
	r.Route("/integrations/google", func(r chi.Router) {
		r.Get("/status", h.GoogleStatusHandler)
	})

	r.Route("/requests", func(r chi.Router) {
		r.Post("/", h.EnqueueRequestHandler)
		r.Get("/{id}", h.GetRequestStatusHandler)
	})

	r.Route("/assets", func(r chi.Router) {
		r.Get("/", h.ListAssetsHandler)
		r.Get("/{id}/download", h.DownloadAssetHandler)
	})

	r.Get("/metrics/dashboard-24h", h.Dashboard24hHandler)
	return r
}
