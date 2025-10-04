package prompt

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"server/internal/domain/jsoncfg"
)

func TestOpenAIEnhancerFallbackMetadata(t *testing.T) {
	fallback := NewStaticEnhancer()
	var capturedReason string
	enhancer, err := NewOpenAIEnhancer(OpenAIOptions{
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
		t.Fatalf("NewOpenAIEnhancer returned error: %v", err)
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

func TestNormalizeOpenAIModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		model  string
		reason string
	}{
		{name: "exact_default", input: "gpt-4o-mini", model: "gpt-4o-mini", reason: ""},
		{name: "exact_free", input: "gpt-3.5-turbo", model: "gpt-3.5-turbo", reason: ""},
		{name: "alias_short", input: "gpt-3.5", model: "gpt-3.5-turbo", reason: "alias"},
		{name: "alias_spaces", input: "GPT 5 Thinking", model: "gpt-4o-mini", reason: "alias"},
		{name: "unsupported", input: "gpt-4.1", model: "gpt-4o-mini", reason: "defaulted"},
		{name: "empty", input: "", model: "gpt-4o-mini", reason: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotModel, gotReason := normalizeOpenAIModel(tc.input)
			if gotModel != tc.model {
				t.Fatalf("model = %q, want %q", gotModel, tc.model)
			}
			if gotReason != tc.reason {
				t.Fatalf("reason = %q, want %q", gotReason, tc.reason)
			}
		})
	}
}

func TestNewOpenAIEnhancerWarnsOnUnsupportedModel(t *testing.T) {
	t.Parallel()
	fallback := NewStaticEnhancer()
	var capturedReason, capturedDetail string
	enhancer, err := NewOpenAIEnhancer(OpenAIOptions{
		APIKey:   "dummy",
		Model:    "gpt-5 thinking",
		Fallback: fallback,
		OnWarning: func(reason, detail string) {
			capturedReason = reason
			capturedDetail = detail
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enhancer == nil {
		t.Fatal("enhancer is nil")
	}
	if capturedReason != "model_alias" {
		t.Fatalf("warning reason = %q, want %q", capturedReason, "model_alias")
	}
	if capturedDetail == "" {
		t.Fatal("expected warning detail to be set")
	}
}
