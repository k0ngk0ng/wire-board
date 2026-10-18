# 街霸方块验证记录

日期：2026-10-19。游戏 `spf2t`，固定 2 人实时对战，默认下架。

## 范围

- 服务端规则：碎裂/强化/钻石/反击宝石、连锁、攻击、回合、单局/三局两胜、电脑与托管、恢复。
- 前端：选角、倒计时、双棋盘、警告、键盘/触屏、托管、刷新、结果浮层与重赛、320px。
- 素材：21 个派生媒体对象上传 OSS，CDN 回读；商业媒体不进入 Git。
- 既有公共平台、创建弹窗、等待房间、玩家栏、回合提示、结果浮层保持共用。

## 命令与结果

```sh
. scripts/env.sh
python3 -B scripts/check_game_boundaries.py
npm --prefix web run build
npm --prefix web test -- spf2t
go test -count=1 ./games/spf2t/server ./games/spf2t/tests
go test -race -count=1 -timeout=10m ./games/spf2t/...
go vet ./games/spf2t/...
python3 -B scripts/spf2t/verify-assets.py --dist
python3 -B scripts/spf2t/verify-assets.py --oss
python3 -B scripts/spf2t/check-browser.py
python3 -B scripts/ci/scope.py run --game spf2t --backend
python3 -B scripts/ci/scope.py run --game shared
```

| 检查 | 结果 | 证据 |
|---|---|---|
| 边界检查 | 21 个游戏无跨游戏/legacy 实现导入 | `scripts/check_game_boundaries.py` 输出 |
| 前端构建 | 通过 | `web/dist/` |
| 前端单测 | 10 通过 | `games/spf2t/tests/{input,events,gems}.test.mjs` |
| Go 规则/模块单测 | 通过 | `games/spf2t/server/rules_test.go` |
| 模块 E2E | 单局/三局两胜完整机器人对局、选角超时、同角色、非法/过期操作 | `.local/spf2t/module/` |
| HTTP/SQLite E2E | 未上架/上架、观战隐私、nonce 幂等、并发行动、重启保留、托管/Reclaim、选项校验 | `.local/poker/spf2t/http/` |
| 竞态 | `go test -race ./games/spf2t/...` 通过 | 终端输出 |
| 浏览器验收（本地素材） | 27 项通过；1280/390/320 截图、对比度、键盘、原生触屏、托管、刷新、结果、重赛 | `.local/spf2t/browser/result-local.json`、`*.png` |
| 浏览器验收（CDN 素材） | 27 项通过；`ASSETS_BASE_URL=https://cdn.ichenj.com/bga/v20261002-1` | `.local/spf2t/browser/result-cdn.json` |
| 素材本地校验 | 21 对象、693,488 字节，尺寸/时长/签名/哈希通过 | `.local/spf2t/assets-verify.json` |
| 素材 OSS 发布 | 21 个不可变对象，CDN SHA-256/MIME 回读通过 | `.local/spf2t/oss-publish.json`、`.local/spf2t/oss-verify.json` |
| 范围检查 | `scope.py run --game spf2t --backend` 5 项通过 | `.local/ci/spf2t/` |
| 共享检查 | `scope.py run --game shared` 通过（`internal/server` 234.5s） | `.local/ci/shared/` |
| 部署脚本 | `bash -n`、`shellcheck`、`test_update.py` 7 项通过 | 终端输出 |

## 浏览器检查清单

`default-unlisted-admin-only`、`create-1280/390/320`、`waiting-1280/320`、`select-1280`、`countdown-1280`、`select-countdown-playing`、`keyboard-move`、`keyboard-rotate`、`keyboard-hard-drop`、`native-touch-move`、`native-touch-hard-drop`、`autoplay-disables-controls`、`autoplay-returned-control`、`game-1280/390/320`、`resources-390/320`、`board-geometry`、`refresh-without-animation-replay`、`result-1280/320`、`result-closable-reopenable`、`rematch-returns-to-waiting`。

浏览器截图：

- `.local/spf2t/browser/select-1280.png`：8 名角色选角、对手已选提示、15 秒选角。
- `.local/spf2t/browser/game-1280.png`、`game-390.png`、`game-320.png`：双棋盘、公共提示、警告、下一对、触屏按钮。
- `.local/spf2t/browser/result-1280.png`、`result-320.png`：共用结果浮层、电脑局不计积分、再来一局。

## 素材

- 角色立绘、BGM、音效来自用户提供的 `.local/spf2t`；派生文件位于 OSS `bga/v20261002-1/spf2t/v1/`。
- 8 名角色映射：morrigan、chunli、ryu、ken、hsienko、donovan、felicia、sakura；Ryu 立绘用参考对局截图补齐，其余来自提取立绘。
- 当前宝石使用内置矢量/渐变绘制；`gems.webp` 图集尚未打包，前端按 `web/gems.ts` 的 4×5 布局保留后续接入点。
- 封面为 `games/spf2t/assets/art.ts` 的原创 SVG；不使用原版盒装封面、扫描图、Capcom Logo 或大段标题字。
- 商业媒体不进入 Git；授权状态见 `games/spf2t/assets/sources.json` 和 `docs/THIRD_PARTY.md`。

## 参考复刻

本地 RetroArch/fBNeo 参考证据保存在 `.local/spf2t/reference/`：角色选择 8 席、`3 ROUND MATCH`、6 列棋盘、NEXT 双宝石、回合/连锁/强化奖励。攻击公式为参考 FAQ 基准的平衡近似；反击图案按提取的 12×6 行优先序列实现。详见 `.local/spf2t/reference/findings.md`。

## 上架

游戏保持默认下架；验证时通过管理员接口临时上架。部署或发布不得自动上架新游戏。
