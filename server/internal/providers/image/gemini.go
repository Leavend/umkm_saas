package image

import (
	"context"

	"server/internal/providers/genai"
)

type GenerateRequest struct {
	Prompt       string
	Quantity     int
	AspectRatio  string
	Provider     string
	RequestID    string
	Locale       string
	WatermarkTag string
}

type Asset struct {
	URL    string
	Format string
	Width  int
	Height int
}

type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) ([]Asset, error)
}

type GeminiGenerator struct {
	client *genai.Client
}

func NewGeminiGenerator(client *genai.Client) *GeminiGenerator {
	return &GeminiGenerator{client: client}
}

func (g *GeminiGenerator) Generate(ctx context.Context, req GenerateRequest) ([]Asset, error) {
	assets, err := g.client.GenerateImages(ctx, genai.ImageRequest{
		Prompt:       req.Prompt,
		Quantity:     req.Quantity,
		AspectRatio:  req.AspectRatio,
		Locale:       req.Locale,
		WatermarkTag: req.WatermarkTag,
		RequestID:    req.RequestID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Asset, len(assets))
	for i, asset := range assets {
		out[i] = Asset{
			URL:    asset.URL,
			Format: asset.Format,
			Width:  asset.Width,
			Height: asset.Height,
		}
	}
	return out, nil
}

var _ Generator = (*GeminiGenerator)(nil)
