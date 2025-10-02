#!/usr/bin/env bash
set -euo pipefail
echo "Starting Postgres via docker-compose..."
docker compose up -d db
echo "Run migrations with goose or your tool of choice."
