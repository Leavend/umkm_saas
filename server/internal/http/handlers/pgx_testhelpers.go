package handlers

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type SimpleRow struct {
	scan func(dest ...any) error
}

func NewSimpleRow(scanner func(dest ...any) error) SimpleRow {
	return SimpleRow{scan: scanner}
}

func (r SimpleRow) Scan(dest ...any) error {
	if r.scan == nil {
		return pgx.ErrNoRows
	}
	return r.scan(dest...)
}

type TestRowsBase struct{}

func (TestRowsBase) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (TestRowsBase) Conn() *pgx.Conn { return nil }

func (TestRowsBase) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (TestRowsBase) Values() ([]any, error) {
	return nil, fmt.Errorf("values not supported in test rows")
}

func (TestRowsBase) RawValues() [][]byte { return nil }
