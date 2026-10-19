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

`python3 -B scripts/accept_wishes.py` 会启动临时服务、内存 OSS 假服务和无头浏览器，共 56 项通过：

- 大厅「许愿池」弹窗打开；文字和图片预览正常显示（图片为 `data:` 预览，不受 CSP 阻断）。
- 提交带图许愿：OSS 收到一次 `PUT`，`x-oss-forbid-overwrite: true`、`Content-Type: image/png`、字节 SHA-256 与本地文件一致。
- 玩家「我的许愿」显示待审核、图片真实渲染、CDN 地址可回读；1280 桌面与 320 手机宽度下弹窗不越界、无横向溢出、操作可用。
- 许愿弹窗、审核卡片、状态标签和备注的计算样式对比度均不低于 4.5:1。
- 管理后台五个列表页（游戏上架、许愿审核、用户与在线、牌桌管理、操作记录）均有独立刷新按钮并可用。
- 管理员后台「许愿审核」显示提交者、文字和参考图；通过 → 正在实现 → 已实现的每次推进都会立即更新状态与备注，并写入操作记录。
- 本地代理使用同一令牌调用受限状态接口，把已通过许愿推进为「正在实现」；管理员后台随轮询显示新状态。
- 玩家重新打开许愿池看到「已实现」和管理员最终备注。
- 本地代理使用 Bearer 令牌分别读到已通过、正在实现和已实现条目，状态字段正确；`status=approved`、`status=in_progress` 在状态推进后不再返回旧条目；错误令牌返回 401。
- 截图：`.local/wishes-e2e/<run>/desktop-wish-dialog.png`、`mobile-320-wish.png`、`admin-wish-pending.png`、`admin-wish-approved.png`、`admin-wish-implemented.png`、`player-wish-implemented.png`；结果 JSON 同目录。

本次运行：`.local/wishes-e2e/660dfed2/result.json`（56/56）。

## 上线配置

- 服务器 `.env` 需同时提供 `OSS_ENDPOINT`、`OSS_BUCKET`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`，并设置 `WISH_IMAGE_BASE_URL`、`WISH_IMAGE_PREFIX`。
- `WISH_AGENT_TOKEN` 至少 16 字节；未配置时代理接口返回 503，不影响玩家提交纯文字许愿。
- 图片写入 `bga/wishes/YYYYMMDD/<random>.<ext>` 不可变对象；媒体不进 Git。
- 新功能不改变游戏上下架状态，也不自动创建或上架游戏。

## 发布与部署

- main 提交 `e0b2a528` 的 `Check main and build` 运行 `39234703806` 全绿（shared 计划/构建/HTTP 持久化与 race、`Test and Linux amd64 binary`）。
- `v1.18.0` 指向同一提交；`Publish version tag` 运行 `39235969348` 复用已验证构建，Release 已发布。
- 服务器运行 `ghcr.io/k0ngk0ng/wire-board@sha256:8de33e069b9eacfeba56e0625bf57ff7854468b3816848f7d4127868e96163b1`，OCI revision `e0b2a528`、version `v1.18.0`，`/healthz` 正常。
- `.env` 已配置 OSS 四项、`WISH_IMAGE_BASE_URL=https://cdn.ichenj.com`、`WISH_IMAGE_PREFIX=bga/wishes` 和 `WISH_AGENT_TOKEN`；Nginx `bga.ichenj.com` 增加 `client_max_body_size 6m;` 并已 `nginx -t`、reload。
- 生产冒烟：临时账号经 `https://bga.ichenj.com` 提交带图许愿，图片写入 OSS 后从 CDN 回读字节一致；随后删除该用户、许愿和 OSS 临时对象，数据库中无残留。
- 代理接口：带令牌读取返回 200（当前为空列表），无令牌返回 401。
- 已有游戏设置保留；管理员此前已上架的 `battlecity` 保持上架，未做任何自动上/下架。
- 保留本次更新前与上一份备份 `data-20261019T101526Z-664607.tar.gz`、`data-20261019T073207Z-307786.tar.gz`（均通过 `quick_check`）；更旧备份已删除。历史镜像已清理，保留 `v1.18.0` 当前镜像和 `v1.17.0` 回滚镜像。
- 本地代理令牌保存在 `.local/wishes/agent-token`（已 Git 忽略）。调用示例见 [许愿池说明](wishes.md)。

## v1.18.1

