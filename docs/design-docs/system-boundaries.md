# System Boundaries

AnyPitch 第一版是单队教练工作台。服务端拥有业务真相，前端只负责交互呈现。

## Backend

Go 服务在启动时迁移 SQLite schema，并通过 `internal/httpapi` 暴露统一 JSON envelope。每个受保护请求都通过 bearer token 解析教练，再定位默认 team。

## Frontend

React/Vite 工作台使用同一套 Web 代码适配桌面和手机屏幕。战术板使用 dnd-kit 拖拽站位，按赛制过滤模板，并把我方/对手 slots JSON 写回后端。

## Out Of Scope

- 多球队/俱乐部
- 队员自助登录
- 通知和日历订阅
- PWA / 原生 App 壳
- 战术动画和比赛统计
