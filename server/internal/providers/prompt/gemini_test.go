package prompt

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"server/internal/domain/jsoncfg"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type fakeEnhancer struct {
	enhance func(context.Context, EnhanceRequest) (*EnhanceResponse, error)
	random  func(context.Context, string) ([]EnhanceResponse, error)
}

func (f fakeEnhancer) Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	if f.enhance != nil {
		return f.enhance(ctx, req)
	}
	return nil, errors.New("enhance not implemented")
}

func (f fakeEnhancer) Random(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	if f.random != nil {
		return f.random(ctx, locale)
	}
	return nil, errors.New("random not implemented")
}

func TestGeminiEnhancerFallbackMetadata(t *testing.T) {
	fallback := NewStaticEnhancer()
	var capturedReason string
	enhancer, err := NewGeminiEnhancer(GeminiOptions{
		APIKey: "dummy",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
		Fallback: fallback,
		OnFallback: func(reason string, err error) {
			capturedReason = reason
		},
	})
	if err != nil {
		t.Fatalf("NewGeminiEnhancer returned error: %v", err)
	}
	req := EnhanceRequest{Prompt: jsoncfg.PromptJSON{Title: "", ProductType: "food"}, Locale: "id"}
	res, err := enhancer.Enhance(context.Background(), req)
	if err != nil {
		t.Fatalf("Enhance returned error: %v", err)
	}
	if res.Provider != staticProviderName {
		t.Fatalf("Provider = %q, want %q", res.Provider, staticProviderName)
	}
	if res.Metadata["fallback_reason"] != "http_request" {
		t.Fatalf("fallback_reason = %q, want %q", res.Metadata["fallback_reason"], "http_request")
	}
	if capturedReason != "http_request" {
		t.Fatalf("captured reason = %q, want %q", capturedReason, "http_request")
	}
}

func TestGeminiEnhancerFallsBackToChainedProvider(t *testing.T) {
	t.Helper()
	fallback := fakeEnhancer{
		enhance: func(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
			return &EnhanceResponse{Provider: openAIProviderName, Metadata: map[string]string{}}, nil
		},
	}
	enhancer, err := NewGeminiEnhancer(GeminiOptions{
		APIKey: "dummy",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})},
		Fallback: fallback,
	})
	if err != nil {
		t.Fatalf("NewGeminiEnhancer returned error: %v", err)
	}
	res, err := enhancer.Enhance(context.Background(), EnhanceRequest{Prompt: jsoncfg.PromptJSON{}, Locale: "en"})
	if err != nil {
		t.Fatalf("Enhance returned error: %v", err)
	}
	if res.Provider != openAIProviderName {
		t.Fatalf("Provider = %q, want %q", res.Provider, openAIProviderName)
	}
	if res.Metadata["fallback_reason"] == "" {
		t.Fatal("expected fallback_reason metadata to be populated")
	}
}
