# 围桌 · Wire Board

为朋友小圈子搭建的中文联机游戏大厅，使用 Go、React/TypeScript、SQLite 和 WebSocket，支持桌游、棋牌及即时对战。游戏持续更新，具体列表、人数和玩法以大厅目录与游戏内「玩法速查」为准。

## 平台功能

- 邀请码注册、账号登录、密码房间、准备开局、结算和再来一局。
- 实时联机、断线恢复、私人信息隔离；部分游戏支持保存退出与原账号续玩。
- 按游戏提供电脑玩家、托管、观战、键盘、手柄和触屏操作。
- 牌桌聊天、好友、个人战绩、积分与排行榜。
- 游戏上下架、用户与牌桌管理、操作审计，以及玩家许愿与管理员审核。

服务器负责规则、隐藏信息和存档。图片、音频、字体及客户端运行模块经 OSS/CDN 按需加载；游戏资源位于 `bga/<game-id>/`，公共资源位于 `bga/_common/`，内容不变时复用原地址。

## 部署

从 [Releases](https://github.com/k0ngk0ng/wire-board/releases) 下载 Linux amd64 发布包与 `SHA256SUMS`，校验后使用包内的可执行文件或 Docker Compose 部署。服务器无需安装 Go 或 Node。

Docker 部署需将 `compose.yaml`、`.env.example` 和 `update.sh` 放到同一目录，复制 `.env.example` 为 `.env`，设置邀请码及站点配置。选择已发布的明确版本执行 `./update.sh vX.Y.Z`。脚本会在线备份 SQLite、固定镜像摘要并检查健康；失败处理与存档兼容限制见[部署说明](docs/deployment.md)。

- 公网访问使用 HTTPS，设置 `PUBLIC_ORIGIN` 与 `COOKIE_SECURE=true`；反向代理需支持 WebSocket。
- `.env` 由 shell 或 Compose 加载，程序本身不读取该文件。
- 单实例运行，SQLite 数据目录放在本地磁盘；不要让多个实例共享同一数据库文件。
- 更新保留数据卷和既有管理设置。不要执行 `docker compose down -v`，该命令会删除数据卷。

main 通过检查后生成候选包；本地 CDN 回读通过后，版本 tag 复用同一提交的构建发布。GitHub Actions 不会自动登录或更新服务器。完整流程见[发布规范](rules/deploy.md)。

## 业务配置与资源

站点域名、CDN 地址和 OSS 信息不写入源码。本地配置保存在 Git 忽略的 `.local/business.json`，执行 `. scripts/env.sh` 后加载；GitHub 测试与发布读取仓库变量。

| 变量 | 用途 |
| --- | --- |
| `PUBLIC_ORIGIN` | 站点 HTTPS 源地址 |
| `CDN_ORIGIN` | CDN HTTPS 源地址 |
| `OSS_BUCKET` | OSS 存储桶 |
| `OSS_ENDPOINT` | OSS 访问端点 |
| `ASSETS_BASE_URL` | 运行时素材根地址，使用配置的 CDN 下的 `/bga` |

OSS 凭据通过本地私密配置或 GitHub secrets 提供。运行服务还需设置 `INVITE_CODE`，数据目录由 `DATA_DIR` 指定。管理、代理与上传配置见 [.env.example](.env.example) 和[部署说明](docs/deployment.md)。

Git 只保存源码、规则数据与资源元数据。素材由 `assets/resources.json` 定位并校验恢复，历史资源登记在 `assets/resource-history.json`；本地素材和验收产物不提交。详见[资源外置与配置](docs/asset-reuse.md)、[素材部署](docs/assets.md)和[加载规范](rules/loading.md)。

## 本地开发

需要 Go 1.26+、Node 22+ 和支持 `lzma` 的 Python 3。先填写 `.local/business.json`，再从仓库根目录执行：

```sh
. scripts/env.sh
python3 -B scripts/resource_store.py hydrate
npm --prefix web ci
npm --prefix web run build
python3 -B games/contra/build/native.py --runtime .local/contra-web-runtime/public/manifest.json --out .local/wire-board
INVITE_CODE=local-friends ADDR=127.0.0.1:18080 DATA_DIR="$PWD/.local/data" .local/wire-board
```

前端开发使用 `npm --prefix web run dev`；后端监听 `127.0.0.1:8080` 时可通过 Vite 访问。首次启动 Go 服务前必须构建前端，并通过原生构建入口绑定运行清单；引擎或构建输入变化后重新构建，不复用旧清单。

缓存、临时文件及验收结果保存在仓库 `.local/` 中；游戏产物使用 `.local/<game-id>/`。按实际改动选择检查范围，复杂交互保留可重复执行的 E2E 结果。开发前阅读 [AGENTS.md](AGENTS.md) 和[通用规范](rules/common.md)。

## 开发与运维文档

- [游戏模块开发](docs/game-development.md)：目录、接入、规则与验收。
- [CI 检查范围](scripts/ci/README.md)：定向检查、完整验收与构建产物。
- [部署与回滚](docs/deployment.md)：服务器配置、备份、更新和故障处理。
- [管理后台](docs/admin.md)、[积分规则](docs/ratings.md)、[许愿池](docs/wishes.md)。
- [版本更新日志](docs/release-history.md)和[验证记录](docs/verification.md)。

## 第三方资源

项目不连接 BGA 游戏服务。第三方素材、数据与授权范围见[第三方声明](docs/THIRD_PARTY.md)，由部署者按相应授权配置和使用。
