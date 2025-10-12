package image

import (
	"context"
	"errors"
	"testing"

	"server/internal/providers/qwen"
)

type stubQwenClient struct {
	asset          *qwen.ImageAsset
	err            error
	hasCredentials bool
	calls          int
	model          string
}

func (s *stubQwenClient) GenerateImage(ctx context.Context, req qwen.ImageRequest) (*qwen.ImageAsset, error) {
	s.calls++
	return s.asset, s.err
}

func (s *stubQwenClient) HasCredentials() bool {
	return s.hasCredentials
}

func (s *stubQwenClient) Model() string {
	if s.model != "" {
		return s.model
	}
	return "qwen-image-plus"
}

type stubGenerator struct {
	assets  []Asset
	err     error
	calls   int
	lastReq GenerateRequest
}

func (s *stubGenerator) Generate(ctx context.Context, req GenerateRequest) ([]Asset, error) {
	s.calls++
	s.lastReq = req
	return s.assets, s.err
}

func TestQwenGeneratorFallsBackWhenNoCredentials(t *testing.T) {
	fallback := &stubGenerator{assets: []Asset{{URL: "fallback"}}}
	client := &stubQwenClient{hasCredentials: false}

	gen := NewQwenGenerator(client, fallback)
	assets, err := gen.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
	if client.calls != 0 {
		t.Fatalf("qwen client should not be invoked without credentials")
	}
	if len(assets) != 1 || assets[0].URL != "fallback" {
		t.Fatalf("unexpected assets: %#v", assets)
	}
}

func TestQwenGeneratorFallsBackOnMissingAPIKeyError(t *testing.T) {
	fallback := &stubGenerator{assets: []Asset{{URL: "synthetic"}}}
	client := &stubQwenClient{
		hasCredentials: true,
		err:            qwen.ErrMissingAPIKey,
	}
	gen := NewQwenGenerator(client, fallback)
	assets, err := gen.Generate(context.Background(), GenerateRequest{Prompt: "sample"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected qwen client to be invoked once, got %d", client.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
	if len(assets) != 1 || assets[0].URL != "synthetic" {
		t.Fatalf("unexpected assets: %#v", assets)
	}
}

func TestQwenGeneratorReturnsErrorWhenRemoteFails(t *testing.T) {
	fallback := &stubGenerator{assets: []Asset{{URL: "unused"}}}
	client := &stubQwenClient{
		hasCredentials: true,
		err:            errors.New("qwen: status 429: rate limited"),
	}
	gen := NewQwenGenerator(client, fallback)
	_, err := gen.Generate(context.Background(), GenerateRequest{Prompt: "sample"})
	if err == nil {
		t.Fatalf("expected error from qwen generator")
	}
	if !errors.Is(err, client.err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback should not be invoked on generation errors")
	}
}

func TestQwenGeneratorSuccess(t *testing.T) {
	generated := &qwen.ImageAsset{URL: "https://example.com/image.png", Format: "image/png", Width: 1024, Height: 1024}
	client := &stubQwenClient{hasCredentials: true, asset: generated}
	gen := NewQwenGenerator(client, nil)
	assets, err := gen.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1", len(assets))
	}
	if assets[0].URL != generated.URL {
		t.Fatalf("asset url = %s, want %s", assets[0].URL, generated.URL)
	}
	if client.calls != 1 {
		t.Fatalf("qwen client calls = %d, want 1", client.calls)
	}
}
