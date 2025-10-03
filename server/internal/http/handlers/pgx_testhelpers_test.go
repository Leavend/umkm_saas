package handlers

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubRows struct {
	TestRowsBase
	scanned bool
}

func (s *stubRows) Next() bool { return !s.scanned }

func (s *stubRows) Scan(dest ...any) error {
	if s.scanned {
		return pgx.ErrNoRows
	}
	s.scanned = true
	return nil
}

func (s *stubRows) Err() error { return nil }

func (s *stubRows) Close() {}

func TestSimpleRowScan(t *testing.T) {
	t.Parallel()

	r := NewSimpleRow(func(dest ...any) error {
		if len(dest) != 1 {
			return errors.New("unexpected arguments")
		}
		if ptr, ok := dest[0].(*string); ok {
			*ptr = "ok"
			return nil
		}
		return errors.New("unexpected destination type")
	})

	var value string
	if err := r.Scan(&value); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if value != "ok" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestSimpleRowNilScannerReturnsErrNoRows(t *testing.T) {
	t.Parallel()

	var r SimpleRow
	if err := r.Scan(); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestTestRowsBaseSatisfiesRowsInterface(t *testing.T) {
	t.Parallel()

	var rows pgx.Rows = &stubRows{}
	if rows.CommandTag() != (pgconn.CommandTag{}) {
		t.Fatal("expected zero-value command tag")
	}
	if rows.Conn() != nil {
		t.Fatal("expected nil connection")
	}
	if rows.FieldDescriptions() != nil {
		t.Fatal("expected nil field descriptions")
	}
	if _, err := rows.Values(); err == nil {
		t.Fatal("expected error from Values")
	}
	if rows.RawValues() != nil {
		t.Fatal("expected nil raw values")
	}

	if !rows.Next() {
		t.Fatal("expected Next to report data on first call")
	}
	if err := rows.Scan(); err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if rows.Next() {
		t.Fatal("expected Next to report no more rows")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("unexpected Err: %v", err)
	}
}
