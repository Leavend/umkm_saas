package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
)

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	CandidateCount   int     `json:"candidateCount,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func NewGeminiEnhancer(opts GeminiOptions) (*GeminiEnhancer, error) {
	if opts.APIKey == "" {
		return nil, errors.New("gemini api key is required")
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "gemini-1.5-flash"
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: geminiDefaultTimeout}
	}
	return &GeminiEnhancer{
		apiKey:     opts.APIKey,
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
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{{
				Text: buildEnhancePromptPayload(req),
			}},
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:      0.5,
			CandidateCount:   1,
			ResponseMimeType: "application/json",
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return g.useFallback(ctx, req, "encode_request", err)
	}
	endpoint := g.endpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallback(ctx, req, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)
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
	text := g.extractText(out)
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
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{{
				Text: buildRandomPromptPayload(locale),
			}},
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:      0.7,
			CandidateCount:   1,
			ResponseMimeType: "application/json",
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return g.useFallbackRandom(ctx, locale, "encode_request", err)
	}
	endpoint := g.endpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallbackRandom(ctx, locale, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)
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
	text := g.extractText(out)
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

func (g *GeminiEnhancer) endpoint() string {
	base := strings.TrimRight(g.baseURL, "/")
	model := url.PathEscape(g.model)
	return fmt.Sprintf("%s/models/%s:generateContent?key=%s", base, model, url.QueryEscape(g.apiKey))
}

func (g *GeminiEnhancer) extractText(resp geminiResponse) string {
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text
			}
		}
	}
	return ""
}

func (g *GeminiEnhancer) useFallback(ctx context.Context, req EnhanceRequest, reason string, fallbackErr error) (*EnhanceResponse, error) {
	g.emitFallback(reason, fallbackErr)
	if g.fallback != nil {
		res, err := g.fallback.Enhance(ctx, req)
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
