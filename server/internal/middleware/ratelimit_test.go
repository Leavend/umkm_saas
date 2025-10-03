package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPForRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		remoteAddr string
		want       string
	}{
		{
			name:       "single ip",
			header:     "203.0.113.1",
			remoteAddr: "198.51.100.10:1234",
			want:       "203.0.113.1",
		},
		{
			name:       "multiple ips use first",
			header:     " 203.0.113.1 , 198.51.100.2 ",
			remoteAddr: "198.51.100.10:1234",
			want:       "203.0.113.1",
		},
		{
			name:       "invalid forwarded falls back",
			header:     "invalid",
			remoteAddr: "198.51.100.10:1234",
			want:       "198.51.100.10",
		},
		{
			name:       "empty forwarded uses remote host",
			header:     "",
			remoteAddr: "198.51.100.10:1234",
			want:       "198.51.100.10",
		},
		{
			name:       "ipv6 forwarded",
			header:     "2001:db8::1",
			remoteAddr: net.JoinHostPort("2001:db8::2", "443"),
			want:       "2001:db8::1",
		},
		{
			name:       "ipv6 remote fallback",
			header:     "invalid",
			remoteAddr: net.JoinHostPort("2001:db8::2", "443"),
			want:       "2001:db8::2",
		},
		{
			name:       "remote without port",
			header:     "invalid",
			remoteAddr: "203.0.113.1",
			want:       "203.0.113.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.header != "" {
				req.Header.Set("X-Forwarded-For", tc.header)
			}
			if got := clientIPForRateLimit(req); got != tc.want {
				t.Fatalf("clientIPForRateLimit() = %q, want %q", got, tc.want)
			}
		})
	}
}
