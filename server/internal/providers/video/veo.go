package video

import (
	"context"
	"fmt"
	"time"
)

type GenerateRequest struct {
	Prompt    string
	Provider  string
	RequestID string
	Locale    string
}

type Asset struct {
	URL    string
	Format string
	Length int
}

type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) (*Asset, error)
}

type VEO struct{}

func NewVEO() *VEO {
	return &VEO{}
}

func (v *VEO) Generate(ctx context.Context, req GenerateRequest) (*Asset, error) {
	select {
	case <-time.After(3 * time.Second):
		return &Asset{
			URL:    fmt.Sprintf("https://cdn.example.com/%s/%s.mp4", req.Provider, req.RequestID),
			Format: "video/mp4",
			Length: 16,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var _ Generator = (*VEO)(nil)
