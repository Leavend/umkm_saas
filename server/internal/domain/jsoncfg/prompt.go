package jsoncfg

import (
	"encoding/json"
	"fmt"
)

type WatermarkConfig struct {
	Enabled  bool   `json:"enabled"`
	Text     string `json:"text"`
	Position string `json:"position"`
}

type ExtrasConfig struct {
	Locale  string `json:"locale"`
	Quality string `json:"quality"`
}

type PromptJSON struct {
	Version      string          `json:"version"`
	Title        string          `json:"title"`
	ProductType  string          `json:"product_type"`
	Style        string          `json:"style"`
	Background   string          `json:"background"`
	Instructions string          `json:"instructions"`
	Watermark    WatermarkConfig `json:"watermark"`
	AspectRatio  string          `json:"aspect_ratio"`
	Quantity     int             `json:"quantity"`
	References   []string        `json:"references"`
	Extras       ExtrasConfig    `json:"extras"`
}

const (
	// DefaultPromptVersion represents the schema version persisted for prompts.
	DefaultPromptVersion = "2024-01"
	// DefaultPromptAspectRatio is used when the request omits the aspect ratio.
	DefaultPromptAspectRatio = "1:1"
	// DefaultPromptQuantity is the minimum quantity allowed for free users.
	DefaultPromptQuantity = 1
	// MaxPromptQuantity enforces the free tier cap for generated assets.
	MaxPromptQuantity = 2
	// DefaultExtrasLocale is applied when no locale preference is provided.
	DefaultExtrasLocale = "en"
	// DefaultExtrasQuality represents the baseline generation quality.
	DefaultExtrasQuality = "standard"
)

// Normalize ensures the prompt JSON respects server defaults and limits.
func (p *PromptJSON) Normalize(preferredLocale string) {
	if p == nil {
		return
	}
	if p.Version == "" {
		p.Version = DefaultPromptVersion
	}
	if p.Quantity <= 0 {
		p.Quantity = DefaultPromptQuantity
	}
	if p.Quantity > MaxPromptQuantity {
		p.Quantity = MaxPromptQuantity
	}
	if p.AspectRatio == "" {
		p.AspectRatio = DefaultPromptAspectRatio
	}
	if p.Extras.Locale == "" {
		if preferredLocale != "" {
			p.Extras.Locale = preferredLocale
		} else {
			p.Extras.Locale = DefaultExtrasLocale
		}
	}
	if p.Extras.Quality == "" {
		p.Extras.Quality = DefaultExtrasQuality
	}
}

type EnhanceOptions struct {
	Target string `json:"target"`
	Notes  string `json:"notes"`
}

type IdeaSuggestion struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type UsageEventPayload struct {
	EventType string `json:"event_type"`
	Provider  string `json:"provider"`
	Success   bool   `json:"success"`
}

func MustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("json marshal: %w", err))
	}
	return b
}
