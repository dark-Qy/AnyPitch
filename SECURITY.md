# SECURITY.md

## Security Boundary

- 密码只保存 bcrypt hash
- Session token 使用随机字节生成，API 只接受 `Authorization: Bearer <token>`
- 前端只在 `localStorage["anypitch_token"]` / `localStorage["anypitch_player_token"]` 保存 bearer token
- SQLite 数据按登录教练的默认 team 隔离
- 队员端只展示出勤汇总，不返回其他队员个人出勤明细
- 不在日志或响应中输出密码、password hash 或 token
- 当前默认教练账号仅用于本地/内网 MVP，生产化前必须替换为正式账号初始化策略

## Current Limitations

- 第一版没有细粒度角色权限
- 第一版队员入口按姓名免密，生产化前需要更强身份校验
- 第一版没有改密、找回密码或用户管理
- 第一版适合本地或内网 MVP，不声明生产级多租户安全完备性
