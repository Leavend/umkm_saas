package jsoncfg

import "testing"

func TestPromptJSONNormalizeDefaults(t *testing.T) {
	p := &PromptJSON{}
	p.Normalize("")

	if p.Version != DefaultPromptVersion {
		t.Fatalf("Version = %q, want %q", p.Version, DefaultPromptVersion)
	}
	if p.AspectRatio != DefaultPromptAspectRatio {
		t.Fatalf("AspectRatio = %q, want %q", p.AspectRatio, DefaultPromptAspectRatio)
	}
	if p.Quantity != DefaultPromptQuantity {
		t.Fatalf("Quantity = %d, want %d", p.Quantity, DefaultPromptQuantity)
	}
	if p.Extras.Locale != DefaultExtrasLocale {
		t.Fatalf("Extras.Locale = %q, want %q", p.Extras.Locale, DefaultExtrasLocale)
	}
	if p.Extras.Quality != DefaultExtrasQuality {
		t.Fatalf("Extras.Quality = %q, want %q", p.Extras.Quality, DefaultExtrasQuality)
	}
}

func TestPromptJSONNormalizePreferredLocaleAndClamp(t *testing.T) {
	p := &PromptJSON{
		Quantity:    10,
		AspectRatio: "16:9",
		Extras: ExtrasConfig{
			Locale: "",
		},
	}
	p.Normalize("id")

	if p.Quantity != MaxPromptQuantity {
		t.Fatalf("Quantity clamp = %d, want %d", p.Quantity, MaxPromptQuantity)
	}
	if p.AspectRatio != "16:9" {
		t.Fatalf("AspectRatio should keep explicit value, got %q", p.AspectRatio)
	}
	if p.Extras.Locale != "id" {
		t.Fatalf("Extras.Locale = %q, want %q", p.Extras.Locale, "id")
	}
}

func TestPromptJSONNormalizeMinimumQuantity(t *testing.T) {
	p := &PromptJSON{Quantity: -5}
	p.Normalize("")
	if p.Quantity != DefaultPromptQuantity {
		t.Fatalf("Quantity = %d, want %d", p.Quantity, DefaultPromptQuantity)
	}
}

func TestPromptJSONValidate(t *testing.T) {
	prompt := PromptJSON{
		Version:      DefaultPromptVersion,
		Title:        "Nasi Goreng Premium",
		ProductType:  "food",
		Style:        "elegan",
		Background:   "marble",
		Instructions: "Gunakan lighting lembut",
		Watermark: WatermarkConfig{
			Enabled:  true,
			Text:     "Brand",
			Position: "bottom-right",
		},
		AspectRatio: "1:1",
		Quantity:    1,
	}
	if err := prompt.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	prompt.AspectRatio = "2:1"
	if err := prompt.Validate(); err == nil {
		t.Fatalf("Validate() expected error for invalid aspect ratio")
	}

	prompt.AspectRatio = "1:1"
	prompt.Watermark.Enabled = true
	prompt.Watermark.Text = ""
	if err := prompt.Validate(); err == nil {
		t.Fatalf("Validate() expected error when watermark text missing")
	}
}
