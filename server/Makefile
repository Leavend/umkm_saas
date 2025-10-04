GO ?= go
GOOSE ?= goose
MIGRATION_DIR := db/migrations

ifneq (,$(wildcard ./.env))
include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env)
endif

.PHONY: run worker migrate fmt vet lint test sqllint verify set-gemini-key

run:
	@set -a; . ./.env 2>/dev/null || true; set +a; \
	$(GO) run ./cmd/api

worker:
	@set -a; . ./.env 2>/dev/null || true; set +a; \
	$(GO) run ./cmd/worker

migrate:
	$(GOOSE) -dir $(MIGRATION_DIR) postgres "$(DATABASE_URL)" up

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required"; exit 1; }
	golangci-lint run

test:
	$(GO) test ./...

sqllint:
	$(GO) run ./internal/tools/sqllint

verify: fmt vet lint sqllint test

set-gemini-key:
	@set -a; . ./.env 2>/dev/null || true; set +a; \
	$(GO) run ./cmd/geminikey -key "$$GEMINI_API_KEY"
