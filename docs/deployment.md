# 部署说明

部署前先在服务器只读确认现有站点对应的进程、架构、反向代理、监听端口、HTTPS、数据库、上传目录、磁盘余量和备份方式；不要从控制台里存在一台机器推断这些细节。2026-09-22 的现场检查和预发布验证见 [部署记录](deployment-2026-09-22.md)。

## 发布目录

运行 `./scripts/package_release.sh` 后会生成 `output/anypitch-release/`：

```text
anypitch-release/
  anypitch
  web/dist/
  .env.example
  RELEASE.md
```

服务以当前工作目录下的 `web/dist` 提供页面，因此运行二进制时工作目录必须是该发布目录。数据库和照片不能放入这个目录，也不能跟着版本目录替换。

```bash
cd /srv/anypitch/releases/2026-09-15
set -a
. /etc/anypitch/anypitch.env
set +a
./anypitch
```

推荐的 `/etc/anypitch/anypitch.env`：

```bash
APP_DB_PATH=/srv/anypitch/data/anypitch.db
ANYPITCH_UPLOAD_DIR=/srv/anypitch/data/uploads
ANYPITCH_COACH_PASSWORD=replace-with-a-private-password
HTTP_ADDR=127.0.0.1:8080
```

反向代理、systemd unit 名称、目标 CPU 架构和实际端口必须在服务器核实后补入，不能猜测。

## 上线前数据检查

当前发布不迁移旧报名状态。生产切换前以只读方式列出 `attendance_records` 中的状态、活动、队员和备注：

```bash
APP_DB_PATH=/srv/anypitch/data/anypitch.db ./scripts/production_preflight.sh
```

- `available`、`unavailable`、`tentative`、`unknown` 可保留为新报名。
- `late`、`injured`、`present`、`absent`、`excused` 等旧值不可自动映射；教练须根据原始情况逐条更正。
- 标题包含“野球”但类型为 `training` 的活动，须由教练在网页编辑流程中确认，不自动改类型。
- 没有可审计实到证据的旧活动保持未核对，不伪造训练榜成绩。

未处理旧枚举前不得切换生产页面到四状态报名逻辑。

## 一致备份与恢复演练

停止写入后，再备份 SQLite 和上传目录。不要在热写入过程中直接复制单个 SQLite 文件当作一致备份。

1. 暂停服务或反向代理写入口。
2. 用 SQLite 的一致备份方式生成数据库副本，例如 `sqlite3 /srv/anypitch/data/anypitch.db ".backup '/backup/anypitch/anypitch.db'"`。
3. 使用保留权限与时间戳的工具备份 `/srv/anypitch/data/uploads/` 到同一备份批次目录。
4. 在隔离目录恢复数据库和上传文件，启动同版本发布目录；以教练和队员账号验证历史、缩略图和原图读取。
5. 记录恢复所用备份批次和验证结果后才恢复写入。

上传目录初始配额为每队 5GB，仍须根据真实磁盘余量保留系统空间，并在上线前设置反向代理的请求体大小和上传超时与服务端 6MB 单图限制一致。

## 切换与回滚

切换顺序：暂停写入并完成一致备份 → 人工处理旧状态 → 上传新发布目录 → 配置环境变量 → 重启服务 → `/healthz`、教练只读、队员只读检查 → 授权 smoke → 恢复使用。

回滚不能只换旧二进制：数据库 schema、照片文件和代码必须作为一个经过恢复验证的组合处理。若新增内容已经写入且没有演练过相应数据恢复，不得直接回退到旧版本。

## 现有限制

- 需补真实 iOS/Android 选图和 HEIC 行为验证；浏览器模拟宽度不能替代真机。
- 队员姓名免密入口不等同于强身份认证；部署说明中的“队内可见”只指现有 session 与 team 授权。

## 微信内容安全回调

小程序 0.2.16 需设置 `ANYPITCH_PUBLIC_URL=https://pitch.coolcookie.cloud`、`ANYPITCH_WECHAT_CALLBACK_TOKEN`（3–32 位字母数字，服务器环境文件权限600）。后台消息推送 URL为 `/api/wechat/content-callback`，配置明文模式及JSON。先部署能够响应签名挑战的服务端，再提交回调配置。必须验证真实微信图片检查请求和回调后才认定内容安全接入生效。待审图片存放上传目录的 `pending/`，随上传目录一起备份；未通过审核不会公开。

### 2026-10-11 验证记录

- 服务端已切换到 `/srv/anypitch/releases/20261011-v0216`，一致备份保存在 `/srv/anypitch/backups/20261011-v0216`。公开健康检查成功。
- 微信后台消息推送已启用。真实文字检查通过；测试图片提交后收到真实微信审核回调，审核通过后才进入相册。
- 小程序相册删除入口已验证：模拟确认框的取消和确认选择，执行真实删除请求，仅删除本次新增的测试照片，原有照片保留。普通队员仅能删除自己的照片，教练及管理员可删除队内照片；权限由服务端校验。
- `go test ./...`、小程序23项测试、Web25项测试、Web构建及隔离 smoke 均通过。Web浏览器验证及小程序开发者工具相册界面验证已执行；真实 iOS/Android 选图尚未验证。
- 开发者工具刷新域名并重启后，原生 `wx.uploadFile` 返回202待审核；真实微信回调通过后照片展示。复测取消删除保留照片、确认删除成功，删除入口点击区域45×45，原有两张照片保留。
- 0.2.16 开发版本上传成功（微信后台时间18:16:39），19:31:23重新提交审核，后台已确认“审核中”。尚未正式发布。

## 2026-10-12 字体与小程序开发版

0.2.17 已通过官方 CLI 上传开发版，字体使用自有 HTTPS 精确静态路径；后端继续运行 20261011-v0216，未更改业务数据。发布详情及后台访问限制见 [交付记录](deployment-2026-10-12.md)，正式审核／公开发布仍未完成。

## 2026-10-16 个人主体审核范围调整

网页管理端发布、小程序只读活动与固定状态报名的 0.2.18 开发版已上传；生产服务端已切换到 `20261016-v0218` 并完成一致备份。小程序相册及发布入口已移除，微信成员会话的自由内容写入由服务端拒绝。验证与未验证状态见 [本次记录](verification/2026-10-16-review-scope/README.md)。尚未重新送审或公开发布。

## 2026-10-17 分享与头像开发版

小程序 0.2.19 已上传开发版，配套服务端已切换到 `20261017-v0219`；微信后台审核和公开发布均未完成。见 [交付状态](deployment-2026-10-17.md)。
