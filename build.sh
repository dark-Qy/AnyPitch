#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GOCACHE="${ROOT_DIR}/.gocache" GOMODCACHE="${ROOT_DIR}/.gomodcache" go test ./...
npm --prefix "${ROOT_DIR}/web" run test
"${ROOT_DIR}/scripts/package_release.sh"
