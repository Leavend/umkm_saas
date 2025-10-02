package pgxpool

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Config is a reduced configuration model compatible with the real pgxpool.
type Config struct {
	ConnString      string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// ParseConfig returns a configuration struct populated with the provided DSN.
func ParseConfig(connString string) (*Config, error) {
	if connString == "" {
		return nil, errors.New("pgxstub: empty connection string")
	}
	return &Config{ConnString: connString}, nil
}

// Pool is a placeholder implementation that satisfies the methods consumed by the application.
type Pool struct{}

// NewWithConfig validates the configuration and returns a stub pool instance.
func NewWithConfig(_ context.Context, cfg *Config) (*Pool, error) {
	if cfg == nil || cfg.ConnString == "" {
		return nil, errors.New("pgxstub: missing connection configuration")
	}
	return &Pool{}, nil
}

// Close is a no-op for the stub implementation.
func (p *Pool) Close() {}

// QueryRow returns a stub row that yields ErrNoRows when scanned.
func (p *Pool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return pgx.SimpleRow{}
}

// Query returns a stub rows iterator that reports no results.
type emptyRows struct{}

func (r emptyRows) Next() bool             { return false }
func (r emptyRows) Scan(dest ...any) error { return pgx.ErrNoRows }
func (r emptyRows) Err() error             { return nil }
func (r emptyRows) Close()                 {}

func (p *Pool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return emptyRows{}, nil
}

// Exec returns an empty command tag and a nil error to mimic successful execution.
func (p *Pool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
