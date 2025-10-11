package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GeminiOptions struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	Fallback   Enhancer
	OnFallback func(reason string, err error)
}

type GeminiEnhancer struct {
	apiKey     string
	model      string
	baseURL    string
	client     *http.Client
	fallback   Enhancer
	onFallback func(reason string, err error)
}

const (
	geminiDefaultTimeout = 15 * time.Second
	geminiDefaultModel   = "google/gemini-2.0-flash-exp:free"
	geminiDefaultBaseURL = "https://openrouter.ai/api/v1"
)

type geminiRequest struct {
	Model          string                `json:"model"`
	Messages       []geminiMessage       `json:"messages"`
	Temperature    float64               `json:"temperature,omitempty"`
	ResponseFormat *geminiResponseFormat `json:"response_format,omitempty"`
}

type geminiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type geminiResponseFormat struct {
	Type string `json:"type"`
}

type geminiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewGeminiEnhancer(opts GeminiOptions) (*GeminiEnhancer, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, errors.New("gemini api key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = geminiDefaultBaseURL
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = geminiDefaultModel
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: geminiDefaultTimeout}
	}
	return &GeminiEnhancer{
		apiKey:     strings.TrimSpace(opts.APIKey),
		model:      model,
		baseURL:    baseURL,
		client:     client,
		fallback:   opts.Fallback,
		onFallback: opts.OnFallback,
	}, nil
}

