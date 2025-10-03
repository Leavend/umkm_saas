package infra

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// SQLExecutor defines the contract required by handlers for executing SQL queries.
type SQLExecutor interface {
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
}

var markerRegexp = regexp.MustCompile(`^--sql [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type SQLRunner struct {
	Pool   *pgxpool.Pool
	Logger zerolog.Logger
}

func NewSQLRunner(pool *pgxpool.Pool, logger zerolog.Logger) *SQLRunner {
	return &SQLRunner{Pool: pool, Logger: logger}
}

func (r *SQLRunner) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	marker, trimmed, err := extractMarker(query)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	r.Logger.Info().Msgf("sql[%s] exec", marker)
	tag, err := r.Pool.Exec(ctx, trimmed, args...)
	if err != nil {
		r.Logger.Error().Err(err).Msgf("sql[%s] error", marker)
		return tag, err
	}
	r.Logger.Info().Msgf("sql[%s] ok", marker)
	return tag, nil
}

func (r *SQLRunner) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	marker, trimmed, err := extractMarker(query)
	if err != nil {
		return errorRow{err: err}
	}
	r.Logger.Info().Msgf("sql[%s] query_row", marker)
	row := r.Pool.QueryRow(ctx, trimmed, args...)
	return loggingRow{row: row, logger: r.Logger, marker: marker}
}

func (r *SQLRunner) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	marker, trimmed, err := extractMarker(query)
	if err != nil {
		return nil, err
	}
	r.Logger.Info().Msgf("sql[%s] query", marker)
	rows, err := r.Pool.Query(ctx, trimmed, args...)
	if err != nil {
		r.Logger.Error().Err(err).Msgf("sql[%s] error", marker)
		return nil, err
	}
	return loggingRows{Rows: rows, logger: r.Logger, marker: marker}, nil
}

type loggingRow struct {
	row    pgx.Row
	logger zerolog.Logger
	marker string
}

func (l loggingRow) Scan(dest ...any) error {
	err := l.row.Scan(dest...)
	if err != nil {
		l.logger.Error().Err(err).Msgf("sql[%s] scan error", l.marker)
	}
	return err
}

type loggingRows struct {
	pgx.Rows
	logger zerolog.Logger
	marker string
}

func (l loggingRows) Close() {
	l.logger.Info().Msgf("sql[%s] rows close", l.marker)
	l.Rows.Close()
}

type errorRow struct {
	err error
}

func (e errorRow) Scan(dest ...any) error {
	return e.err
}

func extractMarker(query string) (string, string, error) {
	trimmed := strings.TrimSpace(query)
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return "", "", errors.New("empty query")
	}
	markerLine := strings.TrimSpace(lines[0])
	if !markerRegexp.MatchString(markerLine) {
		return "", "", errors.New("sql marker missing or invalid")
	}
	return strings.TrimSpace(strings.TrimPrefix(markerLine, "--sql ")), strings.Join(lines[1:], "\n"), nil
}

var _ SQLExecutor = (*SQLRunner)(nil)
