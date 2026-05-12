# System Boundaries

AnyPitch 第一版是单队教练工作台。服务端拥有业务真相，前端只负责交互呈现。

## Backend

Go 服务在启动时迁移 SQLite schema，并通过 `internal/httpapi` 暴露统一 JSON envelope。教练受保护请求通过 bearer token 解析教练，再定位默认 team；队员受保护请求通过单独的 player session 定位 team 和 player。

## Frontend

React/Vite 工作台使用同一套 Web 代码适配桌面和手机屏幕。入口分为教练和队员：教练拥有完整的西土城FC工作台；队员只看到日程表、自己的全部日程状态汇总、默认折叠的已结束日程列表和自己的参加状态。日程月历支持点击具体日程进入详情并直接维护备注与出勤，队员端月历用标题、时间和颜色展示个人参加情况，地点通过下拉和轻量管理控件维护。战术板使用 dnd-kit 拖拽站位，按赛制过滤模板，并把我方/对手 slots JSON 写回后端。

## Out Of Scope

- 多球队/俱乐部
- 通知和日历订阅
- PWA / 原生 App 壳
- 战术动画和比赛统计
