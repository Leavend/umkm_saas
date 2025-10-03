package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/sqlinline"

	"github.com/go-chi/chi/v5"
)

type videoGenerateRequest struct {
	Provider string `json:"provider"`
	Prompt   string `json:"prompt"`
	Locale   string `json:"locale"`
}

func (a *App) VideosGenerate(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	var req videoGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if req.Provider == "" {
		req.Provider = "veo2"
	}
	if _, ok := a.VideoProviders[req.Provider]; !ok {
		a.error(w, http.StatusBadRequest, "bad_request", "unsupported provider")
		return
	}
	promptPayload := map[string]any{
		"version": "2024-06-01",
		"prompt":  req.Prompt,
	}
	if req.Locale != "" {
		promptPayload["locale"] = req.Locale
	}
	promptJSON := jsoncfg.MustMarshal(promptPayload)
	row := a.SQL.QueryRow(r.Context(), sqlinline.QEnqueueVideoJob, userID, promptJSON, req.Provider)
	var jobID string
	var remaining int
	if err := row.Scan(&jobID, &remaining); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to queue video job")
		return
	}
	a.json(w, http.StatusAccepted, jobResponse{JobID: jobID, Status: "QUEUED", RemainingQuota: remaining})
}

func (a *App) VideoStatus(w http.ResponseWriter, r *http.Request) {
	a.ImageStatus(w, r)
}

func (a *App) VideoAssets(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	rows, err := a.SQL.Query(r.Context(), sqlinline.QSelectJobAssets, jobID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to fetch video assets")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, storageKey, mime string
		var bytes int64
		var width, height int
		var aspect string
		var props []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &storageKey, &mime, &bytes, &width, &height, &aspect, &props, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]any{
			"id":           id,
			"storage_key":  storageKey,
			"mime":         mime,
			"bytes":        bytes,
			"width":        width,
			"height":       height,
			"aspect_ratio": aspect,
			"properties":   json.RawMessage(props),
			"created_at":   createdAt,
		})
	}
	a.json(w, http.StatusOK, map[string]any{"items": items})
}
