package video

import (
	"context"

	"server/internal/providers/genai"
)

type GenerateRequest struct {
	Prompt    string
	Provider  string
	RequestID string
	Locale    string
}

type Asset struct {
	StorageKey string
	URL        string
	Format     string
	Length     int
	Data       []byte
}

type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) (*Asset, error)
}

type GeminiGenerator struct {
	client *genai.Client
}

func NewGeminiGenerator(client *genai.Client) *GeminiGenerator {
	return &GeminiGenerator{client: client}
}

func (g *GeminiGenerator) Generate(ctx context.Context, req GenerateRequest) (*Asset, error) {
	asset, err := g.client.GenerateVideo(ctx, genai.VideoRequest{
		Prompt:    req.Prompt,
		Locale:    req.Locale,
		RequestID: req.RequestID,
	})
	if err != nil {
		return nil, err
	}
	return &Asset{
		StorageKey: asset.StorageKey,
		URL:        asset.URL,
		Format:     asset.Format,
		Length:     asset.Length,
		Data:       asset.Data,
	}, nil
}

var _ Generator = (*GeminiGenerator)(nil)
