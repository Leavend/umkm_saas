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
		resolver CountryLookup
		want     string
	}{
		{
			name: "x-locale overrides",
			setup: func(r *http.Request) {
				r.Header.Set("X-Locale", "ID")
			},
			resolver: func(ip string) (string, error) {
				t.Fatalf("resolver should not be called")
				return "", nil
			},
			want: "id",
		},
		{
			name: "accept-language used",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "en-US,en;q=0.9")
			},
			resolver: func(ip string) (string, error) {
				t.Fatalf("resolver should not be called")
				return "", nil
			},
			want: "en",
		},
		{
			name: "accept-language id preference",
			setup: func(r *http.Request) {
				r.Header.Set("Accept-Language", "id-ID,en;q=0.8")
			},
			resolver: func(ip string) (string, error) {
				t.Fatalf("resolver should not be called")
				return "", nil
			},
			want: "id",
		},
		{
			name: "geoip returns id",
			resolver: func(ip string) (string, error) {
				if ip != "203.0.113.4" {
					t.Fatalf("unexpected ip: %s", ip)
				}
				return "ID", nil
			},
			want: "id",
		},
		{
			name: "geoip returns non-id",
			resolver: func(ip string) (string, error) {
				return "US", nil
			},
			want: "en",
		},
		{
			name: "geoip error falls back",
			resolver: func(ip string) (string, error) {
				return "", assertError("boom")
			},
			fallback: "id",
			want:     "id",
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
			req.RemoteAddr = "203.0.113.4:80"
			if tc.setup != nil {
				tc.setup(req)
			}
			got := detectLocale(req, tc.fallback, tc.resolver)
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
