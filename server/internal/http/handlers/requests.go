package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type EnqueueReq struct {
	Provider     string         `json:"provider"`      // NANO_BANANA | GOOGLE_VEO | ...
	Model        string         `json:"model"`         // veo-2 | veo-3 | ...
	SourceAsset  *string        `json:"source_asset"`  // UUID
	PromptJSON   map[string]any `json:"prompt_json"`   // disimpan ke JSONB
	AspectRatio  string         `json:"aspect_ratio"`  // 1:1, 4:3, 3:4, 16:9, 9:16
	Quantity     int            `json:"quantity"`      // server akan cap max 2 utk free
	TaskType     string         `json:"task_type"`     // IMAGE_GEN | VIDEO_GEN | UPSCALE
}

func EnqueueRequestHandler(w http.ResponseWriter, r *http.Request) {
	// NOTE: Di implementasi nyata, panggil sqlc: EnqueueGeneration(...)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      "REQUEST-UUID-PLACEHOLDER",
		"status":  "QUEUED",
		"message": "enqueue accepted (placeholder)",
	})
}

func GetRequestStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"status":   "RUNNING",
		"progress": 42,
	})
}
