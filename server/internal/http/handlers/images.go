package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/internal/db"
	"server/internal/domain/jsoncfg"
	"server/internal/imagegen"
	"server/internal/sqlinline"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxUploadBytes = 12 << 20

type imageJobResponse struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id,omitempty"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	Status      string          `json:"status"`
	Quantity    int32           `json:"quantity"`
	AspectRatio *string         `json:"aspect_ratio,omitempty"`
	Prompt      json.RawMessage `json:"prompt"`
	SourceAsset json.RawMessage `json:"source_asset"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       *string         `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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
	if a.ImageEditor == nil {
		a.error(w, http.StatusServiceUnavailable, "unavailable", "image editor unavailable")
		return
	}

	var req imagegen.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}

	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	if provider == "" || provider == "qwen-image-plus" {
		provider = "qwen-image-edit"
	}
	if provider != "qwen-image-edit" {
		a.error(w, http.StatusBadRequest, "bad_request", "unsupported provider")
		return
	}

	sourceURL := strings.TrimSpace(req.Prompt.SourceAsset.URL)
	parsedURL, err := url.Parse(sourceURL)
	if err != nil || parsedURL == nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		a.error(w, http.StatusUnprocessableEntity, "invalid_source", "prompt.source_asset.url must be a public http(s) URL")
		return
	}
	host := strings.ToLower(parsedURL.Hostname())
	_, allowlisted := a.sourceHostAllowlist[host]
	if err := ensurePublicHTTPURL(parsedURL, a.sourceHostAllowlist); err != nil {
		a.error(w, http.StatusUnprocessableEntity, "invalid_source", err.Error())
		return
	}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 8 {
		quantity = 8
	}

	q := db.New(a.DB)

	promptJSON, err := json.Marshal(req.Prompt)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "failed to encode prompt")
		return
	}
	sourceJSON, err := json.Marshal(req.Prompt.SourceAsset)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "failed to encode source asset")
		return
	}

	var aspectPtr *string
	aspect := strings.TrimSpace(req.AspectRatio)
	if aspect != "" {
		aspectPtr = &aspect
	}
	var userPtr *string
	if userID != "" {
		userPtr = &userID
	}

	jobID, err := q.CreateImageJob(r.Context(), db.CreateImageJobParams{
		UserID:      userPtr,
		Provider:    provider,
		Model:       "qwen-image-edit",
		Quantity:    int32(quantity),
		AspectRatio: aspectPtr,
		Prompt:      promptJSON,
		SourceAsset: sourceJSON,
	})
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to create job")
		return
	}

	source, err := a.prepareSourceImage(r.Context(), sourceURL, parsedURL, req.Prompt.SourceAsset.AssetID, allowlisted)
	if err != nil {
		_ = q.FailImageJob(r.Context(), db.FailImageJobParams{ID: jobID, Error: err.Error()})
		a.error(w, http.StatusUnprocessableEntity, "invalid_source", err.Error())
		return
	}

	if err := q.StartImageJob(r.Context(), jobID); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to start job")
		return
	}

	instruction := imagegen.BuildInstruction(req)
	negative := ""
	if req.Prompt.Extras != nil {
		if v, ok := req.Prompt.Extras["negative_prompt"].(string); ok {
			negative = v
		}
	}

	results := make([]struct {
		url string
		err error
	}, quantity)
	var wg sync.WaitGroup
	for i := 0; i < quantity; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.acquireImageSlot(r.Context()); err != nil {
				results[idx].err = err
				return
			}
			defer a.releaseImageSlot()
			ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
			defer cancel()
			url, err := a.ImageEditor.EditOnce(ctx, source, instruction, req.Prompt.Watermark.Enabled, negative, nil)
			results[idx] = struct {
				url string
				err error
			}{url: url, err: err}
		}()
	}
	wg.Wait()

	var urls []string
	for _, res := range results {
		if res.err != nil {
			_ = q.FailImageJob(r.Context(), db.FailImageJobParams{ID: jobID, Error: res.err.Error()})
			a.error(w, http.StatusBadGateway, "generation_failed", res.err.Error())
			return
		}
		urls = append(urls, res.url)
	}

	outputPayload := map[string]any{
		"images": func() []map[string]string {
			items := make([]map[string]string, 0, len(urls))
			for _, u := range urls {
				items = append(items, map[string]string{"url": u})
			}
			return items
		}(),
	}
	outputJSON, err := json.Marshal(outputPayload)
	if err != nil {
		_ = q.FailImageJob(r.Context(), db.FailImageJobParams{ID: jobID, Error: err.Error()})
		a.error(w, http.StatusInternalServerError, "internal", "failed to encode output")
		return
	}

	if err := q.CompleteImageJob(r.Context(), db.CompleteImageJobParams{ID: jobID, Output: outputJSON}); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to persist output")
		return
	}

	a.json(w, http.StatusCreated, imagegen.GenerateResponse{
		JobID:  jobID.String(),
		Status: "SUCCEEDED",
		Images: urls,
	})
}

