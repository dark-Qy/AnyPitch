# Validation Gate

声明完成前必须留下验证证据：

1. 自动化：`go test ./...`、`npm --prefix web run test`、`npm --prefix web run build`
2. Smoke：`./scripts/validate_api_smoke.sh`
3. 真实链路：浏览器打开 Web，检查桌面和手机视口下登录、战术板、队员、日程、出勤主路径

如果某项无法执行，必须在交付说明中明确原因。
