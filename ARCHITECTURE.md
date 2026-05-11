# ARCHITECTURE.md

## Purpose

本文件记录 AnyPitch 长期稳定边界。代码仍是接口和行为真相源。

## Stable Layers

- 服务入口层：`cmd/server/` 负责启动、读取 `APP_DB_PATH` / `HTTP_ADDR` 并装配 HTTP handler
- SQLite 基础层：`internal/db/` 负责迁移 `users`、`auth_sessions`、`teams`、`players`、`events`、`attendance_records`、`tactic_boards`
- 鉴权层：`internal/auth/` 负责教练注册、登录、session 创建、Bearer token 校验和登出
- 队员层：`internal/player/` 负责队员 CRUD 和位置标签归一化
- 日程层：`internal/event/` 负责 `training` / `friendly` 日程
- 出勤层：`internal/attendance/` 负责事件维度的队员出勤状态
- 战术层：`internal/tactics/` 负责 5/8/11 人制模板、站位 slots 和战术板保存
- HTTP 边界层：`internal/httpapi/` 负责 API envelope、路由、JSON 输入输出和用户隔离
- 前端层：`web/` 负责 React/Vite 响应式工作台和战术板拖拽 UI

## Non-Negotiable Boundaries

- 客户端不直接读写 SQLite
- 公共 API 使用统一 envelope：成功 `{data}`，失败 `{error:{code,message}}`
- 第一版数据隔离以登录教练的默认 team 为边界
- 手机端是响应式 Web，不引入 PWA、service worker 或原生壳
- 新增公共字段或 schema 变更需同步 README、ARCHITECTURE、FRONTEND、相关 product spec 和 CHANGELOG
