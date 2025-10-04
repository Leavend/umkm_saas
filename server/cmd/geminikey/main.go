package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/infra"
	"server/internal/infra/credentials"
)

func main() {
	var keyFlag string
	flag.StringVar(&keyFlag, "key", "", "Gemini API key (optional if GEMINI_API_KEY is set)")
	flag.Parse()

	key := strings.TrimSpace(keyFlag)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	}
	if key == "" {
		fmt.Fprintln(os.Stderr, "Gemini API key is required via -key or GEMINI_API_KEY")
		os.Exit(1)
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger := infra.NewLogger("cli").With().Str("cmd", "geminikey").Logger()
	store := credentials.NewStore(infra.NewSQLRunner(pool, logger))

	ctxExec, cancelExec := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelExec()
	if err := store.SetGeminiAPIKey(ctxExec, key); err != nil {
		fmt.Fprintf(os.Stderr, "failed to persist gemini api key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Gemini API key stored successfully")
}
