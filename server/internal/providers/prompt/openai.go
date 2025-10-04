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

type OpenAIOptions struct {
	APIKey       string
	Model        string
	BaseURL      string
	Organization string
	HTTPClient   *http.Client
	Fallback     Enhancer
	OnFallback   func(reason string, err error)
}

type OpenAIEnhancer struct {
	apiKey       string
	model        string
	baseURL      string
	organization string
	client       *http.Client
	fallback     Enhancer
	onFallback   func(reason string, err error)
}

const openAIDefaultTimeout = 15 * time.Second

type openAIChatRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	ResponseFormat *openAIFormat   `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewOpenAIEnhancer(opts OpenAIOptions) (*OpenAIEnhancer, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, errors.New("openai api key is required")
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: openAIDefaultTimeout}
	}
	return &OpenAIEnhancer{
		apiKey:       strings.TrimSpace(opts.APIKey),
		model:        model,
		baseURL:      baseURL,
		organization: strings.TrimSpace(opts.Organization),
		client:       client,
		fallback:     opts.Fallback,
		onFallback:   opts.OnFallback,
	}, nil
}

func (o *OpenAIEnhancer) Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	if o.apiKey == "" {
		return o.useFallback(ctx, req, "missing_api_key", nil)
	}
	payload := openAIChatRequest{
		Model:       o.model,
		Temperature: 0.6,
		ResponseFormat: &openAIFormat{
			Type: "json_object",
		},
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a helpful marketing prompt assistant that only responds with valid JSON."},
			{Role: "user", Content: buildEnhancePromptPayload(req)},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return o.useFallback(ctx, req, "encode_request", err)
	}
	endpoint := fmt.Sprintf("%s/chat/completions", o.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return o.useFallback(ctx, req, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	if o.organization != "" {
		httpReq.Header.Set("OpenAI-Organization", o.organization)
	}
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return o.useFallback(ctx, req, "http_request", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return o.useFallback(ctx, req, fmt.Sprintf("http_%d", resp.StatusCode), fmt.Errorf("openai status %d", resp.StatusCode))
	}
	var out openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return o.useFallback(ctx, req, "decode_response", err)
	}
	if len(out.Choices) == 0 {
		return o.useFallback(ctx, req, "empty_choices", errors.New("no choices"))
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return o.useFallback(ctx, req, "empty_response", errors.New("empty response"))
	}
	parsed, err := parseModelPayload[modelEnhancePayload](text)
	if err != nil {
		return o.useFallback(ctx, req, "parse_payload", err)
	}
	locale := coalesce(req.Locale, req.Prompt.Extras.Locale)
	response := &EnhanceResponse{
		Title:       coalesce(parsed.Title, req.Prompt.Title),
		Description: coalesce(parsed.Description, req.Prompt.Instructions),
		Keywords:    normalizeKeywords(parsed.Keywords, req.Prompt.ProductType),
		Metadata:    ensureMetadata(parsed.Metadata, locale),
		Provider:    openAIProviderName,
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

func (o *OpenAIEnhancer) Random(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	if o.apiKey == "" {
		return o.useFallbackRandom(ctx, locale, "missing_api_key", nil)
	}
	payload := openAIChatRequest{
		Model:       o.model,
		Temperature: 0.8,
		ResponseFormat: &openAIFormat{
			Type: "json_object",
		},
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a helpful marketing prompt assistant that only responds with valid JSON."},
			{Role: "user", Content: buildRandomPromptPayload(locale)},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return o.useFallbackRandom(ctx, locale, "encode_request", err)
	}
	endpoint := fmt.Sprintf("%s/chat/completions", o.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return o.useFallbackRandom(ctx, locale, "build_request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	if o.organization != "" {
		httpReq.Header.Set("OpenAI-Organization", o.organization)
	}
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return o.useFallbackRandom(ctx, locale, "http_request", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return o.useFallbackRandom(ctx, locale, fmt.Sprintf("http_%d", resp.StatusCode), fmt.Errorf("openai status %d", resp.StatusCode))
	}
	var out openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return o.useFallbackRandom(ctx, locale, "decode_response", err)
	}
	if len(out.Choices) == 0 {
		return o.useFallbackRandom(ctx, locale, "empty_choices", errors.New("no choices"))
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return o.useFallbackRandom(ctx, locale, "empty_response", errors.New("empty response"))
	}
	parsed, err := parseModelPayload[modelRandomPayload](text)
	if err != nil {
		return o.useFallbackRandom(ctx, locale, "parse_payload", err)
	}
	if len(parsed.Items) == 0 {
		return o.useFallbackRandom(ctx, locale, "empty_items", errors.New("no items"))
	}
	var items []EnhanceResponse
	for _, item := range parsed.Items {
		meta := ensureMetadata(map[string]string{"locale": parsed.Locale}, locale)
		res := EnhanceResponse{
			Title:       coalesce(item.Title, item.Description),
			Description: coalesce(item.Description, item.Title),
			Keywords:    normalizeKeywords(item.Keywords, item.Title),
			Metadata:    meta,
			Provider:    openAIProviderName,
		}
		items = append(items, res)
	}
	return items, nil
}

func (o *OpenAIEnhancer) useFallback(ctx context.Context, req EnhanceRequest, reason string, fallbackErr error) (*EnhanceResponse, error) {
	o.emitFallback(reason, fallbackErr)
	if o.fallback != nil {
		res, err := o.fallback.Enhance(ctx, req)
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

func (o *OpenAIEnhancer) useFallbackRandom(ctx context.Context, locale string, reason string, fallbackErr error) ([]EnhanceResponse, error) {
	o.emitFallback(reason, fallbackErr)
	if o.fallback != nil {
		items, err := o.fallback.Random(ctx, locale)
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

func (o *OpenAIEnhancer) emitFallback(reason string, err error) {
	if o.onFallback != nil {
		o.onFallback(reason, err)
	}
}

var _ Enhancer = (*OpenAIEnhancer)(nil)
