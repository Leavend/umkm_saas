package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"server/internal/http/handlers"
	httpapi "server/internal/http/httpapi"
	"server/internal/infra"
)

func main() {
	// Muat .env (opsional)
	_ = godotenv.Load()

	// Konfigurasi & logger
	cfg, err := infra.LoadConfig()
	if err != nil {
		panic(err)
	}
	logger := infra.NewLogger(cfg.AppEnv)

	// DB pool (pgxpool)
	ctx := context.Background()
	dbpool, err := infra.NewDBPool(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect database")
	}
	defer dbpool.Close()

	// App container (inject DB & sqlc queries)
	app := handlers.NewApp(dbpool)

	// Bangun router via package httpapi (sudah ada middleware chi di dalamnya)
	router := httpapi.NewRouter(app)

	// HTTP server wrapper dari infra
	server := infra.NewHTTPServer(cfg, router)

	// Start async
	go func() {
		logger.Info().Msgf("API listening on :%s", cfg.Port)
		if err := server.Start(); err != nil && err != os.ErrClosed {
			logger.Fatal().Err(err).Msg("http server failed")
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPIdleTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("failed to shutdown server")
	}
	logger.Info().Msg("server stopped")
}