func (g *GeminiEnhancer) Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	if g.apiKey == "" {
		return g.useFallback(ctx, req, "missing_api_key", nil)
	}
	payload := geminiRequest{
		Model:       g.model,
		Temperature: 0.5,
		ResponseFormat: &geminiResponseFormat{
			Type: "json_object",
		},
		Messages: []geminiMessage{
			{Role: "system", Content: "You are a helpful marketing assistant that always responds with valid JSON."},
			{Role: "user", Content: buildEnhancePromptPayload(req)},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return g.useFallback(ctx, req, "encode_request", err)
	}
	endpoint := fmt.Sprintf("%s/chat/completions", g.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallback(ctx, req, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return g.useFallback(ctx, req, "http_request", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return g.useFallback(ctx, req, fmt.Sprintf("http_%d", resp.StatusCode), fmt.Errorf("gemini status %d", resp.StatusCode))
	}
	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return g.useFallback(ctx, req, "decode_response", err)
	}
	if len(out.Choices) == 0 {
		return g.useFallback(ctx, req, "empty_choices", errors.New("no choices"))
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return g.useFallback(ctx, req, "empty_response", errors.New("empty response"))
	}
	parsed, err := parseModelPayload[modelEnhancePayload](text)
	if err != nil {
		return g.useFallback(ctx, req, "parse_payload", err)
	}
	response := &EnhanceResponse{
		Title:       coalesce(parsed.Title, req.Prompt.Title),
		Description: coalesce(parsed.Description, req.Prompt.Instructions),
		Keywords:    normalizeKeywords(parsed.Keywords, req.Prompt.ProductType),
		Metadata:    ensureMetadata(parsed.Metadata, req.Locale),
		Provider:    geminiProviderName,
	}
	if len(parsed.Ideas) > 0 {
		for _, idea := range parsed.Ideas {
			response.Ideas = append(response.Ideas, EnhanceIdea{
				Title:       coalesce(idea.Title, response.Title),
				Description: coalesce(idea.Description, response.Description),
				Keywords:    normalizeKeywords(idea.Keywords, req.Prompt.ProductType),
			})
		}
	}
	if len(response.Ideas) == 0 {
		response.Ideas = append(response.Ideas, EnhanceIdea{
			Title:       response.Title,
			Description: response.Description,
			Keywords:    response.Keywords,
		})
	}
	return response, nil
}

func (g *GeminiEnhancer) Random(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	if g.apiKey == "" {
		return g.useFallbackRandom(ctx, locale, "missing_api_key", nil)
	}
	payload := geminiRequest{
		Model:       g.model,
		Temperature: 0.7,
		ResponseFormat: &geminiResponseFormat{
			Type: "json_object",
		},
		Messages: []geminiMessage{
			{Role: "system", Content: "You are a helpful marketing assistant that always responds with valid JSON."},
			{Role: "user", Content: buildRandomPromptPayload(locale)},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return g.useFallbackRandom(ctx, locale, "encode_request", err)
	}
	endpoint := fmt.Sprintf("%s/chat/completions", g.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallbackRandom(ctx, locale, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return g.useFallbackRandom(ctx, locale, "http_request", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return g.useFallbackRandom(ctx, locale, fmt.Sprintf("http_%d", resp.StatusCode), fmt.Errorf("gemini status %d", resp.StatusCode))
	}
	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return g.useFallbackRandom(ctx, locale, "decode_response", err)
	}
	if len(out.Choices) == 0 {
		return g.useFallbackRandom(ctx, locale, "empty_choices", errors.New("no choices"))
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return g.useFallbackRandom(ctx, locale, "empty_response", errors.New("empty response"))
	}
	parsed, err := parseModelPayload[modelRandomPayload](text)
	if err != nil {
		return g.useFallbackRandom(ctx, locale, "parse_payload", err)
	}
	if len(parsed.Items) == 0 {
		return g.useFallbackRandom(ctx, locale, "empty_items", errors.New("no items returned"))
	}
	var results []EnhanceResponse
	for _, item := range parsed.Items {
		meta := ensureMetadata(map[string]string{"locale": parsed.Locale}, locale)
		results = append(results, EnhanceResponse{
			Title:       coalesce(item.Title, ""),
			Description: coalesce(item.Description, ""),
			Keywords:    normalizeKeywords(item.Keywords, ""),
			Metadata:    meta,
			Provider:    geminiProviderName,
		})
	}
	return results, nil
}

func (g *GeminiEnhancer) useFallback(ctx context.Context, req EnhanceRequest, reason string, fallbackErr error) (*EnhanceResponse, error) {
	g.emitFallback(reason, fallbackErr)
	if g.fallback != nil {
		res, err := g.fallback.Enhance(ctx, req)
		if res != nil {
			if res.Provider == "" {
				res.Provider = staticProviderName
			}
			if res.Metadata == nil {
				res.Metadata = map[string]string{}
			}
			if reason != "" {
				res.Metadata["fallback_reason"] = reason
			}
		}
		return res, err
	}
	fallback := NewStaticEnhancer()
	res, err := fallback.Enhance(ctx, req)
	if res != nil {
		res.Provider = staticProviderName
		if res.Metadata == nil {
			res.Metadata = map[string]string{}
		}
		if reason != "" {
			res.Metadata["fallback_reason"] = reason
		}
	}
	return res, err
}

func (g *GeminiEnhancer) useFallbackRandom(ctx context.Context, locale string, reason string, fallbackErr error) ([]EnhanceResponse, error) {
	g.emitFallback(reason, fallbackErr)
	if g.fallback != nil {
		items, err := g.fallback.Random(ctx, locale)
		for i := range items {
			if items[i].Provider == "" {
				items[i].Provider = staticProviderName
			}
			if items[i].Metadata == nil {
				items[i].Metadata = map[string]string{}
			}
			if reason != "" {
				items[i].Metadata["fallback_reason"] = reason
			}
		}
		return items, err
	}
	fallback := NewStaticEnhancer()
	items, err := fallback.Random(ctx, locale)
	for i := range items {
		items[i].Provider = staticProviderName
		if items[i].Metadata == nil {
			items[i].Metadata = map[string]string{}
		}
		if reason != "" {
			items[i].Metadata["fallback_reason"] = reason
		}
	}
	return items, err
}

func (g *GeminiEnhancer) emitFallback(reason string, err error) {
	if g.onFallback != nil {
		g.onFallback(reason, err)
	}
}

var _ Enhancer = (*GeminiEnhancer)(nil)
