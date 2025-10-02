package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"server/internal/middleware"
	"server/internal/sqlinline"
)

type googleVerifyRequest struct {
	IDToken string `json:"id_token"`
}

type googleVerifyResponse struct {
	Token string         `json:"token"`
	User  userProfileDTO `json:"user"`
}

type userProfileDTO struct {
	ID            string         `json:"id"`
	Email         string         `json:"email"`
	Plan          string         `json:"plan"`
	Locale        string         `json:"locale"`
	QuotaDaily    int            `json:"quota_daily"`
	QuotaUsed     int            `json:"quota_used_today"`
	PropertiesRaw map[string]any `json:"properties"`
}

func (a *App) AuthGoogleVerify(w http.ResponseWriter, r *http.Request) {
	var req googleVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if req.IDToken == "" {
		a.error(w, http.StatusBadRequest, "bad_request", "id_token required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	claims, err := a.GoogleVerifier.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		a.Logger.Error().Err(err).Msg("google verify failed")
		a.error(w, http.StatusUnauthorized, "unauthorized", "invalid google token")
		return
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	picture, _ := claims["picture"].(string)
	locale, _ := claims["locale"].(string)
	if locale == "" {
		locale = "en"
	}
	row := a.SQL.QueryRow(r.Context(), sqlinline.QUpsertGoogleUser, sub, email, name, picture, locale)
	var userID string
	var plan string
	var propsBytes []byte
	if err := row.Scan(&userID, &plan, &propsBytes); err != nil {
		a.Logger.Error().Err(err).Msg("upsert user failed")
		a.error(w, http.StatusInternalServerError, "internal", "failed to persist user")
		return
	}
	props, quotaDaily, quotaUsed := extractQuota(propsBytes)
	token, err := middleware.SignJWT(a.JWTSecret, middleware.TokenClaims{
		Sub:      userID,
		Plan:     plan,
		Locale:   locale,
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
		Issuer:   "umkm-saas",
		Audience: "umkm-clients",
	})
	if err != nil {
		a.Logger.Error().Err(err).Msg("sign jwt failed")
		a.error(w, http.StatusInternalServerError, "internal", "failed to sign token")
		return
	}
	a.json(w, http.StatusOK, googleVerifyResponse{
		Token: token,
		User: userProfileDTO{
			ID:            userID,
			Email:         email,
			Plan:          plan,
			Locale:        locale,
			QuotaDaily:    quotaDaily,
			QuotaUsed:     quotaUsed,
			PropertiesRaw: props,
		},
	})
}

func (a *App) Me(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	row := a.SQL.QueryRow(r.Context(), sqlinline.QSelectUserByID, userID)
	var id, googleSub, email, locale, plan string
	var propsBytes []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &googleSub, &email, &locale, &plan, &propsBytes, &createdAt, &updatedAt); err != nil {
		a.error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	props, quotaDaily, quotaUsed := extractQuota(propsBytes)
	a.json(w, http.StatusOK, userProfileDTO{
		ID:            id,
		Email:         email,
		Plan:          plan,
		Locale:        locale,
		QuotaDaily:    quotaDaily,
		QuotaUsed:     quotaUsed,
		PropertiesRaw: props,
	})
}

func extractQuota(b []byte) (map[string]any, int, int) {
	props := map[string]any{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &props)
	}
	quotaDaily := 2
	quotaUsed := 0
	if v, ok := props["quota_daily"].(float64); ok {
		quotaDaily = int(v)
	}
	if v, ok := props["quota_used_today"].(float64); ok {
		quotaUsed = int(v)
	}
	return props, quotaDaily, quotaUsed
}

func (a *App) PromptClear(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	_, err := a.SQL.Exec(r.Context(), sqlinline.QInsertUsageEvent, userID, nil, "PROMPT_CLEAR", true, 0, json.RawMessage(`{"action":"clear"}`))
	if err != nil && !errors.Is(err, context.Canceled) {
		a.Logger.Error().Err(err).Msg("log usage failed")
	}
	w.WriteHeader(http.StatusNoContent)
}
