package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/sqlinline"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDonationsTestimonials_AllowsAnonymousTestimonials(t *testing.T) {
	t.Helper()

	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	testimonials := []donationRow{{
		id:          "donation-123",
		userID:      sql.NullString{},
		amount:      50000,
		note:        "keep going",
		testimonial: "great product",
		properties:  []byte(`{"source":"web"}`),
		createdAt:   createdAt,
	}}

	app := &App{SQL: &donationTestSQL{rows: testimonials}}

	req := httptest.NewRequest("GET", "/donations/testimonials", nil)
	rr := httptest.NewRecorder()

	app.DonationsTestimonials(rr, req)

	if rr.Code != 200 {
		t.Fatalf("unexpected status code: got %d, want 200", rr.Code)
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 testimonial, got %d", len(payload.Items))
	}

	item := payload.Items[0]
	if val, ok := item["user_id"]; ok && val != nil {
		t.Fatalf("expected user_id to be null, got %#v", val)
	}
	if item["testimonial"] != testimonials[0].testimonial {
		t.Fatalf("expected testimonial %q, got %#v", testimonials[0].testimonial, item["testimonial"])
	}
}

type donationRow struct {
	id          string
	userID      sql.NullString
	amount      int64
	note        string
	testimonial string
	properties  []byte
	createdAt   time.Time
}

type donationTestSQL struct {
	rows []donationRow
}

func (d *donationTestSQL) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *donationTestSQL) QueryRow(context.Context, string, ...any) pgx.Row {
	return pgx.SimpleRow{}
}

func (d *donationTestSQL) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	if query != sqlinline.QListDonations {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected args count: %d", len(args))
	}
	return &donationRowsIterator{rows: d.rows}, nil
}

type donationRowsIterator struct {
	rows []donationRow
	idx  int
}

func (d *donationRowsIterator) Next() bool {
	if d.idx >= len(d.rows) {
		return false
	}
	d.idx++
	return true
}

func (d *donationRowsIterator) Scan(dest ...any) error {
	if d.idx == 0 || d.idx > len(d.rows) {
		return pgx.ErrNoRows
	}
	row := d.rows[d.idx-1]
	if len(dest) != 7 {
		return fmt.Errorf("unexpected scan args: %d", len(dest))
	}
	if v, ok := dest[0].(*string); ok {
		*v = row.id
	}
	switch v := dest[1].(type) {
	case *sql.NullString:
		*v = row.userID
	case *string:
		if row.userID.Valid {
			*v = row.userID.String
		} else {
			*v = ""
		}
	}
	if v, ok := dest[2].(*int64); ok {
		*v = row.amount
	}
	if v, ok := dest[3].(*string); ok {
		*v = row.note
	}
	if v, ok := dest[4].(*string); ok {
		*v = row.testimonial
	}
	if v, ok := dest[5].(*[]byte); ok {
		if row.properties != nil {
			*v = append([]byte(nil), row.properties...)
		} else {
			*v = nil
		}
	}
	if v, ok := dest[6].(*time.Time); ok {
		*v = row.createdAt
	}
	return nil
}

func (d *donationRowsIterator) Err() error { return nil }

func (d *donationRowsIterator) Close() {}
