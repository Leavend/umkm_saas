package prompt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"server/internal/domain/jsoncfg"
)

const (
	staticProviderName = "static"
	geminiProviderName = "gemini"
	openAIProviderName = "openai"
)

type modelEnhancePayload struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Keywords    []string           `json:"keywords"`
	Ideas       []modelIdeaPayload `json:"ideas"`
	Metadata    map[string]string  `json:"metadata"`
}

type modelIdeaPayload struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

type modelRandomPayload struct {
	Items  []modelIdeaPayload `json:"items"`
	Locale string             `json:"locale"`
}

func buildEnhancePromptPayload(req EnhanceRequest) string {
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

func buildRandomPromptPayload(locale string) string {
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

func parseModelPayload[T any](raw string) (T, error) {
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
