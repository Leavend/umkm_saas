package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"server/internal/infra"
	"server/internal/sqlinline"
)

const (
	ProviderGemini = "gemini"
)

type Store struct {
	sql infra.SQLExecutor
}

func NewStore(sql infra.SQLExecutor) *Store {
	return &Store{sql: sql}
}

func (s *Store) GeminiAPIKey(ctx context.Context) (string, error) {
	return s.Token(ctx, ProviderGemini)
}

func (s *Store) Token(ctx context.Context, provider string) (string, error) {
	row := s.sql.QueryRow(ctx, sqlinline.QSelectIntegrationToken, provider)
	var token string
	if err := row.Scan(&token); err != nil {
		if infra.IsNoRows(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(token), nil
}

func (s *Store) SetGeminiAPIKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("gemini api key is required")
	}
	return s.upsert(ctx, ProviderGemini, key, nil)
}

func (s *Store) upsert(ctx context.Context, provider, token string, props map[string]any) error {
	payload := props
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.sql.Exec(ctx, sqlinline.QUpsertIntegrationToken, provider, token, raw)
	return err
}
