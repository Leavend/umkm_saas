package imagegen

import (
	"strings"
	"testing"
)

func TestBuildInstruction(t *testing.T) {
	var req GenerateRequest
	req.AspectRatio = "1:1"
	req.Prompt.Title = "Nasi goreng seafood premium"
	req.Prompt.ProductType = "food"
	req.Prompt.Style = "elegan"
	req.Prompt.Background = "marble"
	req.Prompt.Instructions = "Lighting lembut"
	req.Prompt.References = []struct {
		URL string `json:"url"`
	}{{URL: "https://example.com/ref1.png"}}

	got := BuildInstruction(req)

	checks := []string{
		"Nasi goreng seafood premium",
		"jenis: food",
		"Gaya visual: elegan",
		"Ganti/atur latar: marble",
		"Instruksi tambahan: Lighting lembut",
		"Pertahankan bentuk produk asli",
		"rasio 1:1",
	}
	for _, expect := range checks {
		if !strings.Contains(got, expect) {
			t.Fatalf("instruction missing %q: %s", expect, got)
		}
	}
}
