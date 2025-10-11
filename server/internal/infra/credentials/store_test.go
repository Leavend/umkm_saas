package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubExecutor struct {
	token string
	err   error
	exec  struct {
		query string
		args  []any
	}
}

func (s *stubExecutor) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	s.exec.query = query
	s.exec.args = args
	return pgconn.CommandTag{}, s.err
}

func (s *stubExecutor) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return stubRow{token: s.token, err: s.err}
}

func (s *stubExecutor) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

type stubRow struct {
	token string
	err   error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return errors.New("no dest")
	}
	ptr, ok := dest[0].(*string)
	if !ok {
		return errors.New("invalid dest")
	}
	*ptr = r.token
	return nil
}

func TestGeminiAPIKey(t *testing.T) {
	store := NewStore(&stubExecutor{token: " abc123 "})
	key, err := store.GeminiAPIKey(context.Background())
	if err != nil {
		t.Fatalf("GeminiAPIKey error: %v", err)
	}
	if key != "abc123" {
		t.Fatalf("expected abc123, got %q", key)
	}
}

func TestGeminiAPIKey_NoRows(t *testing.T) {
	store := NewStore(&stubExecutor{err: pgx.ErrNoRows})
	key, err := store.GeminiAPIKey(context.Background())
	if err != nil {
		t.Fatalf("GeminiAPIKey error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected empty key, got %q", key)
	}
}

func TestSetGeminiAPIKey(t *testing.T) {
	exec := &stubExecutor{}
	store := NewStore(exec)
	if err := store.SetGeminiAPIKey(context.Background(), "secret"); err != nil {
		t.Fatalf("SetGeminiAPIKey error: %v", err)
	}
	if len(exec.exec.args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(exec.exec.args))
	}
	if v, ok := exec.exec.args[1].(string); !ok || v != "secret" {
		t.Fatalf("expected secret argument, got %T %v", exec.exec.args[1], exec.exec.args[1])
	}
}

func TestSetGeminiAPIKeyEmpty(t *testing.T) {
	store := NewStore(&stubExecutor{})
	if err := store.SetGeminiAPIKey(context.Background(), " "); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestOpenAIAPIKey(t *testing.T) {
	store := NewStore(&stubExecutor{token: " sk-test "})
	key, err := store.OpenAIAPIKey(context.Background())
	if err != nil {
		t.Fatalf("OpenAIAPIKey error: %v", err)
	}
	if key != "sk-test" {
		t.Fatalf("expected sk-test, got %q", key)
	}
}

func TestOpenAIAPIKey_NoRows(t *testing.T) {
	store := NewStore(&stubExecutor{err: pgx.ErrNoRows})
	key, err := store.OpenAIAPIKey(context.Background())
	if err != nil {
		t.Fatalf("OpenAIAPIKey error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected empty key, got %q", key)
	}
}

func TestSetOpenAIAPIKey(t *testing.T) {
	exec := &stubExecutor{}
	store := NewStore(exec)
	if err := store.SetOpenAIAPIKey(context.Background(), "secret"); err != nil {
		t.Fatalf("SetOpenAIAPIKey error: %v", err)
	}
	if len(exec.exec.args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(exec.exec.args))
	}
	if v, ok := exec.exec.args[1].(string); !ok || v != "secret" {
		t.Fatalf("expected secret argument, got %T %v", exec.exec.args[1], exec.exec.args[1])
	}
}

func TestSetOpenAIAPIKeyEmpty(t *testing.T) {
	store := NewStore(&stubExecutor{})
	if err := store.SetOpenAIAPIKey(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestQwenAPIKey(t *testing.T) {
	store := NewStore(&stubExecutor{token: " sk-qwen "})
	key, err := store.QwenAPIKey(context.Background())
	if err != nil {
		t.Fatalf("QwenAPIKey error: %v", err)
	}
	if key != "sk-qwen" {
		t.Fatalf("expected sk-qwen, got %q", key)
	}
}

func TestSetQwenAPIKey(t *testing.T) {
	exec := &stubExecutor{}
	store := NewStore(exec)
	if err := store.SetQwenAPIKey(context.Background(), "secret"); err != nil {
		t.Fatalf("SetQwenAPIKey error: %v", err)
	}
	if len(exec.exec.args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(exec.exec.args))
	}
	if v, ok := exec.exec.args[1].(string); !ok || v != "secret" {
		t.Fatalf("expected secret argument, got %T %v", exec.exec.args[1], exec.exec.args[1])
	}
}

func TestSetQwenAPIKeyEmpty(t *testing.T) {
	store := NewStore(&stubExecutor{})
	if err := store.SetQwenAPIKey(context.Background(), " "); err == nil {
		t.Fatal("expected error for empty key")
	}
}
