# ARCHITECTURE.md

## Purpose

本文件记录 AnyPitch 长期稳定边界。代码仍是接口和行为真相源。

## Stable Layers

- 服务入口层：`cmd/server/` 负责启动、读取 `APP_DB_PATH` / `HTTP_ADDR` 并装配 HTTP handler
- SQLite 基础层：`internal/db/` 负责迁移 `users`、`auth_sessions`、`player_sessions`、`teams`、`players`、`events`、`event_locations`、`attendance_records`、`tactic_boards`
- 鉴权层：`internal/auth/` 负责默认教练初始化、环境变量密码、教练密码登录、队员姓名登录、session 创建、Bearer token 校验和登出
- 队员层：`internal/player/` 负责队员 CRUD 和位置标签归一化
- 日程层：`internal/event/` 负责 `training` / `friendly` 日程、开始/结束时间段和常用地点
- 出勤层：`internal/attendance/` 负责事件维度的队员出勤状态
- 战术层：`internal/tactics/` 负责 5/8/11 人制多模板、稳定模板 ID、我方/对手站位 slots 和战术板保存
- HTTP 边界层：`internal/httpapi/` 负责 API envelope、路由、JSON 输入输出和用户隔离
- 前端层：`web/` 负责 React/Vite 响应式工作台和战术板拖拽 UI

## Non-Negotiable Boundaries

- 客户端不直接读写 SQLite
- 公共 API 使用统一 envelope：成功 `{data}`，失败 `{error:{code,message}}`
- 第一版数据隔离以登录教练的默认 team 为边界
- 教练接口必须使用教练 session；队员 session 只能访问 `/api/player/*` 自助接口
- 手机端是响应式 Web，不引入 PWA、service worker 或原生壳
- 新增公共字段或 schema 变更需同步 README、ARCHITECTURE、FRONTEND、相关 product spec 和 CHANGELOG

## Tactics Board Schema

- `TacticTemplate.id` 是稳定模板标识，前端用它处理赛制/模板联动
- `TacticSlot.side` 可为 `home` 或 `opponent`，旧数据缺省时按 `home` 处理
- `tactic_boards` 持久化 `template_id`、`opponent_template_id` 和 `opponent_formation`，用于复原双方站位上下文

## Event Schema

- `events.starts_at` 和 `events.ends_at` 共同表达训练或友谊赛时间段，旧数据缺少结束时间时按开始后两小时兜底
- `events.location` 未填写时使用 `北京邮电大学（海淀校区）`
- `events.notes` 保存训练内容、注意事项或友谊赛备注，允许为空
- `event_locations` 保存当前 team 的常用地点，启动时确保默认地点存在

## Attendance Schema

- 队员自助状态包含 `unknown`（未确认）/ `available`（参加）/ `unavailable`（拒绝）/ `tentative`（待定）
- 队员端只返回全队状态汇总，不暴露其他队员的个人选择明细
- `GET /api/player/events` 在 `events` 外返回当前队员自己的 `attendance_records` 和 `attendance_summary`，其中 `attendance_records` 带 `event_id`，缺失记录按 `unknown` 计入汇总

## Auth Schema

- 默认教练邮箱固定为 `coach@anypitch.local`
- 教练登录只需要提交密码，默认邮箱仅作为内部用户身份
- 默认教练密码优先读取 `ANYPITCH_COACH_PASSWORD`，未设置时使用 `AnyPitch@2026`
- 启动时如果默认教练已存在，也会把密码哈希同步到当前环境变量值
- 队员 session 存在 `player_sessions`，只绑定 `team_id` 和 `player_id`
