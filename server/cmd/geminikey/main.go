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
	var (
		keyFlag      string
		providerFlag string
	)
	flag.StringVar(&keyFlag, "key", "", "API key for the selected provider (fallbacks to environment)")
	flag.StringVar(&providerFlag, "provider", credentials.ProviderGemini, "Prompt provider to configure (gemini or openai)")
	flag.Parse()

	provider := strings.TrimSpace(strings.ToLower(providerFlag))
	switch provider {
	case credentials.ProviderGemini, credentials.ProviderOpenAI:
	case "":
		provider = credentials.ProviderGemini
	default:
		fmt.Fprintf(os.Stderr, "unsupported provider %q\n", providerFlag)
		os.Exit(1)
	}

	key := strings.TrimSpace(keyFlag)
	if key == "" {
		switch provider {
		case credentials.ProviderOpenAI:
			key = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		default:
			key = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		}
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "%s API key is required via -key or environment\n", strings.ToUpper(provider))
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

	logger := infra.NewLogger("cli").With().Str("cmd", "geminikey").Str("provider", provider).Logger()
	store := credentials.NewStore(infra.NewSQLRunner(pool, logger))

	ctxExec, cancelExec := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelExec()
	var persistErr error
	switch provider {
	case credentials.ProviderOpenAI:
		persistErr = store.SetOpenAIAPIKey(ctxExec, key)
	default:
		persistErr = store.SetGeminiAPIKey(ctxExec, key)
	}
	if persistErr != nil {
		fmt.Fprintf(os.Stderr, "failed to persist %s api key: %v\n", provider, persistErr)
		os.Exit(1)
	}

	fmt.Printf("%s API key stored successfully\n", strings.ToUpper(provider))
}
