# 国际象棋

独立两人模块，沿用平台创建、等待、玩家栏、托管、计时、历史和结算。默认上架，管理员可随时下架。

## 规则与计分

标准 8 × 8 棋盘，随机黑白、白先。只接受服务端生成的合法 UCI 着法，支持王车易位、吃过路兵及后／车／象／马四种升变。不能把 FEN 或任意局面作为创建选项。

- 将杀获胜；无合法着法但未被将军为逼和。
- 三次重复和连续 50 回合不动兵、不吃子可申请和棋；支持预告合法着法，申请成立时不会执行这一步。五次重复和连续 75 回合自动和棋，将杀优先。
- 重复局面比较走子方、棋子分布、易位权，以及实际存在合法吃过路兵时的过路格；被牵制的兵不能制造不同局面。动兵、吃子后重置重复历史。
- 常见子力死局自动判和；双马对单王不能因为无法强制将杀就误判死局。另有固定兵墙的可达性证明，覆盖 FIDE 手册中的异色象封锁实例；这不是对所有复杂死局的穷举证明。
- 和棋提议附在走子上，对方可接受、拒绝或直接走子。不因提议或回应重置时限。认输原则上判负，对手无法将杀时按和棋处理。
- 真人和棋双方 0 积分、不计胜场，保留历史。含电脑的对局不计分。

采用休闲每步时限：30～300 秒，默认 120 秒；不是比赛总时钟。超时由平台托管，取消托管不会重置对方的时限。保存 FEN、UCI／SAN、重复历史和 PGN，重启后继续原局。

合法走子依赖固定版本 `github.com/corentings/chess/v2 v2.6.0`（MIT）。和棋申请、重复判定、结果、时限、事件和策略均留在本游戏目录。规则依据 FIDE Arbiter Manual 2026 的棋规部分；来源与下载摘要保留在棋类调研目录。

## 电脑

两档本地基本策略，不依赖外部模型、付费 API 或服务。快速回退只选择合法着法，优先一步将杀；后台采用迭代加深、Alpha-Beta、吃子静态搜索和重复局面计数。

入门预算为 35ms／1,800 节点／深度 2，标准为 180ms／16,000 节点／深度 4，受平台外层取消和 500ms 限制。预算限制不代表 Elo；不宣称达到专业引擎棋力。计算结果通过原有 HTTP 行动和模块 `Apply` 校验，不能直接修改权威局面。

## 界面与素材

己方在下方，只旋转显示坐标，不改变席位或服务端棋盘编号；观战默认白方在下。鼠标、键盘方向键／回车和触屏均先选棋子、选目标，再确认；升变必须明确选择棋种。

棋盘保持正方形，桌面宽度上限为 620px，手机自适应。走子动画由服务端事件驱动，易位的王与车同时移动；到达前保留被吃棋子和原子力、吃子统计及未结算状态。连续事件排队，刷新／重连／跳号直接恢复最新局面，减少动态效果时立即呈现结果。

19 个素材：12 个棋子 WebP、原创棋盘与封面 WebP、5 个 WAV（走子、吃子、将军、胜利、和棋）。棋子使用 Chessnut，固定提交 `2b8eaf14a31edad7e9deb53b1473e1d4857868a9`，保留 Apache-2.0 许可及版权说明；走子和吃子来自 Kenney CC0 的 `chipsCollide1.ogg`、`chipsCollide2.ogg`；其余音效由平台原创生成器生成。

全部通过 `ASSETS_BASE_URL/chess/v1/` 获取；图片与音效回退到本地 `/chess/v1/`。两处图片均失败时使用可识别的棋种符号。播放只使用 `BoardProps.audio`，遵循平台静音与设备媒体音量设置。素材来源、许可、原始文件和成品 SHA-256 见 `assets/sources.json`。修改已发布素材时提升目录版本，不覆盖不可变对象。

```sh
. scripts/env.sh
node --experimental-strip-types scripts/chess/prepare-assets.mjs
python3 -B scripts/chess/verify-assets.py --dist --decode
```

生成器需要工作区 `.local/chess-family/assets-tools` 中的 `@resvg/resvg-js@2.6.2`、`cwebp`、`ffmpeg` 和固定摘要的 Kenney 压缩包。当前素材已上传，CDN 回读结果在 `.local/chess/assets/cdn.json`。

## 验收

```sh
. scripts/env.sh
python3 -B scripts/ci/scope.py run --game chess --backend
WIRE_BOARD_CHESS_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 \
  python3 -B games/chess/tests/accept_ui.py
WIRE_BOARD_CHESS_CASE=ep \
  WIRE_BOARD_CHESS_FEN='k7/8/8/3pP3/8/8/8/4K3 w - d6 0 1' \
  python3 -B games/chess/tests/accept_ui.py
WIRE_BOARD_CHESS_CASE=promotion \
  WIRE_BOARD_CHESS_FEN='7k/P7/8/8/8/8/8/4K3 w - - 0 1' \
  python3 -B games/chess/tests/accept_ui.py
```

规则 HTTP 用例和动画测试先于实现。浏览器测试先于界面实现；中局 FEN 只存在于测试模块，生产创建不接受此选项。

`TestChessBotCompleteGamesHTTP` 将两档生产策略行动通过真人 HTTP 会话提交，从标准开局自然结束各 1 局。`TestChessAutoPlayCompleteHTTP` 另用真实电脑席位＋托管真人，生产调度与预算，中途重启，两档各 1 局；专项检查对原生调度另跑竞态检查。完整轨迹和结果保存在 `.local/chess/http/`，不能用人为认输或中局夹具冒充电脑完整局。

浏览器覆盖标准开局、原生鼠标／键盘／触屏、黑白朝向、桌面与 390px／320px、易位／吃过路兵／升变、动画、静音、减少动态效果、刷新、公共结算、CDN 故障后的图片／音频回退。以 `.local/chess/browser/result-*.json` 和截图为实际结果；专项范围结果在 `.local/ci/chess/result-0.json`。公共平台单独验证，不运行未修改游戏的规则或完整对局。
