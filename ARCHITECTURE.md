# ARCHITECTURE.md

代码是 AnyPitch 的行为真相。本文件记录当前稳定边界，避免前端复制业务判断或把专题和训练报名混在一起。

## 分层

- `cmd/server/`：读取 `APP_DB_PATH`、`ANYPITCH_UPLOAD_DIR`、`HTTP_ADDR`，启动 HTTP 服务。
- `internal/db/`：在实际 SQLite 连接上启用外键并创建当前 schema。
- `internal/auth/`：教练密码、微信成员注册、独立 session 和 Bearer token 校验。
- `internal/player/`：队员 CRUD、停用与引用删除限制。
- `internal/event/`：`training`、`friendly`、`pickup` 活动和可恢复的取消记录。
- `internal/attendance/`：四状态报名和逐场局部更新。
- `internal/stats/`：学期、训练榜、历史和 CSV。
- `internal/specialactivity/`：校长杯等独立短期专题及其条目。
- `internal/media/`：相册元数据、图片解码压缩、私有文件读取和删除清理。
- `internal/tactics/`：5/8/11 人制模板和双方拖拽站位。
- `internal/httpapi/`：JSON envelope、角色路由、team 授权和静态前端服务。
- `web/`：React/Vite 的响应式交互和展示层。
- `miniprogram/`：微信原生队员端，通过同一 Go API 查看活动与专题、提交固定状态报名、查看记录和训练榜；不复制统计业务。

## 业务真相

### 报名与训练榜

- `attendance_records` 只表达 `available`、`unavailable`、`tentative`、`unknown` 报名和备注。
- 教练和队员都按「活动 × 队员」局部写入；前端不能整表覆盖其他人的报名。
- 取消活动保留报名和相册，但停止报名并退出统计；有引用的活动使用取消而非硬删除。

### 统计与专题

- 月榜按 `Asia/Shanghai` 的活动开始时间归属；学期以教练维护的起止日期计算，不能重叠。
- 训练榜只统计微信成员在未取消的 `training` 与 `friendly` 活动的 `available` 报名次数。野球和校长杯不计入。
- 校长杯存于 `special_activities` 和 `special_activity_entries`，不绑定日常活动，不伪造出勤。

### 相册

- 一个相册只属于一场活动或一届专题；SQLite 仅保存元数据，文件在 `ANYPITCH_UPLOAD_DIR`。
- 服务端只接受 JPEG/PNG/WebP，每请求最大 6MB、最多 2500 万像素，重编码 JPEG 并生成缩略图；单队总空间初始上限 5GB。
- 图片通过需 Bearer token 的 API 读取。现有成员相册内容在网页端只读；仅网页管理员会话可建立相册、上传、修改或删除照片。
- 删除先标记不可见，再清理文件；遇到文件清理失败可再次删除同一照片继续清理。

## API 和授权

- 成功响应为 `{data}`，错误为 `{error:{code,message}}`。
- 网页管理员接口位于 `/api/*`，微信成员自助接口位于 `/api/player/*`；成员可维护自己的昵称、头像和固定报名状态，活动创建及相册写入由服务端拒绝。头像上传经图片校验和异步审核后才替换原头像。
- `/api/stats/history.csv` 只对教练开放，且 CSV 字段会处理可能被 Excel 作为公式的开头字符。
- 静态前端从服务进程工作目录下的 `web/dist` 提供。发布目录必须保留该相对路径。

## 不在范围内

- 多球队或俱乐部管理、赛事淘汰树、积分系统和日历订阅。新活动一次性订阅提醒由独立发送任务处理。
- PWA、原生壳和视频/HEIC 转码服务。
- 旧九状态报名的自动映射或自动生成历史实到。
