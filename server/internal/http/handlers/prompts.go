package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/middleware"
	"server/internal/providers/prompt"
	"server/internal/sqlinline"

	"github.com/google/uuid"
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
	req.Prompt.Normalize(locale)
	if err := req.Prompt.Validate(); err != nil {
		a.error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	enhanceReq := prompt.EnhanceRequest{Prompt: req.Prompt, Locale: req.Prompt.Extras.Locale}
	started := time.Now()
	res, err := a.PromptEnhancer.Enhance(r.Context(), enhanceReq)
	success := err == nil && res != nil
	latency := int(time.Since(started).Milliseconds())
	if latency < 0 {
		latency = 0
	}
	if !success {
		a.logUsageEvent(r, userID, "PROMPT_ENHANCE", false, latency, map[string]any{"error": "enhancer_failed"})
		a.error(w, http.StatusInternalServerError, "internal", "enhancer failed")
		return
	}
	enriched := req.Prompt
	if res.Metadata != nil {
		if v, ok := res.Metadata["locale"]; ok && v != "" {
			enriched.Extras.Locale = v
		}
	}
	ideas := make([]map[string]any, 0, len(res.Ideas))
	for _, idea := range res.Ideas {
		ideas = append(ideas, map[string]any{
			"title":       idea.Title,
			"description": idea.Description,
			"keywords":    idea.Keywords,
		})
	}
	if len(ideas) == 0 {
		ideas = append(ideas, map[string]any{
			"title":       res.Title,
			"description": res.Description,
			"keywords":    res.Keywords,
		})
	}
	props := map[string]any{
		"locale":   enriched.Extras.Locale,
		"provider": res.Provider,
	}
	if len(res.Metadata) > 0 {
		props["metadata"] = res.Metadata
	}
	a.logUsageEvent(r, userID, "PROMPT_ENHANCE", true, latency, props)
	a.json(w, http.StatusOK, promptEnhanceResponse{Prompt: enriched, Ideas: ideas, Extra: res.Metadata})
}

func (a *App) PromptRandom(w http.ResponseWriter, r *http.Request) {
	userID := a.currentUserID(r)
	if userID == "" {
		a.error(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	locale := middleware.LocaleFromContext(r.Context())
	started := time.Now()
	list, err := a.PromptEnhancer.Random(r.Context(), locale)
	success := err == nil
	latency := int(time.Since(started).Milliseconds())
	if latency < 0 {
		latency = 0
	}
	if !success {
		a.logUsageEvent(r, userID, "PROMPT_RANDOM", false, latency, map[string]any{"locale": locale})
		a.error(w, http.StatusInternalServerError, "internal", "failed to fetch prompts")
		return
	}
	provider := ""
	if len(list) > 0 {
		provider = list[0].Provider
	}
	props := map[string]any{"locale": locale, "provider": provider}
	if provider == "static" && len(list) > 0 {
		if reason := list[0].Metadata["fallback_reason"]; reason != "" {
			props["fallback_reason"] = reason
		}
	}
	a.logUsageEvent(r, userID, "PROMPT_RANDOM", true, latency, props)
	a.json(w, http.StatusOK, map[string]any{"items": list, "generated_at": time.Now()})
}

func (a *App) logUsageEvent(r *http.Request, userID, event string, success bool, latency int, props map[string]any) {
	if userID == "" {
		return
	}
	var requestID any
	if id := middleware.RequestIDFromContext(r.Context()); id != "" {
		if parsed, err := uuid.Parse(id); err == nil {
			requestID = parsed
		}
	}
	if props == nil {
		props = map[string]any{}
	}
	payload := jsoncfg.MustMarshal(props)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := a.SQL.Exec(ctx, sqlinline.QInsertUsageEvent, userID, requestID, event, success, latency, payload); err != nil {
		a.Logger.Error().Err(err).Str("event", event).Msg("log usage failed")
	}
}
