#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${ROOT_DIR}/output"

mkdir -p "${OUTPUT_DIR}"

GOCACHE="${ROOT_DIR}/.gocache" GOMODCACHE="${ROOT_DIR}/.gomodcache" go test ./...
npm --prefix "${ROOT_DIR}/web" run test
npm --prefix "${ROOT_DIR}/web" run build
GOCACHE="${ROOT_DIR}/.gocache" GOMODCACHE="${ROOT_DIR}/.gomodcache" go build -o "${OUTPUT_DIR}/anypitch" ./cmd/server
