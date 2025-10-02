package prompt

import (
	"context"
	"fmt"
	"strings"
)

type EnhanceRequest struct {
	Title       string
	ProductType string
	Locale      string
}

type EnhanceResponse struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Metadata    map[string]string `json:"metadata"`
}

type Enhancer interface {
	Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error)
	Random(ctx context.Context, locale string) ([]EnhanceResponse, error)
}

type StaticEnhancer struct{}

func NewStaticEnhancer() *StaticEnhancer {
	return &StaticEnhancer{}
}

func (s *StaticEnhancer) Enhance(ctx context.Context, req EnhanceRequest) (*EnhanceResponse, error) {
	desc := fmt.Sprintf("%s %s dengan gaya premium", strings.Title(req.ProductType), req.Title)
	res := &EnhanceResponse{
		Title:       fmt.Sprintf("%s Signature", req.Title),
		Description: desc,
		Keywords:    []string{"kuliner", "fotografi", "umkm"},
		Metadata: map[string]string{
			"locale": req.Locale,
		},
	}
	return res, nil
}

func (s *StaticEnhancer) Random(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	items := []EnhanceResponse{
		{Title: "Nasi Uduk Rempah", Description: "Hidangan sarapan khas Betawi", Keywords: []string{"nasi", "rempah"}, Metadata: map[string]string{"locale": locale}},
		{Title: "Es Kopi Gula Aren", Description: "Minuman kekinian untuk UMKM", Keywords: []string{"kopi", "gula aren"}, Metadata: map[string]string{"locale": locale}},
		{Title: "Kue Lapis Legit", Description: "Dessert klasik Nusantara", Keywords: []string{"dessert", "nusantara"}, Metadata: map[string]string{"locale": locale}},
	}
	return items, nil
}

var _ Enhancer = (*StaticEnhancer)(nil)
