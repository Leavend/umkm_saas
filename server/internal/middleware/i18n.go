package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type localeContextKey struct{}
type countryContextKey struct{}

var (
	LocaleKey  = localeContextKey{}
	CountryKey = countryContextKey{}
)

// CountryLookup resolves ISO country codes for an IP address.
type CountryLookup func(ip string) (string, error)

func I18N(defaultLocale string, lookup CountryLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			country := ResolveCountry(r, lookup)
			locale := detectLocale(r, defaultLocale, country)
			ctx := context.WithValue(r.Context(), LocaleKey, locale)
			if country != "" {
				ctx = context.WithValue(ctx, CountryKey, strings.ToUpper(country))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func detectLocale(r *http.Request, fallback string, country string) string {
	if v := r.Header.Get("X-Locale"); v != "" {
		return normalizeLocale(v)
	}
	if v := parseAcceptLanguage(r.Header.Get("Accept-Language")); v != "" {
		return v
	}
	if strings.EqualFold(country, "ID") {
		return "id"
	}
	if country != "" {
		return "en"
	}
	if fallback != "" {
		return fallback
	}
	return "en"
}

func parseAcceptLanguage(header string) string {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		locale := strings.TrimSpace(strings.Split(part, ";")[0])
		if locale == "" {
			continue
		}
		return normalizeLocale(locale)
	}
	return ""
}

func normalizeLocale(locale string) string {
	locale = strings.ToLower(locale)
	if strings.HasPrefix(locale, "id") {
		return "id"
	}
	return "en"
}

// ClientIP returns the best-effort client IP address for the request.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func LocaleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(LocaleKey).(string); ok {
		return v
	}
	return "en"
}

// CountryFromContext returns the ISO country code stored in the request context.
func CountryFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CountryKey).(string); ok {
		return v
	}
	return ""
}

// ResolveCountry resolves a best-effort ISO country code for the given request.
func ResolveCountry(r *http.Request, lookup CountryLookup) string {
	if r == nil {
		return ""
	}
	headerHints := []string{"X-Country-Code", "X-IP-Country", "CF-IPCountry", "X-Appengine-Country"}
	for _, key := range headerHints {
		if val := strings.TrimSpace(r.Header.Get(key)); val != "" {
			return strings.ToUpper(val)
		}
	}
	if region := localeRegion(r.Header.Get("X-Locale")); region != "" {
		return region
	}
	if region := localeRegion(r.Header.Get("Accept-Language")); region != "" {
		return region
	}
	if locale := normalizeLocale(r.Header.Get("X-Locale")); locale == "id" {
		return "ID"
	}
	if locale := parseAcceptLanguage(r.Header.Get("Accept-Language")); locale == "id" {
		return "ID"
	}
	if lookup != nil {
		if ip := ClientIP(r); ip != "" {
			if country, err := lookup(ip); err == nil && country != "" {
				return strings.ToUpper(country)
			}
		}
	}
	return ""
}

func localeRegion(accept string) string {
	for _, part := range strings.Split(accept, ",") {
		token := strings.TrimSpace(strings.Split(part, ";")[0])
		if token == "" {
			continue
		}
		if idx := strings.IndexAny(token, "-_"); idx > 0 && idx < len(token)-1 {
			return strings.ToUpper(token[idx+1:])
		}
	}
	return ""
}
