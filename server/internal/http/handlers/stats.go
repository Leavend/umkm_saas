package handlers

import (
	"net/http"

	"server/internal/sqlinline"
)

func (a *App) StatsSummary(w http.ResponseWriter, r *http.Request) {
	row := a.SQL.QueryRow(r.Context(), sqlinline.QStatsSummary)
	var totalUsers, imageGenerated, videoGenerated, requestSuccess, requestFail, image24, video24 int64
	if err := row.Scan(&totalUsers, &imageGenerated, &videoGenerated, &requestSuccess, &requestFail, &image24, &video24); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to load stats")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"total_users":     totalUsers,
		"image_generated": imageGenerated,
		"video_generated": videoGenerated,
		"request_success": requestSuccess,
		"request_fail":    requestFail,
		"image_last_24h":  image24,
		"video_last_24h":  video24,
	})
}
