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
}
