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
