package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	sq "server/internal/db/sqlc"
)

type App struct {
	DB *sql.DB
	Q  *sq.Queries
}

func NewApp(db *sql.DB) *App {
	return &App{DB: db, Q: sq.New(db)}
}

func (a *App) json(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
