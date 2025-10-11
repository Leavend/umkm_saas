package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/middleware"
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
	locale := middleware.LocaleFromContext(r.Context())
	req.Prompt.Normalize(locale)
	if req.Quantity <= 0 {
		req.Quantity = req.Prompt.Quantity
		if req.Quantity <= 0 {
			req.Quantity = jsoncfg.DefaultPromptQuantity
		}
	}
	if req.Quantity > jsoncfg.MaxPromptQuantity {
		req.Quantity = jsoncfg.MaxPromptQuantity
	}
	req.Prompt.Quantity = req.Quantity
	if req.AspectRatio == "" {
		req.AspectRatio = req.Prompt.AspectRatio
		if req.AspectRatio == "" {
			req.AspectRatio = jsoncfg.DefaultPromptAspectRatio
		}
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
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job_id required")
		return
	}
	job, err := a.loadJobForUser(r.Context(), jobID, userID)
	if err != nil {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"id":           job.ID,
		"user_id":      job.UserID,
		"task_type":    job.TaskType,
		"status":       job.Status,
		"provider":     job.Provider,
		"quantity":     job.Quantity,
		"aspect_ratio": job.Aspect,
		"created_at":   job.CreatedAt,
		"updated_at":   job.UpdatedAt,
		"properties":   json.RawMessage(job.Properties),
	})
}

func (a *App) ImageAssets(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job_id required")
		return
	}
	if _, err := a.loadJobForUser(r.Context(), jobID, userID); err != nil {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	rows, err := a.SQL.Query(r.Context(), sqlinline.QSelectJobAssets, jobID, userID)
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
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job_id required")
		return
	}
	if _, err := a.loadJobForUser(r.Context(), jobID, userID); err != nil {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	rows, err := a.SQL.Query(r.Context(), sqlinline.QSelectJobAssets, jobID, userID)
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
		data := loadAssetData(a.Config.StoragePath, storageKey)
		assets = append(assets, zip.Asset{Filename: fmt.Sprintf("%s-%s", jobID, id), MIME: mime, Data: data})
	}
	archive := zip.ArchiveAssets(assets)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=job-%s.zip", jobID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func loadAssetData(basePath, storageKey string) []byte {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil
	}
	lower := strings.ToLower(storageKey)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return []byte(storageKey)
	}
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil
	}
	path := filepath.Join(basePath, filepath.FromSlash(strings.TrimLeft(storageKey, "/")))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func (a *App) ImagesEnhance(w http.ResponseWriter, r *http.Request) {
	a.ImagesGenerate(w, r)
}

type jobRecord struct {
	ID         string
	UserID     string
	TaskType   string
	Status     string
	Provider   string
	Quantity   int
	Aspect     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Properties []byte
}

func (a *App) loadJobForUser(ctx context.Context, jobID, userID string) (*jobRecord, error) {
	row := a.SQL.QueryRow(ctx, sqlinline.QSelectJobStatus, jobID, userID)
	var job jobRecord
	if err := row.Scan(&job.ID, &job.UserID, &job.TaskType, &job.Status, &job.Provider, &job.Quantity, &job.Aspect, &job.CreatedAt, &job.UpdatedAt, &job.Properties); err != nil {
		return nil, err
	}
	return &job, nil
}
