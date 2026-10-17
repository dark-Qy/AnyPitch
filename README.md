# AnyPitch

AnyPitch 是西土城 FC 的单队训练组织网页。教练在一处管理活动、报名、训练榜、校长杯、相册和战术板；队员在手机或电脑上确认参与、查看自己的训练记录和队内照片。

## 当前功能

- 日常活动分为训练、友谊赛和野球；校长杯是独立专题，不伪装成活动类型。
- 报名只有参加、不参加、待定、未确认四种状态。每位队员的一场报名独立自动保存。
- 训练与友谊赛报名参加即计数；无需教练核对实到。取消活动不计，训练榜仅展示微信成员。
- 月榜、年度、上半年和下半年按北京时间自然周期，学期榜由教练设置起止日期；队员可看榜单和本人逐场历史，教练可看全队历史并导出 CSV。
- 校长杯可维护届次、安排、赛果和专题相册，不产生训练出勤。
- 网页管理员可为活动或专题建立队内相册并上传 JPEG、PNG、WebP；成员网页端只读，小程序不展示相册。照片私有鉴权读取。
- 保留 5/8/11 人制、双方模板和拖拽保存的战术板。

## 目录

- `cmd/server/`：Go HTTP 服务入口
- `internal/attendance/`：四状态报名与取消规则
- `internal/event/`：训练、友谊赛、野球和取消状态
- `internal/stats/`：学期、训练榜、历史和 CSV
- `internal/specialactivity/`：校长杯专题与安排
- `internal/media/`：相册元数据、图片压缩、私有读取和清理
- `internal/httpapi/`：鉴权、路由和统一 JSON envelope
- `web/`：React + Vite 响应式网页
- `scripts/validate_api_smoke.sh`：隔离数据库和上传目录下的 API 主路径验证
- `scripts/package_release.sh`：生成不包含业务数据的发布目录
- `scripts/production_preflight.sh`：只读阻断未处理旧报名状态的生产切换

## 本地运行

先安装前端依赖并构建静态页面：

```bash
npm --prefix web ci
npm --prefix web run build
```

再启动服务：

```bash
APP_DB_PATH="$(pwd)/data/anypitch.db" \
ANYPITCH_UPLOAD_DIR="$(pwd)/data/uploads" \
HTTP_ADDR="127.0.0.1:8080" \
GOCACHE="$(pwd)/.gocache" GOMODCACHE="$(pwd)/.gomodcache" \
go run ./cmd/server
```

访问 `http://127.0.0.1:8080`。默认教练密码为 `AnyPitch@2026`；部署时请设置 `ANYPITCH_COACH_PASSWORD`。

开发前端时可以额外运行：

```bash
npm --prefix web run dev
```

## 验证与发布准备

```bash
GOCACHE="$(pwd)/.gocache" GOMODCACHE="$(pwd)/.gomodcache" go test ./...
npm --prefix web run test
npm --prefix web run build
./scripts/validate_api_smoke.sh
./scripts/package_release.sh
```

发布目录和上线、备份、回滚步骤见 [部署说明](docs/deployment.md)。发布包不包含 SQLite 数据库和照片；两者必须放在独立持久化目录。

## 已知边界

- 当前包含单队网页及微信小程序队员端源码；小程序提供活动查看与分享、固定报名、训练榜、个人记录、入队分享和个人头像上传，不包含 PWA 或原生 App。
- 队员通过微信 code2Session 验证身份；首次选择昵称，角色由服务器维护。
- 线上旧的九状态报名记录不会自动迁移。生产切换前必须人工清点非四状态记录，并根据原始备注逐条处理。

## 微信小程序

使用微信开发者工具导入 `miniprogram/`。接入、验证和发布说明见 [小程序说明](miniprogram/README.md)。源代码准备完成不等于体验版上传或审核发布完成。
