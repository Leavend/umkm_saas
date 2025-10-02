package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Env         string
	Port        string
}

func Load() Config {
	// Baca file env (jika ada). Tidak error kalau file tidak ada.
	_ = godotenv.Load(".env", ".env.local")

	c := Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),
		Env:         getenv("APP_ENV", "development"),
		Port:        getenv("PORT", "8080"),
	}
	log.Printf("Config loaded (env=%s, port=%s)", c.Env, c.Port)
	return c
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
