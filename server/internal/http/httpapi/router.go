package httpapi

import (
	"net/http"

	"server/internal/http/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(app *handlers.App) http.Handler {
	r := chi.NewRouter()

	// Middlewares dasar
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Logger,
	)

	// Health
	r.Get("/v1/healthz", app.Health)

	// ---- TODO: wire route lain (ubah ke method receiver kalau perlu DB) ----
	// r.Route("/me", func(r chi.Router) { r.Get("/", app.Me) })
	// r.Route("/integrations/google", func(r chi.Router) {
	// 	r.Get("/status", app.GoogleStatus)
	// })
	// r.Route("/requests", func(r chi.Router) {
	// 	r.Post("/", app.EnqueueRequest)
	// 	r.Get("/{id}", app.GetRequestStatus)
	// })
	// r.Route("/assets", func(r chi.Router) {
	// 	r.Get("/", app.ListAssets)
	// 	r.Get("/{id}/download", app.DownloadAsset)
	// })
	// r.Get("/metrics/dashboard-24h", app.Dashboard24h)

	return r
}
