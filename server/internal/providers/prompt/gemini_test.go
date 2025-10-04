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
