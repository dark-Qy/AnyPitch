# 小程序标题字体

`source-han-serif-medium.woff` 是思源宋体简体中文 Medium（Source Han Serif SC Medium）的子集，覆盖 GB2312 常用简体中文、ASCII、数字、标点与小程序源码中的文案，共 7546 个映射字符。保留 `OFL.txt`；字体采用 SIL Open Font License 1.1。子集修改后内部字体名为 `AnyPitch Serif`，避免使用上游保留名称 Source。

源文件来自 Adobe 官方仓库 `adobe-fonts/source-han-serif` 的 `release/OTF/SimplifiedChinese/SourceHanSerifSC-Medium.otf`。XTC FC 字标由 Google Fonts `ofl/barlowcondensed/BarlowCondensed-BoldItalic.ttf` 转换为路径，其许可证在小程序 `assets/OFL-barlow-condensed.txt`。

本地复现：安装 `fonttools[woff]`，下载上述官方原字体后执行：

```sh
python scripts/build_miniprogram_fonts.py /path/SourceHanSerifSC-Medium.otf /path/BarlowCondensed-BoldItalic.ttf
npm --prefix web run build
```

Web 构建将字体复制到静态发布目录。Go 只为这个 WOFF 路径提供 `font/woff` 和 `Access-Control-Allow-Origin: *`，业务 API 与其他静态资源的跨域权限不变。

小程序启动通过 `wx.loadFontFace` 全局加载 `https://pitch.coolcookie.cloud/assets/miniprogram/fonts/source-han-serif-medium.woff`。2026-10-12 已部署此 HTTPS 资源，浏览器与微信开发者工具全局加载通过；页面就绪时调用初始化。iOS/Android 真机标题显示仍需验证。字体失败时保留可读的系统宋体/serif。
