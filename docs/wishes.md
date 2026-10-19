# 游戏许愿池

玩家可以在大厅顶栏打开「许愿池」，用文字描述想实现的游戏，并可上传或直接粘贴一张参考图。AI 制作代理可以直接处理待审核许愿：需求含糊时最多向提交者追问 2 次，问满或信息足够后开始实现；管理员仍可在「围桌管理室 → 许愿审核」通过、驳回或下架条目。

## 状态与追问

| 状态 | 含义 |
| --- | --- |
| `pending` | 已提交，等待管理员审核或 AI 直接领取 |
| `approved` | 管理员已通过，等待开始制作 |
| `in_progress` | 正在实现，本地代理可识别为已开工 |
| `implemented` | 已实现并完成验收 |
| `rejected` | 已驳回；审核备注会显示给提交者 |

AI 追问不改变主状态，单独记录在 `wish_questions` 中。每个许愿最多 2 个问题；同一时间只能有一个未回答的问题。玩家回答后 AI 才能继续追问或开始实现。未回答的追问会让代理跳过该条目，避免重复提问。

管理员和代理的状态变化都会写入 `admin_audit`，操作记录中显示为「通过许愿」「AI 追问许愿」「开始实现许愿」「完成许愿」或「驳回许愿」。审核时间按秒严格递增，代理使用 `since` 增量轮询时不会漏掉同一秒内的连续状态变化。新游戏现在默认上架，管理员可随时下架。

## 图片

- 可选，支持 PNG、JPEG、WebP、GIF，最大 5 MB；可以点「添加参考图」选择文件，也可以直接在输入框粘贴截图。
- 按文件内容识别类型，不信任扩展名。
- 不接收 SVG（避免在 CDN 上执行主动内容）。
- 图片写入 OSS 不可变对象前缀，例如 `bga/wishes/20261019/<random>.png`，公开地址为 `https://cdn.ichenj.com/bga/wishes/...`。
- 服务端使用 `x-oss-forbid-overwrite` 和内容 MD5，避免覆盖既有对象。
- OSS 写入失败时不会创建许愿记录。

服务端配置（`.env`）：

```sh
OSS_ENDPOINT=https://stoneage-assets.oss-cn-shanghai.aliyuncs.com
OSS_BUCKET=stoneage-assets
OSS_ACCESS_KEY_ID=...
OSS_ACCESS_KEY_SECRET=...
WISH_IMAGE_BASE_URL=https://cdn.ichenj.com
WISH_IMAGE_PREFIX=bga/wishes
```

四项 OSS 配置必须同时存在；留空则关闭图片上传，纯文字许愿仍然可用。密钥只放在服务器私有 `.env`，不要提交仓库。反向代理需允许至少 6 MiB 的请求体（Nginx 示例：`client_max_body_size 6m;`），否则 5 MB 图片会在到达应用前被拒绝。

## 接口

### 玩家

- `GET /api/wishes`：当前登录用户自己的许愿，最多 100 条。
- `POST /api/wishes`：
  - JSON：`{"text":"想做一款……"}`。
  - 表单（可带图）：`multipart/form-data`，字段 `text`、可选 `image`。
  - 每个用户每小时最多提交 10 条。

### 管理员

- `GET /api/admin` 返回 `wishes` 和 `wishStats`（管理后台每 5 秒刷新）。
- `POST /api/admin/wishes/{id}`：

```json
{"action":"approve","note":"已排入下个版本"}
{"action":"start","note":"本周开工"}
{"action":"implement","note":"已完成并部署"}
{"action":"reject","note":"暂不考虑"}
```

`action` 为 `approve`、`start`、`implement` 或 `reject`；`note` 可选，最多 500 字。状态依次为 `approved` → `in_progress` → `implemented`，也可以直接通过或驳回。

### 本地制作代理

代理使用服务器配置的 `WISH_AGENT_TOKEN`（至少 16 字节）读取、追问和推进许愿：

```sh
curl -fsS \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  "https://board.example.com/api/agent/wishes"
```

- 默认返回 `pending`、`approved`、`in_progress`、`implemented` 条目，按创建/审核时间升序。
- `status=pending|approved|in_progress|implemented` 只返回对应状态；`rejected` 不向代理开放。
- 每条包含 `questions`、`questionCount`、`needsAnswer`。`needsAnswer=true` 时表示已有未回答追问，代理必须等待，不能重复提问或开始实现。
- `since=<unix-seconds>` 只返回创建（pending）或最近审核时间晚于该时刻的条目，适合轮询。
- `limit=1..200`，默认 100。
- 也接受 `X-Wish-Token` 请求头；未配置令牌返回 503，令牌错误返回 401。
- 返回字段：`id`、`userName`、`text`、`imageUrl`、`status`、`note`、`createdAt`、`reviewedAt`、`approvedAt`、`reviewedByName`、`questions`。`approvedAt` 只在当前状态为 `approved` 时给出；后续状态以 `status` 和最近一次 `reviewedAt` 为准。

追问（最多 2 次，且上一个问题必须已回答）：

```sh
curl -fsS -X POST \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"question":"更希望单人还是多人？"}' \
  "https://board.example.com/api/agent/wishes/<wish-id>/questions"
```

玩家在自己的许愿池中回答后，代理继续读取；问满 2 次或信息足够即可开始：

```sh
curl -fsS -X POST \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action":"start","note":"需求已确认，开始实现"}' \
  "https://board.example.com/api/agent/wishes/<wish-id>"
```

- `start`：允许 `approved`，或没有未回答追问的 `pending` → `in_progress`。未审批许愿也可以直接开始，无需管理员操作。
- `implement`：允许 `approved` 或 `in_progress` → `implemented`。
- 有未回答追问时 `start` 返回 409；最多追问 2 次；备注最多 500 字。
- 代理不能通过该接口通过或驳回待审核许愿；审批仍只能由管理员完成。
- 操作以「本地制作代理」写入管理操作记录。

玩家回答接口：

```sh
curl -fsS -X POST \
  -H "Cookie: wb_session=..." -H "Content-Type: application/json" \
  -d '{"answer":"希望 2 人合作"}' \
  "https://board.example.com/api/wishes/<wish-id>/answer"
```

- 只有许愿提交者可以回答；一个问题只能回答一次。
- 回答长度 1–1000 字；未回答时 `needsAnswer=true`。

本仓库提供持久化启动脚本：

```sh
./scripts/wish-loop.sh          # 启动 Claude Code 并每 5 分钟执行
./scripts/wish-loop.sh --dry-run # 只看将要执行的 /loop 指令
```

脚本读取本地 `.local/wishes/agent-token`，并把 `scripts/wish-loop-prompt.txt` 作为 `/loop` 指令；换机器时重新配置令牌即可。本地 agent 根据 `text`、问答和 `imageUrl` 自行决定玩法；服务端不会自动创建或下架游戏。制作完成后按仓库流程实现、验收和发布，再回写 `implemented`。
