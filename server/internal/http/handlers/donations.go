package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"server/internal/sqlinline"
)

type donationRequest struct {
	Amount      int64   `json:"amount"`
	Note        string  `json:"note"`
	Testimonial *string `json:"testimonial"`
}

func (a *App) DonationsCreate(w http.ResponseWriter, r *http.Request) {
	var req donationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if req.Amount <= 0 {
		a.error(w, http.StatusBadRequest, "bad_request", "amount must be positive")
		return
	}
	userID := a.currentUserID(r)
	testimonial := ""
	if req.Testimonial != nil {
		testimonial = *req.Testimonial
	}
	row := a.SQL.QueryRow(r.Context(), sqlinline.QInsertDonation, userID, req.Amount, req.Note, testimonial, json.RawMessage(`{}`))
	var donationID string
	if err := row.Scan(&donationID); err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to create donation")
		return
	}
	a.json(w, http.StatusCreated, map[string]any{"id": donationID})
}

func (a *App) DonationsTestimonials(w http.ResponseWriter, r *http.Request) {
	rows, err := a.SQL.Query(r.Context(), sqlinline.QListDonations, 10)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to load donations")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, note, testimonial string
		var userID sql.NullString
		var amount int64
		var props []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &userID, &amount, &note, &testimonial, &props, &createdAt); err != nil {
			continue
		}
		var normalizedUserID any
		if userID.Valid {
			normalizedUserID = userID.String
		}
		items = append(items, map[string]any{
			"id":          id,
			"user_id":     normalizedUserID,
			"amount":      amount,
			"note":        note,
			"testimonial": testimonial,
			"created_at":  createdAt,
			"properties":  json.RawMessage(props),
		})
	}
	a.json(w, http.StatusOK, map[string]any{"items": items})
}