- main 提交 `52d82f7a` 的 `Check main and build` 运行 `39239500736` 全绿；`v1.18.1` Release 运行 `39240777557` 复用同一已验证构建。
- 服务器运行 `ghcr.io/k0ngk0ng/wire-board@sha256:5f44c0401778328ff998eaae616eb05127f25901e655033c9807fb28ebc428df`，OCI revision `52d82f7a`、version `v1.18.1`，`/healthz` 正常。
- 新增 `POST /api/agent/wishes/{id}`：生产环境已验证无令牌 401、未知操作 400、不存在 ID 404；本地 E2E 验证 `approved → in_progress → implemented`、非法转换 409 与审计记录。
- 管理后台五个列表页的独立刷新按钮已进入线上 bundle，浏览器验收共 56 项通过。
- 已有游戏设置保持；`battlecity` 继续上架。
- 保留本次更新前与上一份备份 `data-20261019T105535Z-753854.tar.gz`、`data-20261019T101526Z-664607.tar.gz`（均通过 `quick_check`）；更旧备份已删除。镜像清理后保留当前 `v1.18.1` 和上一版 `v1.18.0` 回滚镜像。

## 追问、粘贴截图与默认上架

2026-10-19 增补：

- `go test -run 'TestWish|TestAgent' ./internal/server` 通过，覆盖：pending 许愿可直接开始、最多 2 次追问、未回答时禁止再次追问/开始、回答只能由提交者完成、管理后台和代理均能看到问答与 `needsAnswer`、追问写入操作记录。
- 真实浏览器验收 63 项通过：截图图片可通过粘贴进入上传预览；AI 追问显示在玩家「我的许愿」；玩家回答后代理读取到回答并将 pending 许愿推进为正在实现；管理员看到问答和状态并完成；桌面/320 布局、对比度和刷新按钮保持通过。
- 本次运行：`.local/wishes-e2e/f4a3b5e2/result.json`（62/62）；增加大厅版本显示后为 63/63。
- 新游戏默认上架：`TestNewModuleDefaultsToPublishedE2E` 验证注册即可见、可直接创建；管理员显式下架后创建被拒绝，重启保留下架决定；重启时新增模块默认上架。共享 HTTP 夹具的 `AssertUnpublished` 现在先由管理员显式下架，再验证隐藏行为；`TestModuleHiddenProfileHTTP` 同样先下架再检查空战绩。
- 管理后台游戏卡片移除「大厅可见，可以创建牌桌」冗余说明，浏览器检查卡片内不再有该段落。
- 大厅顶部和页脚显示部署版本（`WIRE_BOARD_VERSION`），浏览器验收 63/63 通过。
- 持久化入口：`scripts/wish-loop.sh` 读取 `scripts/wish-loop-prompt.txt` 并启动 Claude Code 的 `/loop 5m`；`--dry-run` 只打印将执行的指令。代理令牌仍在 `.local/wishes/agent-token`。
- 策略调整：新游戏默认上架，管理员可显式下架；已存在的显式设置（包括 `dota=0` 和此前 `fortressblue=0`）不会被启动迁移覆盖。

## v1.21.0

- main 提交 `35ca0e3e` 的 `Check main and build` 运行 `39261339886` 全绿（shared、starfort、monopoly、bubbleparty、amd64 构建）。
- `v1.21.0` 指向同一提交；Release 运行 `39262899741` 复用已验证构建。
- 服务器运行 `ghcr.io/k0ngk0ng/wire-board@sha256:73286554726673c1644e933b98a33e7aa776df5d73cdae7b3b006daf40f2f2b0`，OCI revision `35ca0e3e`、version `v1.21.0`，`/healthz` 正常。
- 线上 bundle 包含「粘贴截图」「待回复」；`wish_questions` 表存在；代理读取包含 `questionCount`/`needsAnswer`；追问接口无令牌 401、不存在 ID 404。
- 已有游戏设置保留：`battlecity=1`、`bubbleparty=1`、`fortressblue=1`（管理员已上架）、`dota=0`。
- 保留本次更新前与上一份备份 `data-20261019T135707Z-1130207.tar.gz`、`data-20261019T120740Z-912688.tar.gz`（均通过 `quick_check`）；更旧备份已删除。镜像清理后保留当前 `v1.21.0` 与上一版 `v1.20.0` 回滚镜像。
- 用户要求停止自动许愿循环；会话 cron 已删除。持久化入口仍为 `scripts/wish-loop.sh`，需要时手动启动。
