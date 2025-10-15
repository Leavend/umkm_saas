package infra

import (
	"context"
	"fmt"
	"net/http"
)

// Server adalah wrapper untuk http.Server.
type Server struct {
	*http.Server
	cfg *Config
}

// NewHTTPServer membuat instance Server baru.
func NewHTTPServer(cfg *Config, handler http.Handler) *Server {
	return &Server{
		Server: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.HTTPReadTimeout,
			WriteTimeout: cfg.HTTPWriteTimeout,
			IdleTimeout:  cfg.HTTPIdleTimeout,
		},
		cfg: cfg,
	}
}

// Start menjalankan server.
// Server akan berjalan dengan HTTPS jika CertFile dan KeyFile disediakan di config.
// Jika tidak, server akan berjalan dengan HTTP biasa.
func (s *Server) Start() error {
	// Cek apakah konfigurasi TLS ada
	if s.cfg.CertFile != "" && s.cfg.KeyFile != "" {
		// Jalankan dengan HTTPS
		return s.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile)
	}
	// Jalankan dengan HTTP jika tidak ada konfigurasi TLS
	return s.ListenAndServe()
}

// Shutdown mematikan server dengan graceful shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}
