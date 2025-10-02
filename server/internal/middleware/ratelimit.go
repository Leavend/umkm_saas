package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	count int
	until time.Time
}

func RateLimit(limit int, per time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := make(map[string]*bucket)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIPForRateLimit(r)
			mu.Lock()
			b, ok := buckets[ip]
			now := time.Now()
			if !ok || now.After(b.until) {
				b = &bucket{count: 0, until: now.Add(per)}
				buckets[ip] = b
			}
			if b.count >= limit {
				mu.Unlock()
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			b.count++
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

func clientIPForRateLimit(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return xf
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
