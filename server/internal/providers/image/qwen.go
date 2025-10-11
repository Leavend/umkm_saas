package image

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"server/internal/providers/qwen"
)

// QwenGenerator orchestrates calls to DashScope's Qwen image model and falls back
// to another generator (e.g. synthetic Gemini) when credentials are missing or
// the remote call fails.
type QwenGenerator struct {
	client   *qwen.Client
	fallback Generator
}

// NewQwenGenerator wires a Qwen client with an optional fallback generator.
func NewQwenGenerator(client *qwen.Client, fallback Generator) *QwenGenerator {
	return &QwenGenerator{client: client, fallback: fallback}
}

// Generate fulfils the Generator interface.
func (g *QwenGenerator) Generate(ctx context.Context, req GenerateRequest) ([]Asset, error) {
	if g == nil || g.client == nil {
		if g != nil && g.fallback != nil {
			return g.fallback.Generate(ctx, req)
		}
		return nil, fmt.Errorf("qwen generator not configured")
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	size := AspectRatioSize(req.AspectRatio)
	assets := make([]Asset, 0, quantity)
	for i := 0; i < quantity; i++ {
		prompt := req.Prompt
		if quantity > 1 {
			prompt = fmt.Sprintf("%s\nVariation #%d for the same campaign.", strings.TrimSpace(req.Prompt), i+1)
		}
		seed := deterministicSeed(req.RequestID, req.Provider, req.Locale, prompt, i)
		asset, err := g.client.GenerateImage(ctx, qwen.ImageRequest{
			Prompt:         prompt,
			NegativePrompt: req.NegativePrompt,
			Size:           size,
			Seed:           seed,
			RequestID:      req.RequestID,
		})
		if err != nil {
			if g.fallback != nil {
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
