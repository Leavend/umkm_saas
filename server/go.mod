module server

go 1.24.0

require (
	github.com/go-chi/chi/v5 v5.2.3
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.4
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v0.0.0
	github.com/oschwald/geoip2-golang v1.11.0
	github.com/rs/zerolog v1.33.0
	golang.org/x/text v0.29.0
)

// Provide lightweight stub implementations so the module graph remains
// buildable in constrained environments (e.g. CI without network access).
// Production builds should override these replacements to pull the real
// dependencies.
replace (
	github.com/jackc/pgx/v5 => ./internal/stubs/pgx
	github.com/lib/pq => ./internal/stubs/libpq
	github.com/oschwald/geoip2-golang => ./internal/stubs/geoip2
	github.com/rs/zerolog => ./internal/stubs/zerolog
)
