package handlers

import (
	"encoding/json"
	"net/http"

	sq "server/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	DB *pgxpool.Pool
	Q  *sq.Queries
}

func NewApp(pool *pgxpool.Pool) *App {
	return &App{DB: pool, Q: sq.New(pool)}
}

func (a *App) json(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}
