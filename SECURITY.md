# SECURITY.md

## Security Boundary

- 密码只保存 bcrypt hash
- Session token 使用随机字节生成，API 只接受 `Authorization: Bearer <token>`
- 前端只在 `localStorage["anypitch_token"]` 保存 bearer token
- SQLite 数据按登录教练的默认 team 隔离
- 不在日志或响应中输出密码、password hash 或 token

## Current Limitations

- 第一版没有细粒度角色权限
- 第一版没有队员自助登录
- 第一版适合本地或内网 MVP，不声明生产级多租户安全完备性
