package google

import "testing"

func TestAudienceMatches(t *testing.T) {
	cases := []struct {
		name     string
		aud      any
		clientID string
		want     bool
	}{
		{name: "string match", aud: "client", clientID: "client", want: true},
		{name: "string mismatch", aud: "client", clientID: "other", want: false},
		{name: "slice any match", aud: []any{"other", "client"}, clientID: "client", want: true},
		{name: "slice any mismatch", aud: []any{"other", 1}, clientID: "client", want: false},
		{name: "slice string match", aud: []string{"client", "alt"}, clientID: "client", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := audienceMatches(tc.aud, tc.clientID); got != tc.want {
				t.Fatalf("audienceMatches(%v, %q) = %v, want %v", tc.aud, tc.clientID, got, tc.want)
			}
		})
	}
}
