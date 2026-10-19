# 游戏许愿池验收记录

日期：2026-10-19。范围：共享服务端、管理后台、大厅许愿弹窗、OSS 图片上传与只读制作代理接口；不涉及任何游戏规则。

## 自动化检查

```sh
. scripts/env.sh
go test -count=1 -run 'TestWish|TestAgentFeed' ./internal/server
go test -count=1 ./internal/server
go vet ./...
npm --prefix web run build
npm --prefix web test
python3 -B scripts/check_game_boundaries.py
```

结果：

- `internal/server` 全量与许愿专项测试通过；覆盖未登录提交/读取、文本边界、未知字段、图片类型嗅探、SVG/伪装文件拒绝、5 MB 上限、OSS 失败不落库、每小时 10 条限制、管理员权限、审核备注、审计记录、重启持久化、代理令牌与 `since` 过滤。
- 前端构建与 444 项共享测试通过。
- 游戏边界检查通过。

## 真实浏览器验收

`python3 -B scripts/accept_wishes.py` 会启动临时服务、内存 OSS 假服务和无头浏览器，共 44 项通过：

- 大厅「许愿池」弹窗打开；文字和图片预览正常显示（图片为 `data:` 预览，不受 CSP 阻断）。
- 提交带图许愿：OSS 收到一次 `PUT`，`x-oss-forbid-overwrite: true`、`Content-Type: image/png`、字节 SHA-256 与本地文件一致。
- 玩家「我的许愿」显示待审核、图片真实渲染、CDN 地址可回读；1280 桌面与 320 手机宽度下弹窗不越界、无横向溢出、操作可用。
- 许愿弹窗、审核卡片、状态标签和备注的计算样式对比度均不低于 4.5:1。
- 管理员后台「许愿审核」显示提交者、文字和参考图；通过 → 正在实现 → 已实现的每次推进都会立即更新状态与备注，并写入操作记录。
- 玩家重新打开许愿池看到「已实现」和管理员最终备注。
- 本地代理使用 Bearer 令牌分别读到已通过、正在实现和已实现条目，状态字段正确；`status=approved`、`status=in_progress` 在状态推进后不再返回旧条目；错误令牌返回 401。
- 截图：`.local/wishes-e2e/<run>/desktop-wish-dialog.png`、`mobile-320-wish.png`、`admin-wish-pending.png`、`admin-wish-approved.png`、`admin-wish-implemented.png`、`player-wish-implemented.png`；结果 JSON 同目录。

本次运行：`.local/wishes-e2e/8d18ef37/result.json`（44/44）。

## 上线配置

- 服务器 `.env` 需同时提供 `OSS_ENDPOINT`、`OSS_BUCKET`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`，并设置 `WISH_IMAGE_BASE_URL`、`WISH_IMAGE_PREFIX`。
- `WISH_AGENT_TOKEN` 至少 16 字节；未配置时代理接口返回 503，不影响玩家提交纯文字许愿。
- 图片写入 `bga/wishes/YYYYMMDD/<random>.<ext>` 不可变对象；媒体不进 Git。
- 新功能不改变游戏上下架状态，也不自动创建或上架游戏。
