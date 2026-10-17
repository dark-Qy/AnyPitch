#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${ROOT_DIR}/output/anypitch-release"

rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}/web"

npm --prefix "${ROOT_DIR}/web" run build
GOCACHE="${ROOT_DIR}/.gocache" GOMODCACHE="${ROOT_DIR}/.gomodcache" \
  go build -o "${RELEASE_DIR}/anypitch" "${ROOT_DIR}/cmd/server"
cp -R "${ROOT_DIR}/web/dist" "${RELEASE_DIR}/web/dist"
cp "${ROOT_DIR}/scripts/production_preflight.sh" "${RELEASE_DIR}/production_preflight.sh"
cp "${ROOT_DIR}/docs/deployment.md" "${RELEASE_DIR}/DEPLOYMENT.md"

cat >"${RELEASE_DIR}/.env.example" <<'EOF'
APP_DB_PATH=/srv/anypitch/data/anypitch.db
ANYPITCH_UPLOAD_DIR=/srv/anypitch/data/uploads
ANYPITCH_COACH_PASSWORD=replace-with-a-private-password
HTTP_ADDR=127.0.0.1:8080
EOF

cat >"${RELEASE_DIR}/RELEASE.md" <<'EOF'
# AnyPitch 发布目录

从此目录启动 `./anypitch`，以便服务找到 `web/dist`。

数据库和上传目录由 `APP_DB_PATH`、`ANYPITCH_UPLOAD_DIR` 指向发布目录外的持久化路径；不要把它们放进本目录或纳入代码发布包。

完整切换、备份和回滚步骤见同目录的 `DEPLOYMENT.md`。
EOF

printf 'Release directory ready: %s\n' "${RELEASE_DIR}"
