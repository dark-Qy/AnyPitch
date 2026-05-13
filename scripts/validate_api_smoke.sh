#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${ANYPITCH_SMOKE_PORT:-$((28000 + RANDOM % 10000))}"
BASE_URL="http://127.0.0.1:${PORT}"
SMOKE_DIR="${ROOT_DIR}/data/smoke"
DB_PATH="${SMOKE_DIR}/anypitch-smoke.db"
SERVER_LOG="${SMOKE_DIR}/server.log"
COACH_PASSWORD="${ANYPITCH_COACH_PASSWORD:-AnyPitch@2026}"

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
LOGIN_BODY="$(COACH_PASSWORD="${COACH_PASSWORD}" node -e 'console.log(JSON.stringify({password:process.env.COACH_PASSWORD}))')"
api POST /api/auth/login "" "${LOGIN_BODY}" "${SMOKE_DIR}/login.json"
TOKEN="$(json_get "${SMOKE_DIR}/login.json" data.token)"
[[ -n "${TOKEN}" ]]

echo "==> auth me"
api GET /api/auth/me "${TOKEN}" "" "${SMOKE_DIR}/me.json"
[[ "$(json_get "${SMOKE_DIR}/me.json" data.user.email)" == "coach@anypitch.local" ]]

echo "==> locations"
api GET /api/locations "${TOKEN}" "" "${SMOKE_DIR}/locations.json"
[[ "$(json_get "${SMOKE_DIR}/locations.json" data.locations.0.name)" == "北京邮电大学（海淀校区）" ]]
api POST /api/locations "${TOKEN}" '{"name":"北京邮电大学（沙河校区）"}' "${SMOKE_DIR}/location.json"
LOCATION_ID="$(json_get "${SMOKE_DIR}/location.json" data.location.id)"
[[ -n "${LOCATION_ID}" ]]

echo "==> create player"
api POST /api/players "${TOKEN}" '{"name":"林海","number":10,"positions":["前腰","前锋"]}' "${SMOKE_DIR}/player.json"
PLAYER_ID="$(json_get "${SMOKE_DIR}/player.json" data.player.id)"
[[ -n "${PLAYER_ID}" ]]
[[ "$(json_get "${SMOKE_DIR}/player.json" data.player.positions.0)" == "前腰" ]]

echo "==> create training"
api POST /api/events "${TOKEN}" '{"type":"training","title":"周三控球训练","starts_at":"2026-05-13T20:00:00+08:00","ends_at":"2026-05-13T22:00:00+08:00","location":"北京邮电大学（海淀校区）","opponent":"","notes":"小场压迫"}' "${SMOKE_DIR}/training.json"
EVENT_ID="$(json_get "${SMOKE_DIR}/training.json" data.event.id)"
[[ -n "${EVENT_ID}" ]]
[[ "$(json_get "${SMOKE_DIR}/training.json" data.event.ends_at)" == "2026-05-13T22:00:00+08:00" ]]
api PATCH "/api/events/${EVENT_ID}" "${TOKEN}" '{"starts_at":"2026-05-14T19:30:00+08:00","ends_at":"2026-05-14T21:30:00+08:00","notes":"训练内容：压迫、定位球、防守转换"}' "${SMOKE_DIR}/training-update.json"
[[ "$(json_get "${SMOKE_DIR}/training-update.json" data.event.starts_at)" == "2026-05-14T19:30:00+08:00" ]]
[[ "$(json_get "${SMOKE_DIR}/training-update.json" data.event.ends_at)" == "2026-05-14T21:30:00+08:00" ]]
[[ "$(json_get "${SMOKE_DIR}/training-update.json" data.event.notes)" == "训练内容：压迫、定位球、防守转换" ]]

echo "==> create friendly"
api POST /api/events "${TOKEN}" '{"type":"friendly","title":"周末友谊赛","starts_at":"2026-05-16T18:00:00+08:00","ends_at":"2026-05-16T20:00:00+08:00","location":"北京邮电大学（沙河校区）","opponent":"Blue FC","notes":"八人制"}' "${SMOKE_DIR}/friendly.json"
[[ "$(json_get "${SMOKE_DIR}/friendly.json" data.event.type)" == "friendly" ]]

