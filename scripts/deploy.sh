#!/bin/sh

set -eu

APP_DIR=${APP_DIR:-/home/alexey/home-go}
COMPOSE_SERVICE=${COMPOSE_SERVICE:-home-go}
LOG_TAG=${LOG_TAG:-home-go-deploy}

log() {
  if command -v logger >/dev/null 2>&1; then
    logger -t "$LOG_TAG" "$1"
  fi
  printf '%s %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"
}

cd "$APP_DIR"

if [ ! -f .env ]; then
  log "missing $APP_DIR/.env"
  exit 1
fi

current_tag=$(sed -n 's/^IMAGE_TAG=//p' .env | tail -n 1)
if [ -z "$current_tag" ]; then
  log "IMAGE_TAG is not set in $APP_DIR/.env"
  exit 1
fi

log "reconciling service $COMPOSE_SERVICE to tag $current_tag"
docker compose pull "$COMPOSE_SERVICE"
docker compose up -d "$COMPOSE_SERVICE"
log "deployment check completed for tag $current_tag"
