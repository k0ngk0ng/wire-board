# 坦克大战（battlecity）验收记录

## 范围与来源

- 游戏：1v1／2v2／3v3 双基地实时坦克对战。
- ROM：用户提供的 `/Users/jason/Documents/nes/roms/02/坦克大战(打坦克，Battle City).zip` 内 `Battle City (J).nes`，24,592 bytes，SHA-256 `a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c`。
- ROM 为 mapper 0 NROM-128，16 KiB PRG、8 KiB CHR、512 个未压缩 8×8 tile。
- 研究/提取产物保存在 `.local/battlecity/`；商业媒体不进 Git。

## 已完成的规则与平台验证

- `go test -count=1 ./games/battlecity/... ./games`：通过。
- `go vet ./games/battlecity/...`：通过。
- `go test -race -count=1 ./games/battlecity/...`：通过（全量 12 图 × 2/4/6 矩阵在非 race 模式运行一次；race 覆盖聚焦对局、HTTP 和并发输入）。
- 地图：12 张 13×13 地图，全部通过 180° 对称、基地保护圈、出生点和双方基地可达性校验。
- 模块 E2E：2／4／6 人 × 12 张地图的完整电脑对局；结果 Ranks/Winners/Scores/Team 校验；部分地图自然和局。
- HTTP E2E：真实注册/登录/SQLite；默认下架；管理员上架；2／6 人房间；选坦克；倒计时；输入序号；观众拒绝操作；错误 epoch/seq；服务重启恢复同一 epoch 和席位。
- 人机混合 HTTP：1 名真人 + 1 名电脑的 1v1 实时输入，真人 Seq 从 0 到 1，电脑正常行动。
- 前端单测：`npm --prefix web test -- battlecity`，6 项通过（最新输入、释放、epoch 重置、禁用、D-pad、地图目录对称性）。
- 前端构建：`npm --prefix web run build` 通过（仅有共享 bundle 体积提示）。
- 边界：`python3 -B scripts/check_game_boundaries.py` 通过。
- 真实浏览器（本地素材包）：30 项通过，截图和 `result.json` 在 `.local/battlecity/browser/`。
  - 默认下架、创建两步弹窗、2/4/6 座位选项、12+随机地图、等待座位、6 人 3v3 选坦克、倒计时、键盘移动/开火、原生触屏方向/开火、1280/390/320 资源可见、320 坦克尺寸、托管开/关、刷新保持 epoch/序号、结果浮层、关闭/重开、再来一局。
- 真实浏览器（仅 CDN、构建中无本地商业媒体）：30 项通过；`data-battlecity-atlas="1"` 且 `cdn-audio-metadata` 验证 BGM 可从 `cdn.ichenj.com` 加载。CDN 的 `atlas.json` 不参与运行时（帧表在 `games/battlecity/web/atlas.ts`），避免跨域 CORS 限制。
- 范围检查：`python3 -B scripts/ci/scope.py run --game battlecity --backend` 与 `--game shared` 通过。

## 证据路径

- 地图与规则：`games/battlecity/maps/catalog.json`、`games/battlecity/server/*_test.go`。
- HTTP/SQLite：测试运行输出；`gametest` 证据在 `.local/poker/battlecity/http/`。
- 浏览器：`.local/battlecity/browser/result.json`、`*.png`、`*-contrast.json`、`*-controls-contrast.json`。
- ROM/CHR/参考：`.local/battlecity/extract/`、`.local/battlecity/reference/`、`.local/battlecity/rea/`。

## 素材状态

- 已完成图集：101 帧（地形、基地、5 类坦克双方配色 × 4 方向 × 2 帧、子弹、爆炸、护盾、六种道具），`atlas.webp` 3,698 bytes。
- 已完成 11 个音频键：从原 ROM 的 `$AA51/$AA7E` 声音引擎与队列快照渲染，未合成替代；映射见 `games/battlecity/assets/sources.json`。
- 13 个对象、482,352 bytes 已上传 `bga/v20261002-1/battlecity/v1/` 并通过 CDN 回读（SHA-256/MIME）。
- 素材契约（帧名/音频键）见 `games/battlecity/CONTRACT.md` 第 11 节。
- 上传前缀：`https://cdn.ichenj.com/bga/v20261002-1/battlecity/v1/`；禁止覆盖已有不可变对象。

## 发布与部署

- main 提交 `eae0a2aa`（Merge PR #2）的 `Check main and build` 运行 `39215088393` 全绿：shared 计划/构建/HTTP 持久化与 race、`Game battlecity (0)`、`Game rail (0)`、`Test and Linux amd64 binary` 均成功。
- 版本 `v1.17.0` 指向同一提交；`Publish version tag` 运行 `39216375345` 复用该次构建产物，未重新编译应用，Release 已发布 `wire-board-linux-amd64.tar.gz` 与 `SHA256SUMS`。
- 服务器已更新到 `v1.17.0`：运行镜像 `ghcr.io/k0ngk0ng/wire-board@sha256:40b2d9044b1df734ad0b725a91321089dc6f9b37e810881fc498b52ae29c7d93`，OCI revision `eae0a2aa`、version `v1.17.0`；`/healthz` 返回 `{"ok":true}`，容器健康。
- 部署保留已有上下架设置，新游戏 `battlecity=0`；`spf2t`、`tetris`、`racing` 等原有设置未变。管理员需在后台显式上架后才对玩家开放。
- 更新前已生成在线 SQLite 备份并保留最近两份：`data-20261019T073207Z-307786.tar.gz`（本次更新前）与 `data-20261018T173853Z-2684835.tar.gz`；两者均通过 `PRAGMA quick_check`，更旧备份已按约定删除。
- `update.sh clean` 清理了历史镜像，保留当前镜像与上一版 `v1.16.0` 回滚镜像；未执行全局 Docker prune，服务健康和磁盘空间已复核。
