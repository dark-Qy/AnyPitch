# RELIABILITY.md

## Reliability Model

- SQLite 是第一版唯一持久化真相源
- SQLite 元数据与 `ANYPITCH_UPLOAD_DIR` 中的照片文件共同构成相册真相；上传目录必须独立于发布目录并纳入备份
- API handler 启动时执行幂等迁移
- 所有受保护 API 都必须通过 Bearer token 解析当前教练和默认 team
- 失败响应必须保留稳定 `code` 和可读 `message`
- 报名按活动和微信成员局部写入；训练与友谊赛参加报名直接进入统计，取消活动退出统计
- 图片写入使用临时文件后重命名，写库失败会清理新文件；删除允许重复执行清理

## Validation Gate

完成任何功能前至少执行：

- `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...`
- `npm --prefix web run test`
- `npm --prefix web run build`
- `./scripts/validate_api_smoke.sh`

用户可见 UI 改动还需要浏览器检查桌面和手机视口。
