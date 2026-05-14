# AnyPitch

AnyPitch 是一个面向西土城FC的教练工作台。第一版脚手架覆盖教练登录、队员免密入口、队员管理、训练/友谊赛日程、日程调整与删除、日程内出勤登记、地点管理，以及支持我方/对手站位的 5/8/11 人制战术板保存。

## 目录地图

- `cmd/server/`: Go HTTP 服务入口
- `internal/auth/`: 教练账号、队员姓名入口和 Bearer session
- `internal/player/`: 队员管理
- `internal/event/`: 训练和友谊赛日程、备注、默认地点和地点管理
- `internal/attendance/`: 出勤状态登记
- `internal/tactics/`: 5/8/11 人制多模板、我方/对手 slots 和战术板保存
- `internal/httpapi/`: API envelope、鉴权中间件和路由
- `web/`: React + Vite + TypeScript 响应式 Web 工作台
- `scripts/validate_api_smoke.sh`: 本地 API 主路径 smoke 验证
- `rules/`: 协作规则和验证门禁

## 本地运行

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go run ./cmd/server
```

```bash
npm --prefix web install
npm run dev
```

默认地址：

- 后端：`http://127.0.0.1:8080`
- 前端：`http://127.0.0.1:5173`

默认教练登录：

- 密码：默认 `AnyPitch@2026`，可通过 `ANYPITCH_COACH_PASSWORD` 环境变量覆盖

```bash
ANYPITCH_COACH_PASSWORD='你的教练密码' GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go run ./cmd/server
```

## 验证

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
npm run test
npm run build
./scripts/validate_api_smoke.sh
```

## 当前边界

- 第一版是单队 MVP，不做多球队或俱乐部层级
- 教练必须登录；队员入口只允许输入已存在队员姓名，不需要密码
- 队员只能查看日程、查看参加/拒绝/待定/未确认汇总，并维护自己的参加状态；队员端会展示自己对全部日程的总状况，已结束日程在列表中默认折叠，不能管理队员、地点、战术板或全队出勤
- 日程时间使用开始/结束时间段，可填写训练内容等备注；地点默认 `北京邮电大学（海淀校区）`，并支持在 Web 里维护常用地点、调整已有日程时间和删除已有日程
- 手机端只做 Web 响应式适配，不做 PWA、Capacitor 或原生 App
- 战术板支持多赛制、多原始模板、对手站位和拖拽保存，不做跑位动画或复杂比赛统计
