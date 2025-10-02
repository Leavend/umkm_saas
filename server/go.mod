module server

go 1.22

require (
    github.com/go-chi/chi/v5 v5.2.3
    github.com/google/uuid v1.6.0
    github.com/jackc/pgx/v5 v5.7.4
    github.com/lib/pq v0.0.0
    github.com/joho/godotenv v1.5.1
    github.com/pressly/goose/v3 v3.21.0
    github.com/rs/zerolog v1.33.0
)

replace github.com/jackc/pgx/v5 => ./internal/stubs/pgx
replace github.com/rs/zerolog => ./internal/stubs/zerolog
replace github.com/lib/pq => ./internal/stubs/libpq
replace github.com/pressly/goose/v3 => ./internal/stubs/goose
