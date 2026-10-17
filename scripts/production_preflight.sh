#!/usr/bin/env bash

set -euo pipefail

DB_PATH="${1:-${APP_DB_PATH:-}}"
if [[ -z "${DB_PATH}" ]]; then
  echo "Usage: APP_DB_PATH=/persistent/anypitch.db $0" >&2
  exit 2
fi
if [[ ! -f "${DB_PATH}" ]]; then
  echo "Database not found: ${DB_PATH}" >&2
  exit 2
fi

legacy_count="$(sqlite3 "${DB_PATH}" "SELECT COUNT(*) FROM attendance_records WHERE status NOT IN ('available', 'unavailable', 'tentative', 'unknown');")"
pickup_title_count="$(sqlite3 "${DB_PATH}" "SELECT COUNT(*) FROM events WHERE type = 'training' AND instr(title, '野球') > 0;")"

if [[ "${legacy_count}" != "0" || "${pickup_title_count}" != "0" ]]; then
  echo "Production preflight failed. Do not switch this database yet." >&2
  echo "Legacy RSVP records: ${legacy_count}" >&2
  echo "Training records whose title includes 野球: ${pickup_title_count}" >&2
  echo "Review rows manually; this script never changes production data." >&2
  sqlite3 -header -column "${DB_PATH}" "SELECT e.starts_at AS activity_time, e.title, p.name AS player, a.status, a.note FROM attendance_records a JOIN events e ON e.id = a.event_id JOIN players p ON p.id = a.player_id WHERE a.status NOT IN ('available', 'unavailable', 'tentative', 'unknown') ORDER BY e.starts_at, p.name;" >&2
  sqlite3 -header -column "${DB_PATH}" "SELECT starts_at, title, type FROM events WHERE type = 'training' AND instr(title, '野球') > 0 ORDER BY starts_at;" >&2
  exit 1
fi

echo "Production preflight passed: no legacy RSVP states or title-based pickup candidates."
