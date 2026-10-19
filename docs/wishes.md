# 游戏许愿池

玩家可以在大厅顶栏打开「许愿池」，用文字描述想实现的游戏，并可附一张参考图。管理员在「围桌管理室 → 许愿审核」中通过或驳回。通过后的许愿会出现在只读代理接口中，供本地制作 agent 领取。

## 状态

| 状态 | 含义 |
| --- | --- |
| `pending` | 已提交，等待管理员审核 |
| `approved` | 已通过，可被本地代理读取 |
| `rejected` | 已驳回；审核备注会显示给提交者 |

管理员可以重新审核已通过或已驳回的条目。每次审核都会写入 `admin_audit`，操作记录中显示为「通过许愿」或「驳回许愿」。

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

四项 OSS 配置必须同时存在；留空则关闭图片上传，纯文字许愿仍然可用。密钥只放在服务器私有 `.env`，不要提交仓库。

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
```

`action` 为 `approve` 或 `reject`；`note` 可选，最多 500 字。

### 本地制作代理

代理接口只读，需在服务器配置 `WISH_AGENT_TOKEN`（至少 16 字节）：

```sh
curl -fsS \
  -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
  "https://board.example.com/api/agent/wishes?status=approved"
```

- 只返回 `approved` 条目，按审核时间升序。
- `since=<unix-seconds>` 只返回审核时间晚于该时刻的条目，适合轮询。
- `limit=1..200`，默认 100。
- 也接受 `X-Wish-Token` 请求头。
- 未配置令牌时返回 503；令牌错误返回 401。
- 返回字段：`id`、`userName`、`text`、`imageUrl`、`status`、`note`、`createdAt`、`reviewedAt`、`approvedAt`、`reviewedByName`。

示例轮询脚本：

```sh
last=0
while true; do
  curl -fsS -H "Authorization: Bearer $WISH_AGENT_TOKEN" \
    "https://board.example.com/api/agent/wishes?since=$last" |
    jq -c '.wishes[] | {id,text,imageUrl,note}' >> wishes.jsonl
  now=$(date +%s); last=$now; sleep 300
done
```

本地 agent 根据 `text` 和 `imageUrl` 自行决定是否制作；服务端不会自动修改游戏或上架新游戏。制作完成后仍需按仓库流程实现、验收和发布。
