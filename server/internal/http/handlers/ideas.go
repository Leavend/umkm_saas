package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"server/internal/domain/jsoncfg"
)

type ideasFromImageRequest struct {
	ImageBase64 string `json:"image_base64"`
}

func (a *App) IdeasFromImage(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	var req ideasFromImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if req.ImageBase64 == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "image_base64 required")
		return
	}
	ideas := []jsoncfg.IdeaSuggestion{
		{Title: "Kreasi Menu Sarapan", Description: "Tampilkan hidangan dengan prop besar", Tags: []string{"sarapan", "umkm"}},
		{Title: "Promo Paket Hemat", Description: "Gunakan caption yang menonjolkan harga", Tags: []string{"promo", "hemat"}},
	}
	a.json(w, http.StatusOK, map[string]any{"ideas": ideas, "generated_at": time.Now()})
}
