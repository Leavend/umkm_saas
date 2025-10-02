package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/middleware"
	"server/internal/providers/prompt"
	"server/internal/sqlinline"
)

type promptEnhanceRequest struct {
	Prompt jsoncfg.PromptJSON `json:"prompt"`
}

type promptEnhanceResponse struct {
	Prompt jsoncfg.PromptJSON `json:"prompt"`
	Ideas  []map[string]any   `json:"ideas"`
	Extra  map[string]string  `json:"extra"`
}

func (a *App) PromptEnhance(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	var req promptEnhanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	locale := middleware.LocaleFromContext(r.Context())
	res, err := a.PromptEnhancer.Enhance(r.Context(), prompt.EnhanceRequest{
		Title:       req.Prompt.Title,
		ProductType: req.Prompt.ProductType,
		Locale:      locale,
	})
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "enhancer failed")
		return
	}
	enriched := req.Prompt
	enriched.Extras.Locale = locale
	ideas := []map[string]any{{
		"title":       res.Title,
		"description": res.Description,
		"keywords":    res.Keywords,
	}}
	_, _ = a.SQL.Exec(r.Context(), sqlinline.QInsertUsageEvent, userID, nil, "PROMPT_ENHANCE", true, 0, jsoncfg.MustMarshal(map[string]any{"locale": locale}))
	a.json(w, http.StatusOK, promptEnhanceResponse{Prompt: enriched, Ideas: ideas, Extra: res.Metadata})
}

func (a *App) PromptRandom(w http.ResponseWriter, r *http.Request) {
	locale := middleware.LocaleFromContext(r.Context())
	list, err := a.PromptEnhancer.Random(r.Context(), locale)
	if err != nil {
		a.error(w, http.StatusInternalServerError, "internal", "failed to fetch prompts")
		return
	}
	a.json(w, http.StatusOK, map[string]any{"items": list, "generated_at": time.Now()})
}
