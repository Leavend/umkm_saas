package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type assertError string

func (e assertError) Error() string { return string(e) }

func TestDetectLocale(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(r *http.Request)
		fallback string
		country  string
		want     string
	}{
		{
			name: "x-locale overrides",
			setup: func(r *http.Request) {
				r.Header.Set("X-Locale", "ID")
			},
			country: "US",
			want:    "id",
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
			name:    "country id overrides",
			country: "ID",
			want:    "id",
		},
		{
			name:    "country non-id falls back to en",
			country: "US",
			want:    "en",
		},
		{
			name:     "configured fallback",
			fallback: "id",
			want:     "id",
		},
		{
			name: "default to en",
			want: "en",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.setup != nil {
				tc.setup(req)
			}
			got := detectLocale(req, tc.fallback, tc.country)
			if got != tc.want {
				t.Fatalf("detectLocale() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveCountry(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(r *http.Request)
		resolver CountryLookup
		want     string
	}{
		{
			name: "header precedence",
			setup: func(r *http.Request) {
				r.Header.Set("X-Country-Code", "us")
				r.Header.Set("CF-IPCountry", "id")
			},
			want: "US",
		},
		{
			name: "locale region fallback",
			setup: func(r *http.Request) {
				r.Header.Set("X-Locale", "en-AU")
			},
			want: "AU",
		},
		{
			name: "accept-language region",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "en-GB,en;q=0.9")
			},
			want: "GB",
		},
		{
			name: "id locale normalization",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "id;q=0.8")
			},
			want: "ID",
		},
		{
			name: "resolver fallback",
			resolver: func(ip string) (string, error) {
				if ip != "203.0.113.4" {
					t.Fatalf("unexpected ip: %s", ip)
				}
				return "my", nil
			},
			want: "MY",
		},
		{
			name: "resolver error returns empty",
			resolver: func(ip string) (string, error) {
				return "", assertError("boom")
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "203.0.113.4:80"
			if tc.setup != nil {
				tc.setup(req)
			}
			got := ResolveCountry(req, tc.resolver)
			if got != tc.want {
				t.Fatalf("ResolveCountry() = %q, want %q", got, tc.want)
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
