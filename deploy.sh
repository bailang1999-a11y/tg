#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$ROOT_DIR/deploy/.env.production"
ENV_TEMPLATE="$ROOT_DIR/deploy/.env.production.example"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.prod.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed or not in PATH." >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is not available. Install Docker Desktop or the docker compose plugin." >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  cp "$ENV_TEMPLATE" "$ENV_FILE"
  echo "Created deploy/.env.production from template."
  echo "Edit deploy/.env.production and replace every replace-with-* value, then run ./deploy.sh again."
  exit 1
fi

if grep -q "replace-with-" "$ENV_FILE"; then
  echo "deploy/.env.production still contains replace-with-* placeholders. Replace them before deploying." >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
