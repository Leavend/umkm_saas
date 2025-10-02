GO ?= go
GOOSE ?= goose
MIGRATION_DIR := db/migrations

.PHONY: run worker migrate fmt vet lint test sqllint verify

run:
	@set -a; \
	. ./.env 2>/dev/null || true; \
	set +a; \
	$(GO) run ./cmd/api

worker:
	@set -a; \
	. ./.env 2>/dev/null || true; \
	set +a; \
	$(GO) run ./cmd/worker

migrate:
	@if [ -z "$$DATABASE_URL" ]; then \
	echo "DATABASE_URL is required"; \
	exit 1; \
	fi
	$(GOOSE) -dir $(MIGRATION_DIR) postgres "$$DATABASE_URL" up

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
