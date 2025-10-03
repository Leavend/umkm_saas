package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectLocale(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(r *http.Request)
		fallback string
		want     string
	}{
		{
			name: "x-locale overrides",
			setup: func(r *http.Request) {
				r.Header.Set("X-Locale", "ID")
			},
			want: "id",
		},
		{
			name: "accept-language used",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "en-US,en;q=0.9")
			},
			want: "en",
		},
		{
			name: "accept-language id preference",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "id-ID,en;q=0.8")
			},
			want: "id",
		},
		{
			name: "geoip fallback",
			setup: func(r *http.Request) {
				r.RemoteAddr = "103.21.77.15:1234"
			},
			want: "id",
		},
		{
			name:     "configured fallback",
			setup:    func(r *http.Request) {},
			fallback: "id",
			want:     "id",
		},
		{
			name:  "default to en",
			setup: func(r *http.Request) {},
			want:  "en",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.1:80"
			if tc.setup != nil {
				tc.setup(req)
			}
			got := detectLocale(req, tc.fallback)
			if got != tc.want {
				t.Fatalf("detectLocale() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocaleFromContext(t *testing.T) {
	ctx := context.Background()
	if got := LocaleFromContext(ctx); got != "en" {
		t.Fatalf("LocaleFromContext() default = %q, want %q", got, "en")
	}
	ctx = context.WithValue(ctx, LocaleKey, "id")
	if got := LocaleFromContext(ctx); got != "id" {
		t.Fatalf("LocaleFromContext() with value = %q, want %q", got, "id")
	}
}
