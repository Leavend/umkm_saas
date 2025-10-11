package image

import (
	"context"
	"strings"
)

// WorkflowMode enumerates supported editing modes for image generation.
type WorkflowMode string

const (
	WorkflowModeGenerate   WorkflowMode = "generate"
	WorkflowModeBackground WorkflowMode = "background"
	WorkflowModeEnhance    WorkflowMode = "enhance"
	WorkflowModeRetouch    WorkflowMode = "retouch"
)

// Workflow conveys how the provider should manipulate the image.
type Workflow struct {
	Mode            WorkflowMode
	BackgroundTheme string
	BackgroundStyle string
	EnhanceLevel    string
	RetouchStrength string
	Notes           string
}

// SourceImage describes an uploaded asset that can be used as conditioning input.
type SourceImage struct {
	AssetID    string
	StorageKey string
	URL        string
	MIME       string
	Data       []byte
	Width      int
	Height     int
	Filename   string
}

// GenerateRequest describes a normalized request passed to any image provider.
type GenerateRequest struct {
	Prompt         string
	Quantity       int
	AspectRatio    string
	Provider       string
	RequestID      string
	Locale         string
	WatermarkTag   string
	Quality        string
	NegativePrompt string
	Workflow       Workflow
	SourceImage    *SourceImage
}

// Asset represents a generated or edited image.
type Asset struct {
	StorageKey string
	URL        string
	Format     string
	Width      int
	Height     int
	Data       []byte
}

// Generator is the contract implemented by all image providers.
type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) ([]Asset, error)
}

// NormalizeWorkflowMode sanitizes free-form user input into a supported mode.
func NormalizeWorkflowMode(mode string) WorkflowMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(WorkflowModeBackground):
		return WorkflowModeBackground
	case string(WorkflowModeEnhance):
		return WorkflowModeEnhance
	case string(WorkflowModeRetouch):
		return WorkflowModeRetouch
	default:
		return WorkflowModeGenerate
	}
}
