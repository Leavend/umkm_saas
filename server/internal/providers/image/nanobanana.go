package image

import (
	"context"
	"fmt"
	"time"
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

type NanoBanana struct{}

func NewNanoBanana() *NanoBanana {
	return &NanoBanana{}
}

func (n *NanoBanana) Generate(ctx context.Context, req GenerateRequest) ([]Asset, error) {
	assets := make([]Asset, req.Quantity)
	for i := range assets {
		assets[i] = Asset{
			URL:    fmt.Sprintf("https://cdn.example.com/nanobanana/%s/%d.png", req.RequestID, i+1),
			Format: "image/png",
			Width:  1024,
			Height: 1024,
		}
	}
	select {
	case <-time.After(1500 * time.Millisecond):
		return assets, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var _ Generator = (*NanoBanana)(nil)
