package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type localeKey string

const LocaleKey localeKey = "locale"

// CountryLookup resolves ISO country codes for an IP address.
type CountryLookup func(ip string) (string, error)

func I18N(defaultLocale string, lookup CountryLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := detectLocale(r, defaultLocale, lookup)
			ctx := context.WithValue(r.Context(), LocaleKey, locale)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func detectLocale(r *http.Request, fallback string, lookup CountryLookup) string {
	if v := r.Header.Get("X-Locale"); v != "" {
		return normalizeLocale(v)
	}
	if v := parseAcceptLanguage(r.Header.Get("Accept-Language")); v != "" {
		return v
	}
	if lookup != nil {
		if ip := clientIP(r); ip != "" {
			if country, err := lookup(ip); err == nil {
				if strings.EqualFold(country, "ID") {
					return "id"
				}
				if country != "" {
					return "en"
				}
			}
		}
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

func clientIP(r *http.Request) string {
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
