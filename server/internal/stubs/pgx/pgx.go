package pgx

import "errors"

// ErrNoRows mirrors the sentinel error from the real pgx package.
var ErrNoRows = errors.New("pgxstub: no rows in result set")

// Row defines a minimal subset of pgx.Row used in the application.
type Row interface {
	Scan(dest ...any) error
}

// SimpleRow provides a trivial Row implementation backed by a scan func.
type SimpleRow struct {
	ScanFunc func(dest ...any) error
}

// Scan executes the stored scan function or returns ErrNoRows when nil.
func (r SimpleRow) Scan(dest ...any) error {
	if r.ScanFunc == nil {
		return ErrNoRows
	}
	return r.ScanFunc(dest...)
}

// Rows represents an iterator over query results.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// CommandTag is a placeholder for the result metadata of Exec statements.
type CommandTag struct{}

// TxOptions mirrors pgx transaction configuration.
type TxOptions struct{}

// Batch is a lightweight queue structure compatible with the pgx API.
type Batch struct{}

// Queue is a no-op implementation to satisfy usage in the code base.
func (b *Batch) Queue(_ string, _ ...any) {}

// BatchResults is a stub for the object returned by SendBatch.
type BatchResults struct{}

// Close is a no-op for compatibility with the real pgx implementation.
func (b *BatchResults) Close() error { return nil }
