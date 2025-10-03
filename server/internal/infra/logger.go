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

// Logger aliases the zerolog.Logger so callers outside the infra package can
// depend on the logging contract without importing the third-party module
// directly. It keeps the freedom to replace the underlying logger in the
// future while presenting a stable surface area.
type Logger = zerolog.Logger
