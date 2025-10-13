package image

import (
	"context"
	"errors"
	"strings"
	"testing"

	"server/internal/providers/qwen"
)

type stubQwenClient struct {
	asset          *qwen.ImageAsset
	err            error
	hasCredentials bool
	calls          int
	model          string
	lastReq        qwen.ImageRequest
	requests       []qwen.ImageRequest
	queue          []stubQwenResponse
}

func (s *stubQwenClient) GenerateImage(ctx context.Context, req qwen.ImageRequest) (*qwen.ImageAsset, error) {
	s.calls++
	s.lastReq = req
	s.requests = append(s.requests, req)
	if len(s.queue) > 0 {
		next := s.queue[0]
		s.queue = s.queue[1:]
		if next.err != nil {
			return nil, next.err
		}
		if next.asset != nil {
			return next.asset, nil
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.asset, nil
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

type stubQwenResponse struct {
	asset *qwen.ImageAsset
	err   error
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

func TestQwenGeneratorFallsBackOnInternalError(t *testing.T) {
	fallback := &stubGenerator{assets: []Asset{{URL: "synthetic"}}}
	client := &stubQwenClient{
		hasCredentials: true,
		err:            errors.New("qwen: The request processing has failed due to some unknown error. (InternalError)"),
	}
	gen := NewQwenGenerator(client, fallback)
	assets, err := gen.Generate(context.Background(), GenerateRequest{Prompt: "sample"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected qwen client to be invoked twice, got %d", client.calls)
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

func TestQwenGeneratorRetriesWithSimplifiedPayload(t *testing.T) {
	generated := &qwen.ImageAsset{URL: "https://example.com/image.png", Format: "image/png", Width: 1024, Height: 1024}
	client := &stubQwenClient{
		hasCredentials: true,
		queue: []stubQwenResponse{
			{err: errors.New("qwen: status 400: invalid parameter locale")},
			{asset: generated},
		},
	}
	gen := NewQwenGenerator(client, nil)
	req := GenerateRequest{
		Prompt:         "hello",
		NegativePrompt: "avoid",
		Quality:        "hd",
		Locale:         "id",
		Workflow: Workflow{
			Mode:  WorkflowModeGenerate,
			Notes: "colour balance",
		},
	}
	assets, err := gen.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected 2 qwen calls, got %d", len(client.requests))
	}
	first := client.requests[0]
	if first.NegativePrompt == "" {
		t.Fatalf("first request should include negative prompt")
	}
	second := client.requests[1]
	if second.NegativePrompt != "" {
		t.Fatalf("second request should clear negative prompt, got %q", second.NegativePrompt)
	}
	if second.Workflow != (qwen.Workflow{}) {
		t.Fatalf("second request workflow should be empty, got %#v", second.Workflow)
	}
	if second.Quality != "" {
		t.Fatalf("second request should clear quality, got %q", second.Quality)
	}
	if second.Locale != "" {
		t.Fatalf("second request should clear locale, got %q", second.Locale)
	}
	if second.Seed != 0 {
		t.Fatalf("second request should clear seed, got %d", second.Seed)
	}
	if len(assets) != 1 || assets[0].URL != generated.URL {
		t.Fatalf("unexpected assets: %#v", assets)
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

func TestQwenGeneratorDefaultsEnhanceWorkflowForSourceImage(t *testing.T) {
	generated := &qwen.ImageAsset{URL: "https://example.com/image.png", Format: "image/png", Width: 1024, Height: 1024}
	client := &stubQwenClient{hasCredentials: true, asset: generated}
	gen := NewQwenGenerator(client, nil)
	req := GenerateRequest{
		Prompt: "hello",
		SourceImage: &SourceImage{
			Data: []byte{0x01, 0x02},
			MIME: "image/png",
		},
	}
	if _, err := gen.Generate(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.lastReq.SourceImage == nil {
		t.Fatalf("source image not forwarded to qwen client")
	}
	if got := strings.TrimSpace(client.lastReq.Workflow.Mode); got != string(WorkflowModeEnhance) {
		t.Fatalf("workflow mode = %q, want %q", got, WorkflowModeEnhance)
	}
}
