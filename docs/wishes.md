# 游戏许愿池

玩家可以在大厅顶栏打开「许愿池」，用文字描述想实现的游戏，并可附一张参考图。管理员在「围桌管理室 → 许愿审核」中通过或驳回。通过后的许愿会出现在只读代理接口中，供本地制作 agent 领取。

## 状态

| 状态 | 含义 |
| --- | --- |
| `pending` | 已提交，等待管理员审核 |
| `approved` | 已通过，等待开始制作 |
| `in_progress` | 正在实现，本地代理可识别为已开工 |
| `implemented` | 已实现并完成验收 |
| `rejected` | 已驳回；审核备注会显示给提交者 |

管理员可以把已通过条目推进到「正在实现」和「已实现」，也可以重新审核已驳回条目。每次状态变化都会写入 `admin_audit`，操作记录中显示为「通过许愿」「开始实现许愿」「完成许愿」或「驳回许愿」。审核时间按秒严格递增，代理使用 `since` 增量轮询时不会漏掉同一秒内的连续状态变化。

## 图片

- 可选，支持 PNG、JPEG、WebP、GIF，最大 5 MB；按文件内容识别类型，不信任扩展名。
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

代理接口只读，需在服务器配置 `WISH_AGENT_TOKEN`（至少 16 字节）：

```sh
curl -fsS \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  "https://board.example.com/api/agent/wishes?status=approved"
```

- 默认返回 `approved`、`in_progress`、`implemented` 三种已审核条目，按审核时间升序；代理可据此区分待制作、正在实现和已完成。
- `status=approved|in_progress|implemented` 只返回对应状态。
- `since=<unix-seconds>` 只返回审核时间晚于该时刻的条目，适合轮询。
- `limit=1..200`，默认 100。
- 也接受 `X-Wish-Token` 请求头。
- 未配置令牌时返回 503；令牌错误返回 401。
- 返回字段：`id`、`userName`、`text`、`imageUrl`、`status`、`note`、`createdAt`、`reviewedAt`、`approvedAt`、`reviewedByName`。`approvedAt` 只在当前状态为 `approved` 时给出；进入「正在实现」或「已实现」后，以 `status` 和最近一次 `reviewedAt` 为准。

代理也可以推进自己领取的已通过许愿，而不需要管理员账号：

```sh
curl -fsS -X POST \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action":"start","note":"已开始实现"}' \
  "https://board.example.com/api/agent/wishes/<wish-id>"
```

- `start`：仅允许 `approved` → `in_progress`。
- `implement`：允许 `approved` 或 `in_progress` → `implemented`。
- 备注最多 500 字；省略时写入默认说明。
- 非法状态转换返回 409；操作以「本地制作代理」写入管理操作记录，不会授予其他管理权限。
- 代理不能通过该接口通过或驳回待审核许愿；审批仍只能由管理员完成。

示例轮询脚本：

```sh
last=0
while true; do
  curl -fsS -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
    "https://board.example.com/api/agent/wishes?since=$last" |
    jq -c '.wishes[] | {id,status,text,imageUrl,note}' >> wishes.jsonl
  now=$(date +%s); last=$now; sleep 300
done
```

本地 agent 根据 `text` 和 `imageUrl` 自行决定是否制作；服务端不会自动修改游戏或上架新游戏。制作完成后仍需按仓库流程实现、验收和发布。
