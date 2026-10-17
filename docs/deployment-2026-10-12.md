# 2026-10-12 小程序 0.2.17 交付记录

## 当前状态

- **开发版上传成功**：官方微信开发者工具 CLI 上传 `0.2.17`，描述为统一视觉、宋体标题、分类图标、报名头像与窄屏弹层，返回 `✔ upload`、exit 0；包体 273,808 bytes。回执 `output/miniprogram-upload-v0217.json`。
- **字体资源已上线**：自有 HTTPS 返回 200、`font/woff`、CORS `*`、3,968,644 bytes；OFL 许可证保留。网页浏览器实际字体加载成功，微信开发者工具冷启动 `globalFontLoaded: true`，证据 `output/miniprogram-v0217-font-check.json` 和 `docs/verification/2026-10-12-visual/font-https-live.png`。
- **尚未正式公开发布**：浏览器工具的 site-safety policy 明确阻止访问微信后台版本管理页面；没有绕过限制、操作审核或发布，也无法核对后台当前审核状态。用户需在微信后台继续完成 0.2.17 的审核／发布流程。
- iOS／Android 真机字体、SVG 和安全区仍未验证。

## 字体部署

只新增字体静态资源，业务服务继续运行 `/srv/anypitch/releases/20261011-v0216`；未切换 Go 程序、网页、数据库、上传目录、成员权限或业务配置。

- 文件：`/srv/anypitch/static/miniprogram/fonts/source-han-serif-medium.woff`、同目录 `OFL.txt`。
- Nginx：`/etc/nginx/sites-available/anypitch` 新增两个精确静态路径；只有 WOFF 设置 `font/woff` 与 `Access-Control-Allow-Origin: *`，许可证为普通文本，不扩大业务 API 跨域权限。
- 配置备份：`/srv/anypitch/backups/20261012-v0217-font/anypitch.nginx.conf`。`nginx -t` 通过，平滑 reload 后 HTTPS 字体成功，业务健康检查仍为 200，API 响应未增加 CORS。
- 回退仅涉及本次 Nginx 字体路径与新增静态文件，不需要回退业务程序或数据。

## 加载修正与验证

字体在 `App.onLaunch` 阶段加载时，入口立即跳转后初始化未完成；已改为页面 `onReady` 调用共享 `App.initTitleFont()`，成功后后续页面跳过重复初始化，保持全局字体。省去固定 Medium 字体不需要的可选描述参数。现已确认冷启动全局加载成功，成功／失败状态有自动化测试。

`node --test scripts/miniprogram/*.test.cjs`：33 / 33 通过。Go 测试、Web 25 项测试与构建、隔离 API smoke、全部页面及 320／390／430px 视觉验证见 `docs/verification/2026-10-12-visual/README.md`。开发版上传与开发者工具验证不等同于微信审核通过、真机通过或正式发布。

## 手机预览

官方 CLI 预览生成成功：`output/miniprogram-v0217-preview.png`，预览包 273,373 bytes，回执 `output/miniprogram-v0217-preview.json`。此二维码用于手机预览，不代表已设置体验版、审核通过或正式发布。
