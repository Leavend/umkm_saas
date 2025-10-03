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
