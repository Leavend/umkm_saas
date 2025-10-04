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

	"server/internal/domain/jsoncfg"
)

type GeminiOptions struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	Fallback   Enhancer
}

type GeminiEnhancer struct {
	apiKey   string
	model    string
	baseURL  string
	client   *http.Client
	fallback Enhancer
}

const (
	geminiDefaultTimeout = 15 * time.Second
	geminiProviderName   = "gemini"
	staticProviderName   = "static"
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

type geminiEnhancePayload struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Keywords    []string            `json:"keywords"`
	Ideas       []geminiIdeaPayload `json:"ideas"`
	Metadata    map[string]string   `json:"metadata"`
}

type geminiIdeaPayload struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

type geminiRandomPayload struct {
	Items  []geminiIdeaPayload `json:"items"`
	Locale string              `json:"locale"`
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
		apiKey:   opts.APIKey,
		model:    model,
		baseURL:  baseURL,
		client:   client,
		fallback: opts.Fallback,
	}, nil
}

func (g *GeminiEnhancer) Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	if g.apiKey == "" {
		return g.useFallback(ctx, req)
	}
	payload := geminiRequest{
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{{
				Text: g.buildEnhancePrompt(req),
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
		return g.useFallback(ctx, req)
	}
	endpoint := g.endpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallback(ctx, req)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return g.useFallback(ctx, req)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return g.useFallback(ctx, req)
	}
	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return g.useFallback(ctx, req)
	}
	text := g.extractText(out)
	if text == "" {
		return g.useFallback(ctx, req)
	}
	parsed, err := parseGeminiPayload[geminiEnhancePayload](text)
	if err != nil {
		return g.useFallback(ctx, req)
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
		return g.useFallbackRandom(ctx, locale)
	}
	payload := geminiRequest{
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{{
				Text: g.buildRandomPrompt(locale),
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
		return g.useFallbackRandom(ctx, locale)
	}
	endpoint := g.endpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return g.useFallbackRandom(ctx, locale)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return g.useFallbackRandom(ctx, locale)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return g.useFallbackRandom(ctx, locale)
	}
	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return g.useFallbackRandom(ctx, locale)
	}
	text := g.extractText(out)
	if text == "" {
		return g.useFallbackRandom(ctx, locale)
	}
	parsed, err := parseGeminiPayload[geminiRandomPayload](text)
	if err != nil {
		return g.useFallbackRandom(ctx, locale)
	}
	if len(parsed.Items) == 0 {
		return g.useFallbackRandom(ctx, locale)
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

func (g *GeminiEnhancer) useFallback(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	if g.fallback != nil {
		res, err := g.fallback.Enhance(ctx, req)
		if res != nil {
			res.Provider = staticProviderName
		}
		return res, err
	}
	fallback := NewStaticEnhancer()
	res, err := fallback.Enhance(ctx, req)
	if res != nil {
		res.Provider = staticProviderName
	}
	return res, err
}

func (g *GeminiEnhancer) useFallbackRandom(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	if g.fallback != nil {
		items, err := g.fallback.Random(ctx, locale)
		for i := range items {
			items[i].Provider = staticProviderName
		}
		return items, err
	}
	fallback := NewStaticEnhancer()
	items, err := fallback.Random(ctx, locale)
	for i := range items {
		items[i].Provider = staticProviderName
	}
	return items, err
}

func (g *GeminiEnhancer) buildEnhancePrompt(req EnhanceRequest) string {
	p := req.Prompt
	locale := req.Locale
	if locale == "" {
		locale = p.Extras.Locale
	}
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "You are a marketing prompt expert helping Indonesian small businesses. Respond strictly with JSON matching this schema: ")
	sb.WriteString(`{"title":string,"description":string,"keywords":string[],"ideas":[{"title":string,"description":string,"keywords":string[]}],"metadata":{"locale":string}}`)
	fmt.Fprintf(sb, ". Use locale '%s' for language choices. Input details: title=%q, product_type=%q, style=%q, background=%q, instructions=%q, watermark_enabled=%t. Focus on persuasive yet concise copy.", locale, p.Title, p.ProductType, p.Style, p.Background, p.Instructions, p.Watermark.Enabled)
	return sb.String()
}

func (g *GeminiEnhancer) buildRandomPrompt(locale string) string {
	if locale == "" {
		locale = "en"
	}
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "Generate three unique product marketing prompt ideas for small businesses. Respond strictly as JSON: {\"items\":[{\"title\":string,\"description\":string,\"keywords\":string[]}],\"locale\":%q}. Use locale '%s' for language and make each response noticeably different. randomness_token=%d.", locale, locale, time.Now().UnixNano())
	return sb.String()
}

func ensureMetadata(meta map[string]string, locale string) map[string]string {
	if meta == nil {
		meta = map[string]string{}
	}
	if locale != "" {
		meta["locale"] = locale
	} else if _, ok := meta["locale"]; !ok {
		meta["locale"] = jsoncfg.DefaultExtrasLocale
	}
	return meta
}

func normalizeKeywords(keywords []string, fallback string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		kwLower := strings.ToLower(kw)
		if _, ok := seen[kwLower]; ok {
			continue
		}
		seen[kwLower] = struct{}{}
		result = append(result, kw)
	}
	if len(result) == 0 && fallback != "" {
		result = []string{fallback}
	}
	return result
}

func coalesce(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func parseGeminiPayload[T any](raw string) (T, error) {
	var zero T
	cleaned := extractJSONFragment(raw)
	if cleaned == "" {
		return zero, errors.New("empty payload")
	}
	var decoded T
	if err := json.Unmarshal([]byte(cleaned), &decoded); err != nil {
		return zero, err
	}
	return decoded, nil
}

func extractJSONFragment(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	text = trimCodeFence(text)
	start := strings.IndexAny(text, "{[")
	end := strings.LastIndexAny(text, "]}")
	if start >= 0 && end >= start {
		text = text[start : end+1]
	}
	return strings.TrimSpace(text)
}

func trimCodeFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```JSON")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
}

var _ Enhancer = (*GeminiEnhancer)(nil)
