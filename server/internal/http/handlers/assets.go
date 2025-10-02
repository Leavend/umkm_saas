package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func ListAssetsHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode([]map[string]any{
		{"id": "ASSET-UUID-1", "kind": "ORIGINAL"},
		{"id": "ASSET-UUID-2", "kind": "GENERATED"},
	})
}

func DownloadAssetHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      chi.URLParam(r, "id"),
		"message": "signed-url placeholder",
		"url":     "https://storage.example/signed-url",
	})
}
