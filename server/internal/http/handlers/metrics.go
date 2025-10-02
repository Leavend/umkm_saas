package handlers

import (
	"encoding/json"
	"net/http"
)

func Dashboard24hHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"image_generated": 12,
		"video_generated": 3,
		"request_success": 14,
		"request_fail":    1,
		"active_online":   5,
	})
}
