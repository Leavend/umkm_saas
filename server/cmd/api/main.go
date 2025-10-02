package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"server/internal/config"
	api "server/internal/http"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := api.NewRouter(db)
	addr := ":" + cfg.Port
	log.Printf("API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
