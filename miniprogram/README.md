# 吸土城FC 微信小程序

小程序以活动查看和固定状态报名为主。网页管理端发布训练、友谊赛、野球和赛事专题，队员在小程序查看活动、选择报名状态、查看训练榜与个人记录。`0.2.29` 已上传开发版，在「我的」加入新活动提醒订阅入口。配套服务端已部署。发布状态见 `docs/deployment-2026-10-23.md`。

## 使用范围

- 底部四个入口：活动、报名、训练榜、我的；赛事专题从活动和我的进入，只读。
- 报名状态为参加、不参加、待定、未确认；保存后由服务端更新，取消或结束活动不可报名。
- 登录使用 wx.login 与服务端独立成员会话。首次完善昵称后才能报名。昵称是基本身份信息，服务端仍进行文字内容检查。
- 小程序不提供活动创建、相册、照片上传和教练编辑。成员可在「我的」上传自己的 JPEG、PNG 或 WebP 头像；服务端限制 6 MB 和 2500 万像素，微信图片审核通过后替换头像。
- 「邀请队友」分享球队卡片；活动详情的「邀请参加」分享按当前活动生成的卡片。新成员从活动卡进入并登录后返回活动详情。
- 「我的 → 新活动提醒」可主动订阅下一条新活动；一次微信授权对应一条提醒。教练创建活动后，服务端向有可用授权的成员发送订阅消息。真机送达仍需验证。
- 网页管理员使用独立密码会话发布活动、专题与照片，保存战术板。微信成员会话不能使用网页管理 API，也不能通过 /api/player 创建活动或写入相册。
- 既有照片及头像数据未删除。网页相册对成员只读，管理员可继续维护。

## 开发与验证

使用微信开发者工具导入本目录，AppID 为 `wxb69316be98ac4a56`，生产 API 地址由 `utils/api.js` 指定为 `https://pitch.coolcookie.cloud`。AppSecret 只在服务端环境变量。request、uploadFile 与头像下载合法域名均需包含该域名。头像异步审核还需要服务端 `ANYPITCH_PUBLIC_URL` 和微信内容安全回调配置。

```bash
node --test scripts/miniprogram/*.test.cjs
GOCACHE="$(pwd)/.gocache" GOMODCACHE="$(pwd)/.gomodcache" go test ./...
npm --prefix web run test
npm --prefix web run build
./scripts/validate_api_smoke.sh
```

标题使用自有 HTTPS 提供的思源宋体 Medium 子集，`utils/fonts.js` 通过 `wx.loadFontFace` 全局加载；队徽为原始文件，XTC FC 字标为 SVG。视觉说明见 `docs/design-docs/miniprogram-v021.md`。

开发者工具预览、真机验证、上传开发版、重新提交审核和公开发布是独立状态。个人主体是否通过审核由微信本次实际审查决定。
