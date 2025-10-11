package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"server/internal/sqlinline"

	"github.com/go-chi/chi/v5"
)

func (a *App) ListAssets(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, err := a.SQL.Query(r.Context(), sqlinline.QListAssetsByUser, userID, limit, offset)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to load assets")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, requestID, storageKey, mime string
		var bytes int64
		var width, height int
		var aspect string
		var props []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &requestID, &storageKey, &mime, &bytes, &width, &height, &aspect, &props, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]any{
			"id":           id,
			"request_id":   requestID,
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

func (a *App) DownloadAsset(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	assetID := chi.URLParam(r, "id")
	row := a.SQL.QueryRow(r.Context(), sqlinline.QSelectAssetByID, assetID)
	var id, ownerID, storageKey, mime string
	var bytes int64
	var width, height int
	var aspect string
	var props []byte
	if err := row.Scan(&id, &ownerID, &storageKey, &mime, &bytes, &width, &height, &aspect, &props); err != nil {
		a.error(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	if ownerID != userID {
		a.error(w, http.StatusForbidden, "forbidden", "not your asset")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"url":          a.assetURL(storageKey),
		"mime":         mime,
		"width":        width,
		"height":       height,
		"aspect_ratio": aspect,
	})
}