echo "==> attendance"
api PUT "/api/events/${EVENT_ID}/attendance" "${TOKEN}" "{\"records\":[{\"player_id\":\"${PLAYER_ID}\",\"status\":\"available\",\"note\":\"准时\"}]}" "${SMOKE_DIR}/attendance.json"
[[ "$(json_get "${SMOKE_DIR}/attendance.json" data.records.0.status)" == "available" ]]

echo "==> player self service"
api POST /api/player/login "" '{"name":"林海"}' "${SMOKE_DIR}/player-login.json"
PLAYER_TOKEN="$(json_get "${SMOKE_DIR}/player-login.json" data.token)"
[[ -n "${PLAYER_TOKEN}" ]]
[[ "$(json_get "${SMOKE_DIR}/player-login.json" data.player.name)" == "林海" ]]
api GET /api/player/events "${PLAYER_TOKEN}" "" "${SMOKE_DIR}/player-events.json"
[[ "$(json_get "${SMOKE_DIR}/player-events.json" data.events.0.title)" == "周三控球训练" ]]
[[ "$(json_get "${SMOKE_DIR}/player-events.json" data.attendance_records.0.status)" == "available" ]]
[[ "$(json_get "${SMOKE_DIR}/player-events.json" data.attendance_summary.available)" == "1" ]]
[[ "$(json_get "${SMOKE_DIR}/player-events.json" data.attendance_summary.unknown)" == "1" ]]
api PUT "/api/player/events/${EVENT_ID}/attendance" "${PLAYER_TOKEN}" '{"status":"tentative"}' "${SMOKE_DIR}/player-attendance.json"
[[ "$(json_get "${SMOKE_DIR}/player-attendance.json" data.record.status)" == "tentative" ]]
[[ "$(json_get "${SMOKE_DIR}/player-attendance.json" data.summary.tentative)" == "1" ]]
api GET /api/player/events "${PLAYER_TOKEN}" "" "${SMOKE_DIR}/player-events-updated.json"
[[ "$(json_get "${SMOKE_DIR}/player-events-updated.json" data.attendance_records.0.status)" == "tentative" ]]
[[ "$(json_get "${SMOKE_DIR}/player-events-updated.json" data.attendance_summary.tentative)" == "1" ]]

echo "==> tactic templates"
api GET /api/tactics/templates "${TOKEN}" "" "${SMOKE_DIR}/templates.json"
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.0.id)" == "f5-121-press" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.0.format)" == "5" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.0.slots.0.label)" == "门将" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.1.format)" == "5" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.2.format)" == "8" ]]
[[ "$(json_get "${SMOKE_DIR}/templates.json" data.templates.4.format)" == "11" ]]

echo "==> tactic board"
api POST /api/tactics/boards "${TOKEN}" "{\"name\":\"五人制高位压迫\",\"template_id\":\"f5-121-press\",\"opponent_template_id\":\"f5-211-counter\",\"format\":5,\"formation\":\"1-2-1\",\"opponent_formation\":\"2-1-1\",\"slots\":[{\"slot_id\":\"home:gk\",\"label\":\"门将\",\"x\":50,\"y\":91,\"player_id\":\"${PLAYER_ID}\"},{\"slot_id\":\"opponent:gk\",\"label\":\"门将\",\"side\":\"opponent\",\"x\":50,\"y\":9,\"player_id\":\"\"}]}" "${SMOKE_DIR}/board.json"
[[ "$(json_get "${SMOKE_DIR}/board.json" data.board.format)" == "5" ]]
[[ "$(json_get "${SMOKE_DIR}/board.json" data.board.template_id)" == "f5-121-press" ]]
[[ "$(json_get "${SMOKE_DIR}/board.json" data.board.slots.1.side)" == "opponent" ]]

echo "==> logout"
api POST /api/auth/logout "${TOKEN}" "" "${SMOKE_DIR}/logout.json"
[[ "$(json_get "${SMOKE_DIR}/logout.json" data.ok)" == "true" ]]

echo "AnyPitch API smoke validation passed"
