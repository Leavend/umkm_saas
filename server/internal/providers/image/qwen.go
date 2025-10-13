package image

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"server/internal/providers/qwen"
)

// QwenGenerator orchestrates calls to DashScope's Qwen image model and falls back
// to another generator (e.g. synthetic Gemini) when credentials are missing or
// the remote call fails.
type qwenImageClient interface {
	GenerateImage(context.Context, qwen.ImageRequest) (*qwen.ImageAsset, error)
	HasCredentials() bool
	Model() string
}

// QwenGenerator orchestrates calls to DashScope's Qwen image model and falls back
// to another generator (e.g. synthetic Gemini) when credentials are missing or
// the remote call fails.
type QwenGenerator struct {
	client   qwenImageClient
	fallback Generator
}

// NewQwenGenerator wires a Qwen client with an optional fallback generator.
func NewQwenGenerator(client qwenImageClient, fallback Generator) *QwenGenerator {
	return &QwenGenerator{client: client, fallback: fallback}
}

// Generate fulfils the Generator interface.
func (g *QwenGenerator) Generate(ctx context.Context, req GenerateRequest) ([]Asset, error) {
	if g == nil {
		return nil, fmt.Errorf("qwen generator not configured")
	}
	if g.client == nil {
		if g.fallback != nil {
			return g.fallback.Generate(ctx, req)
		}
		return nil, fmt.Errorf("qwen generator not configured")
	}
	if !g.client.HasCredentials() {
		if g.fallback != nil {
			return g.fallback.Generate(ctx, req)
		}
		return nil, fmt.Errorf("qwen generator missing credentials")
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	size := AspectRatioSize(req.AspectRatio)
	workflowMode := NormalizeWorkflowMode(string(req.Workflow.Mode))
	baseWorkflow := qwen.Workflow{
		Mode:            string(workflowMode),
		BackgroundTheme: strings.TrimSpace(req.Workflow.BackgroundTheme),
		BackgroundStyle: strings.TrimSpace(req.Workflow.BackgroundStyle),
		EnhanceLevel:    strings.TrimSpace(req.Workflow.EnhanceLevel),
		RetouchStrength: strings.TrimSpace(req.Workflow.RetouchStrength),
		Notes:           strings.TrimSpace(req.Workflow.Notes),
	}
	source := qwenSourceFromRequest(req.SourceImage)
	assets := make([]Asset, 0, quantity)
	for i := 0; i < quantity; i++ {
		prompt := buildVariationPrompt(strings.TrimSpace(req.Prompt), quantity, i)
		seed := deterministicSeed(req.RequestID, req.Provider, req.Locale, prompt, i)
		workflow := derivedWorkflow(baseWorkflow, source)
		imageReq := qwen.ImageRequest{
			Prompt:         prompt,
			NegativePrompt: strings.TrimSpace(req.NegativePrompt),
			Size:           size,
			Seed:           seed,
			RequestID:      req.RequestID,
			Quality:        strings.TrimSpace(req.Quality),
			Locale:         strings.TrimSpace(req.Locale),
			Workflow:       workflow,
			SourceImage:    source,
		}

		asset, err := g.invokeQwen(ctx, imageReq)
		if err != nil {
			if shouldFallbackToSynthetic(err) && g.fallback != nil {
				return g.fallback.Generate(ctx, req)
			}
			return nil, err
		}
		assets = append(assets, Asset{
			StorageKey: "",
			URL:        asset.URL,
			Format:     normalizeFormat(asset.Format),
			Width:      asset.Width,
			Height:     asset.Height,
			Data:       asset.Data,
		})
	}
	return assets, nil
}

func (g *QwenGenerator) String() string {
	if g == nil || g.client == nil {
		return "qwen"
	}
	return g.client.Model()
}

var _ Generator = (*QwenGenerator)(nil)

func (g *QwenGenerator) invokeQwen(ctx context.Context, req qwen.ImageRequest) (*qwen.ImageAsset, error) {
	asset, err := g.client.GenerateImage(ctx, req)
	if err == nil {
		return asset, nil
	}
	if !isTransientQwenError(err) {
		return nil, err
	}

	simplified := simplifyQwenRequest(req)
	asset, retryErr := g.client.GenerateImage(ctx, simplified)
	if retryErr != nil {
		return nil, retryErr
	}
	return asset, nil
}

func shouldFallbackToSynthetic(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, qwen.ErrMissingAPIKey) {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") {
		return true
	}
	if isTransientQwenError(err) {
		return true
	}
	return false
}

func qwenSourceFromRequest(src *SourceImage) *qwen.SourceImage {
	if src == nil {
		return nil
	}
	payload := &qwen.SourceImage{
		AssetID:    strings.TrimSpace(src.AssetID),
		StorageKey: strings.TrimSpace(src.StorageKey),
		URL:        strings.TrimSpace(src.URL),
		MIME:       strings.TrimSpace(src.MIME),
		Filename:   strings.TrimSpace(src.Filename),
	}
	if len(src.Data) > 0 {
		payload.Data = append([]byte(nil), src.Data...)
	}
	if src.Width > 0 {
		payload.Width = src.Width
	}
	if src.Height > 0 {
		payload.Height = src.Height
	}
	return payload
}

func deterministicSeed(values ...any) int {
	if len(values) == 0 {
		return 0
	}
	var parts []string
	for _, v := range values {
		parts = append(parts, fmt.Sprint(v))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	n := binary.BigEndian.Uint32(sum[:4])
	value := int(n % 2147483647)
	if value <= 0 {
		fallback := binary.BigEndian.Uint32(sum[4:8]) % 2147483647
		if fallback == 0 {
			fallback = 1
		}
		value = int(fallback)
	}
	return value
}

func normalizeFormat(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch mime {
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	default:
		if strings.HasPrefix(mime, "image/") {
			return mime
		}
		return "image/png"
	}
}

func buildVariationPrompt(prompt string, total, index int) string {
	trimmed := strings.TrimSpace(prompt)
	if total <= 1 {
		return trimmed
	}
	if trimmed == "" {
		return fmt.Sprintf("Variation #%d for the same campaign.", index+1)
	}
	return fmt.Sprintf("%s\nVariation #%d for the same campaign.", trimmed, index+1)
}

func derivedWorkflow(base qwen.Workflow, source *qwen.SourceImage) qwen.Workflow {
	workflow := base
	if source != nil {
		if workflow.Mode == "" || workflow.Mode == string(WorkflowModeGenerate) {
			workflow.Mode = string(WorkflowModeEnhance)
		}
	}
	return workflow
}

func simplifyQwenRequest(req qwen.ImageRequest) qwen.ImageRequest {
	simplified := req
	simplified.NegativePrompt = ""
	if simplified.Workflow.Mode == "" || simplified.Workflow.Mode == string(WorkflowModeGenerate) {
		simplified.Workflow = qwen.Workflow{}
	} else {
		simplified.Workflow.Notes = strings.TrimSpace(simplified.Workflow.Notes)
	}
	return simplified
}

func isTransientQwenError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "internalerror") || strings.Contains(msg, "internal error") {
		return true
	}
	if strings.Contains(msg, "service unavailable") || strings.Contains(msg, "server unavailable") {
		return true
	}
	if strings.Contains(msg, "timeout") {
		return true
	}
	return false
}
