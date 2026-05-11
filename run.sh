#!/usr/bin/env bash

set -euo pipefail

APP_DB_PATH="${APP_DB_PATH:-$(pwd)/data/anypitch.db}" \
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1:8080}" \
GOCACHE="$(pwd)/.gocache" \
GOMODCACHE="$(pwd)/.gomodcache" \
go run ./cmd/server
