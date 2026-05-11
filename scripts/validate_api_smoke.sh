#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${ANYPITCH_SMOKE_PORT:-$((28000 + RANDOM % 10000))}"
BASE_URL="http://127.0.0.1:${PORT}"
SMOKE_DIR="${ROOT_DIR}/data/smoke"
DB_PATH="${SMOKE_DIR}/anypitch-smoke.db"
SERVER_LOG="${SMOKE_DIR}/server.log"

rm -rf "${SMOKE_DIR}"
mkdir -p "${SMOKE_DIR}"

APP_DB_PATH="${DB_PATH}" \
HTTP_ADDR="127.0.0.1:${PORT}" \
GOCACHE="${ROOT_DIR}/.gocache" \
GOMODCACHE="${ROOT_DIR}/.gomodcache" \
go run ./cmd/server >"${SERVER_LOG}" 2>&1 &
SERVER_PID=$!

cleanup() {
  kill "${SERVER_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

for _ in $(seq 1 50); do
  if ! kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
    echo "server exited before health check" >&2
    cat "${SERVER_LOG}" >&2 || true
    exit 1
  fi
  if curl -sS "${BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

json_get() {
  local file="$1"
  local path="$2"
  node -e '
const fs = require("fs");
const data = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
let current = data;
for (const part of process.argv[2].split(".")) {
  current = Array.isArray(current) ? current[Number(part)] : current[part];
}
if (typeof current === "object") {
  console.log(JSON.stringify(current));
} else {
  console.log(current ?? "");
}
' "${file}" "${path}"
}

api() {
  local method="$1"
  local path="$2"
  local token="${3:-}"
  local body="${4:-}"
  local output="$5"
  if [[ -n "${body}" ]]; then
    curl -sS -X "${method}" "${BASE_URL}${path}" \
      -H "Content-Type: application/json" \
      ${token:+-H "Authorization: Bearer ${token}"} \
      --data "${body}" >"${output}"
  else
    curl -sS -X "${method}" "${BASE_URL}${path}" \
      -H "Content-Type: application/json" \
      ${token:+-H "Authorization: Bearer ${token}"} >"${output}"
  fi
}

echo "==> health"
api GET /api/healthz "" "" "${SMOKE_DIR}/health.json"
[[ "$(json_get "${SMOKE_DIR}/health.json" data.ok)" == "true" ]]

echo "==> login"
api POST /api/auth/login "" '{"email":"coach@anypitch.local","password":"AnyPitch@2026"}' "${SMOKE_DIR}/login.json"
TOKEN="$(json_get "${SMOKE_DIR}/login.json" data.token)"
[[ -n "${TOKEN}" ]]

echo "==> auth me"
api GET /api/auth/me "${TOKEN}" "" "${SMOKE_DIR}/me.json"
[[ "$(json_get "${SMOKE_DIR}/me.json" data.user.email)" == "coach@anypitch.local" ]]

echo "==> create player"
api POST /api/players "${TOKEN}" '{"name":"林海","number":10,"positions":["前腰","前锋"]}' "${SMOKE_DIR}/player.json"
PLAYER_ID="$(json_get "${SMOKE_DIR}/player.json" data.player.id)"
[[ -n "${PLAYER_ID}" ]]
[[ "$(json_get "${SMOKE_DIR}/player.json" data.player.positions.0)" == "前腰" ]]

echo "==> create training"
api POST /api/events "${TOKEN}" '{"type":"training","title":"周三控球训练","starts_at":"2026-05-13T20:00:00+08:00","location":"东区球场","opponent":"","notes":"小场压迫"}' "${SMOKE_DIR}/training.json"
EVENT_ID="$(json_get "${SMOKE_DIR}/training.json" data.event.id)"
[[ -n "${EVENT_ID}" ]]

echo "==> create friendly"
api POST /api/events "${TOKEN}" '{"type":"friendly","title":"周末友谊赛","starts_at":"2026-05-16T18:00:00+08:00","location":"西区球场","opponent":"Blue FC","notes":"八人制"}' "${SMOKE_DIR}/friendly.json"
[[ "$(json_get "${SMOKE_DIR}/friendly.json" data.event.type)" == "friendly" ]]

echo "==> attendance"
api PUT "/api/events/${EVENT_ID}/attendance" "${TOKEN}" "{\"records\":[{\"player_id\":\"${PLAYER_ID}\",\"status\":\"available\",\"note\":\"准时\"}]}" "${SMOKE_DIR}/attendance.json"
[[ "$(json_get "${SMOKE_DIR}/attendance.json" data.records.0.status)" == "available" ]]

echo "==> tactic templates"
api GET /api/tactics/templates "${TOKEN}" "" "${SMOKE_DIR}/templates.json"
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.0.format)" == "5" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.0.slots.0.label)" == "门将" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.1.format)" == "8" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.2.format)" == "11" ]]

echo "==> tactic board"
api POST /api/tactics/boards "${TOKEN}" "{\"name\":\"五人制高位压迫\",\"format\":5,\"formation\":\"1-2-1\",\"slots\":[{\"slot_id\":\"gk\",\"label\":\"门将\",\"x\":50,\"y\":91,\"player_id\":\"${PLAYER_ID}\"}]}" "${SMOKE_DIR}/board.json"
[[ "$(json_get "${SMOKE_DIR}/board.json" data.board.format)" == "5" ]]

echo "==> logout"
api POST /api/auth/logout "${TOKEN}" "" "${SMOKE_DIR}/logout.json"
[[ "$(json_get "${SMOKE_DIR}/logout.json" data.ok)" == "true" ]]

echo "AnyPitch API smoke validation passed"
