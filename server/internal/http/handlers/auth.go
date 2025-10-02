package handlers

import (
	"encoding/json"
	"net/http"
)

// TODO: verifikasi Clerk JWT via middleware, context-kan user.
func MeHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"me":   "placeholder",
		"note": "Integrasikan Clerk JWT verifier di middleware.",
	})
}

func GoogleStatusHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider": "google",
		"status":   "not-connected (placeholder)",
	})
}
