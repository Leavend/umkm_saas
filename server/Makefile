GOOSE ?= goose
MIGRATION_DIR := internal/adapter/repo/migrations

.PHONY: migrate
migrate:
@if [ -z "$${DATABASE_URL}" ]; then \
echo "DATABASE_URL is required"; \
exit 1; \
fi
$(GOOSE) -dir $(MIGRATION_DIR) postgres "$${DATABASE_URL}" up
