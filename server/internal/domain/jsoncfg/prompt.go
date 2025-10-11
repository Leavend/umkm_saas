package jsoncfg

import (
	"encoding/json"
	"fmt"
	"strings"
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

// SourceAssetConfig represents an uploaded or remote asset referenced by a prompt.
type SourceAssetConfig struct {
	AssetID    string `json:"asset_id"`
	StorageKey string `json:"storage_key"`
	URL        string `json:"url"`
	Mime       string `json:"mime"`
	Filename   string `json:"filename"`
}

// WorkflowConfig captures the editing intent selected by the user when working
// with existing assets.
type WorkflowConfig struct {
	Mode            string `json:"mode"`
	BackgroundTheme string `json:"background_theme"`
	BackgroundStyle string `json:"background_style"`
	EnhanceLevel    string `json:"enhance_level"`
	RetouchStrength string `json:"retouch_strength"`
	Notes           string `json:"notes"`
}

type PromptJSON struct {
	Version      string            `json:"version"`
	Title        string            `json:"title"`
	ProductType  string            `json:"product_type"`
	Style        string            `json:"style"`
	Background   string            `json:"background"`
	Instructions string            `json:"instructions"`
	Watermark    WatermarkConfig   `json:"watermark"`
	AspectRatio  string            `json:"aspect_ratio"`
	Quantity     int               `json:"quantity"`
	References   []string          `json:"references"`
	Extras       ExtrasConfig      `json:"extras"`
	SourceAsset  SourceAssetConfig `json:"source_asset"`
	Workflow     WorkflowConfig    `json:"workflow"`
}

var allowedAspectRatios = map[string]struct{}{
	"1:1":  {},
	"4:3":  {},
	"3:4":  {},
	"16:9": {},
	"9:16": {},
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
	// DefaultWorkflowMode is applied when the prompt does not specify an editing intent.
	DefaultWorkflowMode = WorkflowModeGenerate
)

// Workflow modes supported by the MVP image pipeline.
const (
	WorkflowModeGenerate   = "generate"
	WorkflowModeBackground = "background"
	WorkflowModeEnhance    = "enhance"
	WorkflowModeRetouch    = "retouch"
)

var allowedWorkflowModes = map[string]struct{}{
	WorkflowModeGenerate:   {},
	WorkflowModeBackground: {},
	WorkflowModeEnhance:    {},
	WorkflowModeRetouch:    {},
}

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

	p.Workflow.Mode = normalizeWorkflowMode(p.Workflow.Mode)
	p.Workflow.BackgroundTheme = strings.TrimSpace(p.Workflow.BackgroundTheme)
	p.Workflow.BackgroundStyle = strings.TrimSpace(p.Workflow.BackgroundStyle)
	p.Workflow.EnhanceLevel = strings.TrimSpace(p.Workflow.EnhanceLevel)
	p.Workflow.RetouchStrength = strings.TrimSpace(p.Workflow.RetouchStrength)
	p.Workflow.Notes = strings.TrimSpace(p.Workflow.Notes)

	p.SourceAsset.AssetID = strings.TrimSpace(p.SourceAsset.AssetID)
	p.SourceAsset.StorageKey = strings.TrimSpace(p.SourceAsset.StorageKey)
	p.SourceAsset.URL = strings.TrimSpace(p.SourceAsset.URL)
	p.SourceAsset.Mime = strings.TrimSpace(p.SourceAsset.Mime)
	p.SourceAsset.Filename = strings.TrimSpace(p.SourceAsset.Filename)
}

// Validate ensures the prompt JSON satisfies the required contract before persistence or enhancement.
func (p PromptJSON) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(p.ProductType) == "" {
		return fmt.Errorf("product_type is required")
	}
	if strings.TrimSpace(p.Style) == "" {
		return fmt.Errorf("style is required")
	}
	if strings.TrimSpace(p.Background) == "" {
		return fmt.Errorf("background is required")
	}
	if p.Quantity < 1 || p.Quantity > MaxPromptQuantity {
		return fmt.Errorf("quantity must be between 1 and %d", MaxPromptQuantity)
	}
	if _, ok := allowedAspectRatios[p.AspectRatio]; !ok {
		return fmt.Errorf("aspect_ratio must be one of 1:1, 4:3, 3:4, 16:9, 9:16")
	}
	if p.Watermark.Enabled {
		if strings.TrimSpace(p.Watermark.Text) == "" {
			return fmt.Errorf("watermark.text is required when watermark.enabled is true")
		}
		if strings.TrimSpace(p.Watermark.Position) == "" {
			return fmt.Errorf("watermark.position is required when watermark.enabled is true")
		}
	}
	mode := normalizeWorkflowMode(p.Workflow.Mode)
	if _, ok := allowedWorkflowModes[mode]; !ok {
		return fmt.Errorf("workflow.mode must be one of generate, background, enhance, retouch")
	}
	if mode != WorkflowModeGenerate && p.SourceAsset.IsZero() {
		return fmt.Errorf("source_asset is required when workflow.mode is %s", mode)
	}
	return nil
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

// IsZero reports whether the prompt references any existing asset.
func (s SourceAssetConfig) IsZero() bool {
	return strings.TrimSpace(s.AssetID) == "" && strings.TrimSpace(s.StorageKey) == "" && strings.TrimSpace(s.URL) == ""
}

func normalizeWorkflowMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = DefaultWorkflowMode
	}
	if _, ok := allowedWorkflowModes[mode]; !ok {
		return DefaultWorkflowMode
	}
	return mode
}
