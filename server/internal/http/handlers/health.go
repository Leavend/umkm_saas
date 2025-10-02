package handlers

import (
	"net/http"
)

func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	a.json(w, http.StatusOK, map[string]string{"status": "ok"})
}
