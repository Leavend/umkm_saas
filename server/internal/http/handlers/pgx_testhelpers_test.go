package handlers

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

type simpleRow struct {
	scan func(dest ...any) error
}

func (r simpleRow) Scan(dest ...any) error {
	if r.scan == nil {
		return pgx.ErrNoRows
	}
	return r.scan(dest...)
}

type testRowsBase struct{}

func (testRowsBase) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (testRowsBase) Conn() *pgx.Conn { return nil }

func (testRowsBase) FieldDescriptions() []pgproto3.FieldDescription { return nil }

func (testRowsBase) Values() ([]any, error) {
	return nil, fmt.Errorf("values not supported in test rows")
}

func (testRowsBase) RawValues() [][]byte { return nil }
