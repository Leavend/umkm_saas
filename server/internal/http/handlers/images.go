package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
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

const maxUploadBytes = 12 << 20

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

func (a *App) ImagesUpload(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	if a.FileStore == nil {
		a.error(w, http.StatusInternalServerError, "internal", "file storage unavailable")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
	if err := r.ParseMultipartForm(maxUploadBytes + 1024); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid upload payload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}
	if len(data) == 0 {
		a.error(w, http.StatusBadRequest, "bad_request", "empty file")
		return
	}
	if len(data) > maxUploadBytes {
		a.error(w, http.StatusRequestEntityTooLarge, "too_large", "file exceeds 12MB limit")
		return
	}

	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	detectedMIME := http.DetectContentType(sniff)
	width, height, normalizedMIME, err := decodeImageDimensions(data, detectedMIME)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "unsupported image format")
		return
	}
	if normalizedMIME != "" {
		detectedMIME = normalizedMIME
	}
	if !isSupportedImageMime(detectedMIME) {
		a.error(w, http.StatusBadRequest, "bad_request", "format not supported")
		return
	}
	aspect := deriveAspectLabel(width, height)
	ext := extensionForUpload(detectedMIME)
	if ext == "" {
		ext = ".png"
	}
	storageKey := fmt.Sprintf("uploads/%s/%d%s", userID, time.Now().UnixNano(), ext)
	savedKey, err := a.FileStore.Write(r.Context(), storageKey, data)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to persist file")
		return
	}

	props := map[string]any{
		"source":            "upload",
		"original_filename": header.Filename,
		"filename":          filepath.Base(savedKey),
		"url":               a.assetURL(savedKey),
	}
	if mode := strings.TrimSpace(r.FormValue("mode")); mode != "" {
		props["mode"] = mode
	}
	if theme := strings.TrimSpace(r.FormValue("background_theme")); theme != "" {
		props["background_theme"] = theme
	}
	if enhance := strings.TrimSpace(r.FormValue("enhance_level")); enhance != "" {
		props["enhance_level"] = enhance
	}

	row := a.SQL.QueryRow(
		r.Context(),
		sqlinline.QInsertUploadedAsset,
		userID,
		"",
		savedKey,
		detectedMIME,
		int64(len(data)),
		width,
		height,
		aspect,
		jsoncfg.MustMarshal(props),
	)
	var assetID string
	if err := row.Scan(&assetID); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to record upload")
		return
	}

	a.json(w, http.StatusCreated, map[string]any{
		"asset_id":     assetID,
		"storage_key":  savedKey,
		"mime":         detectedMIME,
		"bytes":        len(data),
		"width":        width,
		"height":       height,
		"aspect_ratio": aspect,
		"url":          a.assetURL(savedKey),
	})
}

func decodeImageDimensions(data []byte, fallback string) (int, int, string, error) {
	reader := bytes.NewReader(data)
	cfg, format, err := image.DecodeConfig(reader)
	if err == nil {
		return cfg.Width, cfg.Height, mimeFromFormat(format, fallback), nil
	}
	if isLikelyWebP(data) || strings.Contains(strings.ToLower(fallback), "webp") {
		if width, height, webpErr := decodeWebPDimensions(data); webpErr == nil {
			return width, height, "image/webp", nil
		}
	}
	return 0, 0, fallback, err
}

func decodeWebPDimensions(data []byte) (int, int, error) {
	if len(data) < 30 {
		return 0, 0, fmt.Errorf("webp: insufficient data")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, fmt.Errorf("webp: invalid riff header")
	}
	chunk := string(data[12:16])
	switch chunk {
	case "VP8X":
		if len(data) < 30 {
			return 0, 0, fmt.Errorf("webp: truncated vp8x chunk")
		}
		width := int(uint32(data[24]) | uint32(data[25])<<8 | uint32(data[26])<<16)
		height := int(uint32(data[27]) | uint32(data[28])<<8 | uint32(data[29])<<16)
		return width + 1, height + 1, nil
	case "VP8 ":
		if len(data) < 30 {
			return 0, 0, fmt.Errorf("webp: truncated vp8 chunk")
		}
		rawW := binary.LittleEndian.Uint16(data[26:28])
		rawH := binary.LittleEndian.Uint16(data[28:30])
		return int(rawW & 0x3FFF), int(rawH & 0x3FFF), nil
	case "VP8L":
		if len(data) < 25 {
			return 0, 0, fmt.Errorf("webp: truncated vp8l chunk")
		}
		if data[20] != 0x2f {
			return 0, 0, fmt.Errorf("webp: invalid vp8l signature")
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		width := int(bits&0x3FFF) + 1
		height := int((bits>>14)&0x3FFF) + 1
		return width, height, nil
	default:
		return 0, 0, fmt.Errorf("webp: unsupported chunk %s", chunk)
	}
}

func isLikelyWebP(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func mimeFromFormat(format, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	default:
		return fallback
	}
}

func isSupportedImageMime(mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch mime {
	case "image/png", "image/jpeg", "image/jpg", "image/webp":
		return true
	default:
		return false
	}
}

func extensionForUpload(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func deriveAspectLabel(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	ratio := float64(width) / float64(height)
	targets := map[string]float64{
		"1:1":  1.0,
		"4:3":  4.0 / 3.0,
		"3:4":  3.0 / 4.0,
		"16:9": 16.0 / 9.0,
		"9:16": 9.0 / 16.0,
	}
	best := ""
	bestDiff := math.MaxFloat64
	for label, target := range targets {
		diff := math.Abs(ratio - target)
		if diff < bestDiff {
			bestDiff = diff
			best = label
		}
	}
	if best != "" && bestDiff <= 0.12 {
		return best
	}
	g := gcd(width, height)
	if g <= 0 {
		return fmt.Sprintf("%d:%d", width, height)
	}
	return fmt.Sprintf("%d:%d", width/g, height/g)
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
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
		provider = "qwen-image-plus"
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
