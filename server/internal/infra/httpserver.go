package infra

import (
	"context"
	"net/http"
	"time"
)

// HTTPServer wraps http.Server to provide graceful startup and shutdown helpers.
type HTTPServer struct {
	server *http.Server
}

// NewHTTPServer creates a configured HTTP server instance.
func NewHTTPServer(cfg *Config, handler http.Handler) *HTTPServer {
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &HTTPServer{server: srv}
}

// Start runs the HTTP server in the current goroutine.
func (s *HTTPServer) Start() error {
	if s.server == nil {
		return nil
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
