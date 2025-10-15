package infra

import "testing"

func TestLoadConfigDefaultStorageBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "")
	t.Setenv("STORAGE_BASE_URL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	expected := "http://localhost:8080/static"
	if cfg.StorageBaseURL != expected {
		t.Fatalf("StorageBaseURL mismatch: got %q want %q", cfg.StorageBaseURL, expected)
	}
	if len(cfg.ImageSourceAllowlist) != 1 || cfg.ImageSourceAllowlist[0] != "localhost" {
		t.Fatalf("ImageSourceAllowlist mismatch: %#v", cfg.ImageSourceAllowlist)
	}
}

func TestLoadConfigInheritsPortInStorageBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "1919")
	t.Setenv("STORAGE_BASE_URL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	expected := "http://localhost:1919/static"
	if cfg.StorageBaseURL != expected {
		t.Fatalf("StorageBaseURL mismatch: got %q want %q", cfg.StorageBaseURL, expected)
	}
	if len(cfg.ImageSourceAllowlist) != 1 || cfg.ImageSourceAllowlist[0] != "localhost" {
		t.Fatalf("ImageSourceAllowlist mismatch: %#v", cfg.ImageSourceAllowlist)
	}
}

func TestLoadConfigHonorsExplicitStorageBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "1919")
	t.Setenv("STORAGE_BASE_URL", "https://cdn.example.com/static")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	expected := "https://cdn.example.com/static"
	if cfg.StorageBaseURL != expected {
		t.Fatalf("StorageBaseURL mismatch: got %q want %q", cfg.StorageBaseURL, expected)
	}
	if len(cfg.ImageSourceAllowlist) != 1 || cfg.ImageSourceAllowlist[0] != "cdn.example.com" {
		t.Fatalf("ImageSourceAllowlist mismatch: %#v", cfg.ImageSourceAllowlist)
	}
}

func TestLoadConfigMergesExplicitAllowlist(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "1919")
	t.Setenv("STORAGE_BASE_URL", "https://cdn.example.com/static")
	t.Setenv("IMAGE_SOURCE_HOST_ALLOWLIST", "media.example.com, localhost ")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	expected := []string{"cdn.example.com", "localhost", "media.example.com"}
	if len(cfg.ImageSourceAllowlist) != len(expected) {
		t.Fatalf("ImageSourceAllowlist mismatch: got %#v want %#v", cfg.ImageSourceAllowlist, expected)
	}
	for i, host := range expected {
		if cfg.ImageSourceAllowlist[i] != host {
			t.Fatalf("ImageSourceAllowlist[%d] = %q, want %q", i, cfg.ImageSourceAllowlist[i], host)
		}
	}
}
