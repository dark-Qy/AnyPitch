# AnyPitch

AnyPitch 是一个面向单支足球队的教练工作台。第一版脚手架覆盖教练登录、队员管理、训练/友谊赛日程、出勤登记，以及支持我方/对手站位的 5/8/11 人制战术板保存。

## 目录地图

- `cmd/server/`: Go HTTP 服务入口
- `internal/auth/`: 教练账号、密码和 Bearer session
- `internal/player/`: 队员管理
- `internal/event/`: 训练和友谊赛日程
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
npm --prefix web run dev
```

默认地址：

- 后端：`http://127.0.0.1:8080`
- 前端：`http://127.0.0.1:5173`

默认教练账号：

- 邮箱：`coach@anypitch.local`
- 密码：`AnyPitch@2026`

## 验证

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
npm --prefix web run test
npm --prefix web run build
./scripts/validate_api_smoke.sh
```

## 当前边界

- 第一版是单队 MVP，不做多球队或俱乐部层级
- 第一版只有教练账号，不做队员自助登录或通知
- 手机端只做 Web 响应式适配，不做 PWA、Capacitor 或原生 App
- 战术板支持多赛制、多原始模板、对手站位和拖拽保存，不做跑位动画或复杂比赛统计
