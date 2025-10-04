package prompt

import (
	"context"
	"fmt"

	"server/internal/domain/jsoncfg"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type EnhanceRequest struct {
	Prompt jsoncfg.PromptJSON
	Locale string
}

type EnhanceIdea struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

type EnhanceResponse struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Ideas       []EnhanceIdea     `json:"ideas,omitempty"`
	Metadata    map[string]string `json:"metadata"`
	Provider    string            `json:"-"`
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
	c := cases.Title(language.Und)
	title := req.Prompt.Title
	if title == "" {
		title = "Produk UMKM"
	}
	product := req.Prompt.ProductType
	if product == "" {
		product = "produk"
	}
	desc := fmt.Sprintf("%s %s dengan gaya premium", c.String(product), title)
	res := &EnhanceResponse{
		Title:       fmt.Sprintf("%s Signature", title),
		Description: desc,
		Keywords:    []string{"kuliner", "fotografi", "umkm"},
		Metadata: map[string]string{
			"locale": req.Locale,
		},
		Provider: staticProviderName,
	}
	res.Ideas = []EnhanceIdea{{
		Title:       res.Title,
		Description: res.Description,
		Keywords:    res.Keywords,
	}}
	return res, nil
}

func (s *StaticEnhancer) Random(ctx context.Context, locale string) ([]EnhanceResponse, error) {
	items := []EnhanceResponse{
		{Title: "Nasi Uduk Rempah", Description: "Hidangan sarapan khas Betawi", Keywords: []string{"nasi", "rempah"}, Metadata: map[string]string{"locale": locale}, Provider: staticProviderName},
		{Title: "Es Kopi Gula Aren", Description: "Minuman kekinian untuk UMKM", Keywords: []string{"kopi", "gula aren"}, Metadata: map[string]string{"locale": locale}, Provider: staticProviderName},
		{Title: "Kue Lapis Legit", Description: "Dessert klasik Nusantara", Keywords: []string{"dessert", "nusantara"}, Metadata: map[string]string{"locale": locale}, Provider: staticProviderName},
	}
	return items, nil
}

var _ Enhancer = (*StaticEnhancer)(nil)
