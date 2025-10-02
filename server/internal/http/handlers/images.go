package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/sqlinline"
	"server/pkg/zip"

	"github.com/go-chi/chi/v5"
)

type imageGenerateRequest struct {
	Provider    string             `json:"provider"`
	Quantity    int                `json:"quantity"`
	AspectRatio string             `json:"aspect_ratio"`
	Prompt      jsoncfg.PromptJSON `json:"prompt"`
}

type jobResponse struct {
	JobID          string `json:"job_id"`
	Status         string `json:"status"`
	RemainingQuota int    `json:"remaining_quota"`
}

func (a *App) ImagesGenerate(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	var req imageGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "1:1"
	}
	provider := req.Provider
	if provider == "" {
		provider = "gemini"
	}
	if _, ok := a.ImageProviders[provider]; !ok {
		a.error(w, http.StatusBadRequest, "bad_request", "unsupported provider")
		return
	}
	promptBytes, _ := json.Marshal(req.Prompt)
	row := a.SQL.QueryRow(r.Context(), sqlinline.QEnqueueImageJob, userID, promptBytes, req.Quantity, req.AspectRatio, provider)
	var jobID string
	var remaining int
	if err := row.Scan(&jobID, &remaining); err != nil {
		if strings.Contains(err.Error(), "quota exceeded") {
			a.error(w, http.StatusForbidden, "quota_exceeded", "daily quota exceeded")
			return
		}
		a.error(w, http.StatusInternalServerError, "internal", "failed to queue job")
		return
	}
	a.json(w, http.StatusAccepted, jobResponse{JobID: jobID, Status: "QUEUED", RemainingQuota: remaining})
}

func (a *App) ImageStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job_id required")
		return
	}
	row := a.SQL.QueryRow(r.Context(), sqlinline.QSelectJobStatus, jobID)
	var id, userID, taskType, status, provider string
	var quantity int
	var aspect string
	var createdAt, updatedAt time.Time
	var props []byte
	if err := row.Scan(&id, &userID, &taskType, &status, &provider, &quantity, &aspect, &createdAt, &updatedAt, &props); err != nil {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"id":           id,
		"user_id":      userID,
		"task_type":    taskType,
		"status":       status,
		"provider":     provider,
		"quantity":     quantity,
		"aspect_ratio": aspect,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
		"properties":   json.RawMessage(props),
	})
}

func (a *App) ImageAssets(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	rows, err := a.SQL.Query(r.Context(), sqlinline.QSelectJobAssets, jobID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to load assets")
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

func (a *App) ImageZip(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	rows, err := a.SQL.Query(r.Context(), sqlinline.QSelectJobAssets, jobID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to fetch assets")
		return
	}
	defer rows.Close()
	var assets []zip.Asset
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
		assets = append(assets, zip.Asset{Filename: fmt.Sprintf("%s-%s", jobID, id), MIME: mime, URL: storageKey})
	}
	archive := zip.ArchiveAssets(assets)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=job-%s.zip", jobID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func (a *App) ImagesEnhance(w http.ResponseWriter, r *http.Request) {
	a.ImagesGenerate(w, r)
}
