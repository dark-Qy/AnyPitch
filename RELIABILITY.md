# RELIABILITY.md

## Reliability Model

- SQLite 是第一版唯一持久化真相源
- API handler 启动时执行幂等迁移
- 所有受保护 API 都必须通过 Bearer token 解析当前教练和默认 team
- 失败响应必须保留稳定 `code` 和可读 `message`

## Validation Gate

完成任何功能前至少执行：

- `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...`
- `npm --prefix web run test`
- `npm --prefix web run build`
- `./scripts/validate_api_smoke.sh`

用户可见 UI 改动还需要浏览器检查桌面和手机视口。