func (a *App) ImageJob(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job id required")
		return
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid job id")
		return
	}
	q := db.New(a.DB)
	job, err := q.GetImageJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.error(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		a.error(w, http.StatusInternalServerError, "internal", "failed to load job")
		return
	}
	if job.UserID.Valid && job.UserID.String != userID {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}

	var aspectPtr *string
	if job.AspectRatio.Valid {
		v := job.AspectRatio.String
		aspectPtr = &v
	}
	var errPtr *string
	if job.Error.Valid && job.Error.String != "" {
		v := job.Error.String
		errPtr = &v
	}
	userVal := ""
	if job.UserID.Valid {
		userVal = job.UserID.String
	}
	resp := imageJobResponse{
		ID:          job.ID.String(),
		UserID:      userVal,
		Provider:    job.Provider,
		Model:       job.Model,
		Status:      job.Status,
		Quantity:    job.Quantity,
		AspectRatio: aspectPtr,
		Prompt:      json.RawMessage(job.Prompt),
		SourceAsset: json.RawMessage(job.SourceAsset),
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
	if len(job.Output) > 0 {
		resp.Output = json.RawMessage(job.Output)
	}
	resp.Error = errPtr

	a.json(w, http.StatusOK, resp)
}

func (a *App) ImageDownload(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	idStr := chi.URLParam(r, "job_id")
	if idStr == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job id required")
		return
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid job id")
		return
	}
	q := db.New(a.DB)
	job, err := q.GetImageJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.error(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		a.error(w, http.StatusInternalServerError, "internal", "failed to load job")
		return
	}
	if job.UserID.Valid && job.UserID.String != userID {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	if job.Status != "SUCCEEDED" || len(job.Output) == 0 {
		a.error(w, http.StatusConflict, "job_pending", "job has not completed")
		return
	}
	urls := extractImageURLs(job.Output)
	if len(urls) == 0 {
		a.error(w, http.StatusNotFound, "no_image", "no image available")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, urls[0], nil)
	if err != nil {
		a.error(w, http.StatusBadGateway, "download_error", err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.error(w, http.StatusBadGateway, "download_error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		a.error(w, http.StatusBadGateway, "download_error", fmt.Sprintf("remote status %d", resp.StatusCode))
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=job-%s.png", job.ID.String()))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func (a *App) ImageDownloadZip(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	idStr := chi.URLParam(r, "job_id")
	if idStr == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "job id required")
		return
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid job id")
		return
	}
	q := db.New(a.DB)
	job, err := q.GetImageJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.error(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		a.error(w, http.StatusInternalServerError, "internal", "failed to load job")
		return
	}
	if job.UserID.Valid && job.UserID.String != userID {
		a.error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}

	urls := extractImageURLs(job.Output)
	if len(urls) == 0 {
		a.error(w, http.StatusNotFound, "no_image", "no image available")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=job-%s.zip", job.ID.String()))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for idx, imgURL := range urls {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imgURL, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			resp.Body.Close()
			continue
		}
		filename := fmt.Sprintf("image_%02d.png", idx+1)
		if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "jpeg") {
			filename = fmt.Sprintf("image_%02d.jpg", idx+1)
		}
		writer, err := zipWriter.Create(filename)
		if err != nil {
			resp.Body.Close()
			continue
		}
		_, _ = io.Copy(writer, resp.Body)
		resp.Body.Close()
	}
}

func (a *App) acquireImageSlot(ctx context.Context) error {
	if a.imageLimiter == nil {
		return nil
	}
	select {
	case a.imageLimiter <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) releaseImageSlot() {
	if a.imageLimiter == nil {
		return
	}
	select {
	case <-a.imageLimiter:
	default:
	}
}

func extractImageURLs(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	urls := make([]string, 0, len(payload.Images))
	for _, item := range payload.Images {
		if u := strings.TrimSpace(item.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func (a *App) prepareSourceImage(ctx context.Context, rawURL string, parsed *url.URL, assetID string, allowlisted bool) (imagegen.SourceImage, error) {
	src := imagegen.SourceImage{URL: rawURL}
	baseName := strings.TrimSpace(path.Base(parsed.Path))
	if baseName != "" && baseName != "." && baseName != "/" {
		src.Name = baseName
	} else {
		src.Name = strings.TrimSpace(assetID)
	}
	if allowlisted {
		data, mimeType, err := a.fetchAllowlistedSource(ctx, rawURL)
		if err != nil {
			return imagegen.SourceImage{}, err
		}
		src.Data = data
		src.MIMEType = mimeType
	}
	return src, nil
}

func (a *App) fetchAllowlistedSource(ctx context.Context, rawURL string) ([]byte, string, error) {
	client := a.sourceFetcher
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request for source asset: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch source asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("failed to fetch source asset: http %d", resp.StatusCode)
	}
	const maxSourceBytes = 20 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read source asset: %w", err)
	}
	if len(data) > maxSourceBytes {
		return nil, "", errors.New("source asset exceeds 20MB limit")
	}
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	return data, mimeType, nil
}

func ensurePublicHTTPURL(u *url.URL, allowlist map[string]struct{}) error {
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("prompt.source_asset.url must include a hostname")
	}
	lower := strings.ToLower(host)
	if _, ok := allowlist[lower]; ok {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
			return errors.New("prompt.source_asset.url must be publicly accessible")
		}
		return nil
	}
	if lower == "localhost" || strings.HasSuffix(lower, ".local") || strings.HasSuffix(lower, ".internal") {
		return errors.New("prompt.source_asset.url must be publicly accessible")
	}
	return nil
}
