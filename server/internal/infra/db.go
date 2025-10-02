package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewDBPool initializes a new pgx connection pool using the provided configuration.
func NewDBPool(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	return pool, nil
}
