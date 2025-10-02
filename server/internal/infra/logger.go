package infra

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// NewLogger constructs a zerolog.Logger with sane defaults for the service.
func NewLogger(appEnv string) zerolog.Logger {
	level := zerolog.InfoLevel
	if appEnv == "development" {
		level = zerolog.DebugLevel
	}

	logger := zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Logger()

	if appEnv == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	return logger
}
